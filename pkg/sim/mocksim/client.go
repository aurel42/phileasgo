package mocksim

import (
	"context"
	"fmt"
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
	StageTakeoff  = "TAKEOFF"
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

type ScenarioStep struct {
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
	scenario         []ScenarioStep
	scenarioIdx      int
	stepStart        time.Time
	lastTurnTime     time.Time
	groundAlt        float64
	safeAltReached   bool
	elevation        *terrain.ElevationProvider
	lastUpdate       time.Time // Wall-clock time of the last physics update
	isLanding        bool
	landingStartTime time.Time
	turnCount        int

	// Ground Track Calculation
	trackBuf *geo.TrackBuffer
	vsBuf    *sim.VerticalSpeedBuffer

	// State Machine
	stageMachine *sim.StageMachine

	useCustomScenario bool
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
			Provider:           "mock",
		},
		groundAlt:    cfg.StartAlt,
		stateStart:   time.Now(),
		lastTurnTime: time.Now(),
		trackBuf:     geo.NewTrackBuffer(5),
		vsBuf:        sim.NewVerticalSpeedBuffer(5 * time.Second),
		stageMachine: sim.NewStageMachine(),
		lastUpdate:   time.Now(),
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

// GetLastTransition returns the timestamp of the last transition to the given stage.
func (m *MockClient) GetLastTransition(stage string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stageMachine == nil {
		return time.Time{}
	}
	return m.stageMachine.GetLastTransition(stage)
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

// GetStageState returns the current stage machine state (stub).
func (c *MockClient) GetStageState() sim.StageState {
	return sim.StageState{}
}

// RestoreStageState restores the stage machine state (stub).
func (c *MockClient) RestoreStageState(s sim.StageState) {
	// No-op for mock
}

// ExecuteCommand handles external commands for the mock simulator.
func (m *MockClient) ExecuteCommand(ctx context.Context, cmd string, args map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch cmd {
	case "land":
		if m.state == StageAirborne {
			m.isLanding = true
			m.landingStartTime = time.Now()
			return nil
		}
		return fmt.Errorf("cannot land: not airborne (state: %s)", m.state)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// SetEventRecorder delegates to the stage machine.
func (c *MockClient) SetEventRecorder(r sim.EventRecorder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stageMachine != nil {
		c.stageMachine.SetRecorder(r)
	}
}

// ObjectClient Implementation

func (m *MockClient) SpawnAirTraffic(reqID uint32, title, livery, tailNum string, lat, lon, alt, hdg float64) (uint32, error) {
	return 0, nil // No-op for mock
}

func (m *MockClient) RemoveObject(objectID, reqID uint32) error {
	return nil // No-op for mock
}

func (m *MockClient) SetObjectPosition(objectID uint32, lat, lon, alt, pitch, bank, hdg float64) error {
	return nil // No-op for mock
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
	dt := now.Sub(m.lastUpdate).Seconds()
	m.lastUpdate = now

	stateDuration := now.Sub(m.stateStart)
	m.tel.EngineOn = true

	switch m.state {
	case StageParked:
		m.updateParked(stateDuration)
	case StageTaxiing:
		m.updateTaxiing(dt, stateDuration)
	case StageHolding:
		m.updateHolding(stateDuration)
	case StageTakeoff:
		m.updateTakeoff(dt)
	case StageAirborne:
		m.updateAirborne(dt, now)
	}

	m.updateDerivedState(now)
}

func (m *MockClient) updateParked(stateDuration time.Duration) {
	m.tel.EngineOn = false
	m.tel.GroundSpeed = 0
	m.tel.IsOnGround = true

	// If we just landed, wait 5 minutes before restarting takeoff sequence
	if !m.landingStartTime.IsZero() {
		if stateDuration < 5*time.Minute {
			return
		}
		// Reset landing state to allow normal scenario to resume
		m.landingStartTime = time.Time{}
		m.safeAltReached = false
	}

	if stateDuration >= m.config.DurationParked {
		m.state = StageTaxiing
		m.stateStart = time.Now()
	}
}

func (m *MockClient) updateTaxiing(dt float64, stateDuration time.Duration) {
	m.tel.IsOnGround = true
	m.tel.GroundSpeed = 15.0
	m.moveGeodesic(dt)

	if stateDuration >= m.config.DurationTaxi {
		m.state = StageHolding
		m.stateStart = time.Now()
	}
}

func (m *MockClient) updateHolding(stateDuration time.Duration) {
	m.tel.IsOnGround = true
	m.tel.GroundSpeed = 0
	if stateDuration >= m.config.DurationHold {
		m.state = StageTakeoff
		m.stateStart = time.Now()
	}
}

func (m *MockClient) updateTakeoff(dt float64) {
	m.tel.IsOnGround = true
	m.tel.GroundSpeed += 5.0 * dt // Accelerate ~5 kts/s
	m.moveGeodesic(dt)

	if m.tel.GroundSpeed >= 80.0 {
		m.state = StageAirborne
		m.stateStart = time.Now()
		m.initScenario()
	}
}

func (m *MockClient) moveGeodesic(dt float64) {
	distMeters := m.tel.GroundSpeed * 0.514444 * dt
	if distMeters <= 0 {
		return
	}
	nextPos := geo.DestinationPoint(
		geo.Point{Lat: m.tel.Latitude, Lon: m.tel.Longitude},
		distMeters,
		m.tel.Heading,
	)
	m.tel.Latitude = nextPos.Lat
	m.tel.Longitude = nextPos.Lon
}

func (m *MockClient) updateDerivedState(now time.Time) {
	// Update Prediction
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
		m.tel.PredictedLatitude = m.tel.Latitude
		m.tel.PredictedLongitude = m.tel.Longitude
	}

	// Terrain Following
	if m.elevation != nil {
		elev, err := m.elevation.GetElevation(m.tel.Latitude, m.tel.Longitude)
		if err == nil {
			m.groundAlt = float64(elev) * 3.28084
		}
	}

	isOnGround := true
	if m.state == StageAirborne {
		isOnGround = m.tel.AltitudeMSL-m.groundAlt < 50
		m.tel.AltitudeAGL = math.Max(0, m.tel.AltitudeMSL-m.groundAlt)
	} else {
		m.tel.AltitudeAGL = 0
	}

	m.tel.IsOnGround = isOnGround
	if isOnGround {
		m.trackBuf.Reset()
	} else {
		m.trackBuf.Push(geo.Point{Lat: m.tel.Latitude, Lon: m.tel.Longitude}, m.tel.Heading)
	}

	m.tel.VerticalSpeed = m.vsBuf.Update(now, m.tel.AltitudeMSL)
	m.tel.FlightStage = m.stageMachine.Update(&m.tel)
}

func (m *MockClient) initScenario() {
	if m.useCustomScenario {
		return
	}
	// Calculate bottom based on airfield elevation (round down to nearest 1000)
	bottom := math.Floor(m.groundAlt/1000.0) * 1000.0

	m.scenario = []ScenarioStep{
		{Type: "CLIMB", Target: 1500.0 + bottom, Rate: 500.0},
		{Type: "WAIT", Duration: 120.0},
	}
	// Step climbs to 5500 + bottom
	alt := 1500.0
	for alt < 5500.0 {
		alt += 1000.0
		m.scenario = append(m.scenario,
			ScenarioStep{Type: "CLIMB", Target: alt + bottom, Rate: 500.0},
			ScenarioStep{Type: "WAIT", Duration: 120.0},
		)
	}
	m.scenario = append(m.scenario,
		ScenarioStep{Type: "CLIMB", Target: 12000.0 + bottom, Rate: 2000.0},
		ScenarioStep{Type: "WAIT", Duration: 120.0},
		ScenarioStep{Type: "CLIMB", Target: 8000.0 + bottom, Rate: -1000.0}, // Descent
		ScenarioStep{Type: "CLIMB", Target: 1500.0 + bottom, Rate: -500.0},  // Descent
	)
	m.scenarioIdx = 0
	m.stepStart = time.Time{}
}

// SetScenario allows replacing the default flight scenario.
// Useful for testing specific flight tracking or speeding up tests.
func (m *MockClient) SetScenario(steps []ScenarioStep) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenario = steps
	m.scenarioIdx = 0
	m.stepStart = time.Time{}
	m.useCustomScenario = true
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
