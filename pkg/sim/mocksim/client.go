package mocksim

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
)

const (
	// FlightStages
	StageParked   = "PARKED"
	StageTaxiing  = "TAXIING"
	StageHolding  = "HOLDING"
	StageAirborne = "AIRBORNE"

	// Physics constants
	tickRateMs = 100
)

// Config holds timing configuration for the mock simulation.
type Config struct {
	DurationParked time.Duration
	DurationTaxi   time.Duration
	DurationHold   time.Duration
	StartLat       float64
	StartLon       float64
	StartAlt       float64
	StartHeading   float64
}

type scenarioStep struct {
	Type     string
	Target   float64 // for CLIMB
	Rate     float64 // rate in units/min (fpm)
	Duration float64 // seconds for WAIT
}

// MockClient implements sim.Client.
type MockClient struct {
	mu               sync.Mutex
	tel              sim.Telemetry
	state            string
	stateStart       time.Time
	config           Config
	stopCh           chan struct{}
	predictionWindow time.Duration
	wg               sync.WaitGroup
	scenario         []scenarioStep
	scenarioIdx      int
	stepStart        time.Time
	lastTurnTime     time.Time
	groundAlt        float64
}

// NewClient creates a new mock simulator client.
func NewClient(cfg Config) *MockClient {
	m := &MockClient{
		config:           cfg,
		stopCh:           make(chan struct{}),
		predictionWindow: 60 * time.Second,
		state:            StageParked,
		tel: sim.Telemetry{
			Latitude:           cfg.StartLat,
			Longitude:          cfg.StartLon,
			AltitudeMSL:        cfg.StartAlt,
			AltitudeAGL:        0,
			Heading:            cfg.StartHeading,
			IsOnGround:         true,
			PredictedLatitude:  cfg.StartLat, // Initialize to start position
			PredictedLongitude: cfg.StartLon, // Initialize to start position
		},
		groundAlt:    cfg.StartAlt,
		stateStart:   time.Now(),
		lastTurnTime: time.Now(),
	}

	m.wg.Add(1)
	go m.physicsLoop()
	return m
}

// GetTelemetry returns the current state of the simulated aircraft.
func (m *MockClient) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Update timestamp to now on read
	return m.tel, nil
}

// GetState returns the current simulator connection/activity state.
// Mock is always active.
func (m *MockClient) GetState() sim.State {
	return sim.StateActive
}

// SetPredictionWindow sets the time duration for future position prediction.
func (m *MockClient) SetPredictionWindow(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.predictionWindow = d
}

// Close stops the physics loop and releases resources.
func (m *MockClient) Close() error {
	close(m.stopCh)
	m.wg.Wait()
	return nil
}

func (m *MockClient) physicsLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Duration(tickRateMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.update()
		}
	}
}

