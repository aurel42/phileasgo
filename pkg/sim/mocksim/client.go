package mocksim

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/terrain"
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
	StartHeading   *float64
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
	safeAltReached   bool
	elevation        *terrain.ElevationProvider

	// Ground Track Calculation
	trackBuf *geo.TrackBuffer
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
			Heading:            getHeading(cfg.StartHeading),
			IsOnGround:         true,
			PredictedLatitude:  cfg.StartLat, // Initialize to start position
			PredictedLongitude: cfg.StartLon, // Initialize to start position
			Squawk:             1200,
			Ident:              false,
		},
		groundAlt:    cfg.StartAlt,
		stateStart:   time.Now(),
		lastTurnTime: time.Now(),
		trackBuf:     geo.NewTrackBuffer(5),
	}

	m.wg.Add(1)
	go m.physicsLoop()
	return m
}

// SetElevationProvider injects an elevation provider for accurate AGL calculations.
func (m *MockClient) SetElevationProvider(e *terrain.ElevationProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.elevation = e

	// If provider is set, update initial ground altitude immediately
	if e != nil {
		elevM, err := e.GetElevation(m.tel.Latitude, m.tel.Longitude)
		if err == nil {
			m.groundAlt = float64(elevM) * 3.28084 // meters to feet

			// If we are on ground, snap to new ground alt
			if m.state != StageAirborne {
				m.tel.AltitudeMSL = m.groundAlt
			}

			// Re-init scenario with new ground alt
			m.initScenario()
		}
	}
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

// SetTransponder updates the transponder state in the mock.
func (m *MockClient) SetTransponder(squawk int, ident bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tel.Squawk = squawk
	m.tel.Ident = ident
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
		m.updateAirborne(dt, now)
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

	// Update Ground Altitude from ETOPO1 if available
	if m.elevation != nil {
		elev, err := m.elevation.GetElevation(m.tel.Latitude, m.tel.Longitude)
		if err == nil {
			m.groundAlt = float64(elev) * 3.28084 // feet
		}
	}

	// Always update IsOnGround based on state and altitude
	isOnGround := true
	if m.state == StageAirborne {
		isOnGround = m.tel.AltitudeMSL-m.groundAlt < 50
		m.tel.AltitudeAGL = math.Max(0, m.tel.AltitudeMSL-m.groundAlt)
	} else {
		m.tel.AltitudeAGL = 0
	}

	// Calculate TrackTrue (Ground Track)
	currentPos := geo.Point{Lat: m.tel.Latitude, Lon: m.tel.Longitude}
	trackTrue := m.tel.Heading // Default

	if isOnGround {
		m.trackBuf.Reset()
	} else {
		trackTrue = m.trackBuf.Push(currentPos, m.tel.Heading)
	}

	m.tel.IsOnGround = isOnGround
	m.tel.Heading = trackTrue
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

func getHeading(h *float64) float64 {
	if h == nil {
		return rand.Float64() * 360.0
	}
	return *h
}