func (m *MockClient) update() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	dt := float64(tickRateMs) / 1000.0 // seconds
	stateDuration := now.Sub(m.stateStart)

	switch m.state {
	case StageParked:
		m.tel.GroundSpeed = 0
		m.tel.IsOnGround = true
		if stateDuration >= m.config.DurationParked {
			m.state = StageTaxiing
			m.stateStart = now
		}

	case StageTaxiing:
		m.tel.IsOnGround = true
		m.tel.GroundSpeed = 15.0
		// Move straight
		distNm := m.tel.GroundSpeed * (dt / 3600.0)
		distDeg := distNm / 60.0
		radHeading := m.tel.Heading * (math.Pi / 180.0)
		m.tel.Latitude += distDeg * math.Cos(radHeading)
		m.tel.Longitude += distDeg * math.Sin(radHeading)

		if stateDuration >= m.config.DurationTaxi {
			m.state = StageHolding
			m.stateStart = now
		}

	case StageHolding:
		m.tel.IsOnGround = true
		m.tel.GroundSpeed = 0
		if stateDuration >= m.config.DurationHold {
			m.state = StageAirborne
			m.stateStart = now
			m.initScenario()
		}

	case StageAirborne:
		m.tel.IsOnGround = false
		m.tel.GroundSpeed = 120.0
		// Wander logic
		if now.Sub(m.lastTurnTime) > 60*time.Second {
			change := (rand.Float64() * 20) - 10 // -10 to +10 degrees
			m.tel.Heading = math.Mod(m.tel.Heading+change, 360.0)
			if m.tel.Heading < 0 {
				m.tel.Heading += 360.0
			}
			m.lastTurnTime = now
		}

		m.updateScenario(dt, now)

		// Move
		distNm := m.tel.GroundSpeed * (dt / 3600.0)
		distDeg := distNm / 60.0
		radHeading := m.tel.Heading * (math.Pi / 180.0)
		m.tel.Latitude += distDeg * math.Cos(radHeading)
		m.tel.Longitude += distDeg * math.Sin(radHeading)
	}

	// Update Prediction for ALL stages
	distMetersPred := m.tel.GroundSpeed * 0.514444 * m.predictionWindow.Seconds()
	if distMetersPred > 0 {
		pred := geo.DestinationPoint(
			geo.Point{Lat: m.tel.Latitude, Lon: m.tel.Longitude},
			distMetersPred,
			m.tel.Heading,
		)
		m.tel.PredictedLatitude = pred.Lat
		m.tel.PredictedLongitude = pred.Lon
	} else {
		// Stationary: predicted position = current position
		m.tel.PredictedLatitude = m.tel.Latitude
		m.tel.PredictedLongitude = m.tel.Longitude
	}

	// Always update IsOnGround based on state and altitude
	if m.state == StageAirborne {
		m.tel.IsOnGround = m.tel.AltitudeMSL-m.groundAlt < 50
		m.tel.AltitudeAGL = math.Max(0, m.tel.AltitudeMSL-m.groundAlt)
	} else {
		m.tel.IsOnGround = true
		m.tel.AltitudeAGL = 0
	}
	m.tel.FlightStage = sim.DetermineFlightStage(&m.tel)
}

func (m *MockClient) initScenario() {
	// Calculate bottom based on airfield elevation (round down to nearest 1000)
	bottom := math.Floor(m.groundAlt/1000.0) * 1000.0

	m.scenario = []scenarioStep{
		{Type: "CLIMB", Target: 1500.0 + bottom, Rate: 500.0},
		{Type: "WAIT", Duration: 120.0},
	}
	// Step climbs to 5500 + bottom
	alt := 1500.0
	for alt < 5500.0 {
		alt += 1000.0
		m.scenario = append(m.scenario,
			scenarioStep{Type: "CLIMB", Target: alt + bottom, Rate: 500.0},
			scenarioStep{Type: "WAIT", Duration: 120.0},
		)
	}
	m.scenario = append(m.scenario,
		scenarioStep{Type: "CLIMB", Target: 12000.0 + bottom, Rate: 2000.0},
		scenarioStep{Type: "WAIT", Duration: 120.0},
		scenarioStep{Type: "CLIMB", Target: 8000.0 + bottom, Rate: -1000.0}, // Descent
		scenarioStep{Type: "CLIMB", Target: 1500.0 + bottom, Rate: -500.0},  // Descent
	)
	m.scenarioIdx = 0
	m.stepStart = time.Time{}
}

func (m *MockClient) updateScenario(dt float64, now time.Time) {
	if len(m.scenario) == 0 {
		return
	}
	if m.scenarioIdx >= len(m.scenario) {
		m.scenarioIdx = 0
		m.stepStart = time.Time{}
	}

	step := m.scenario[m.scenarioIdx]

	switch step.Type {
	case "WAIT":
		m.tel.VerticalSpeed = 0
		if m.stepStart.IsZero() {
			m.stepStart = now
		}
		if now.Sub(m.stepStart).Seconds() >= step.Duration {
			m.scenarioIdx++
			m.stepStart = time.Time{}
		}
	case "CLIMB":
		delta := (step.Rate / 60.0) * dt
		m.tel.VerticalSpeed = step.Rate

		// If we are close enough or passed target, snap and move next
		// Note logic handled for both climb (+) and descent (-)
		reached := false
		if step.Rate > 0 {
			if m.tel.AltitudeMSL+delta >= step.Target {
				reached = true
			}
		} else {
			if m.tel.AltitudeMSL+delta <= step.Target {
				reached = true
			}
		}

		if reached {
			m.tel.AltitudeMSL = step.Target
			m.tel.VerticalSpeed = 0
			m.scenarioIdx++
			m.stepStart = time.Time{}
		} else {
			m.tel.AltitudeMSL += delta
		}
	}
}
