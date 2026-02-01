package beacon

import (
	"context"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/sim/simconnect"
)

// MockClient implements ObjectClient for testing
type MockClient struct {
	// Telemetry to return
	Tel sim.Telemetry

	// Track calls
	Spawns  []SpawnCall
	Moves   []MoveCall
	Removes []uint32

	// ID counter
	nextID uint32
}

type SpawnCall struct {
	ReqID              uint32
	Title              string
	Lat, Lon, Alt, Hdg float64
}

type MoveCall struct {
	ID            uint32
	Lat, Lon, Alt float64
}

func (m *MockClient) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return m.Tel, nil
}

func (m *MockClient) GetState() sim.State {
	return sim.StateActive
}

func (m *MockClient) GetLastTransition(stage string) time.Time { return time.Time{} }

func (m *MockClient) SetPredictionWindow(d time.Duration) {}

func (m *MockClient) Close() error {
	return nil
}

func (m *MockClient) GetStageState() sim.StageState      { return sim.StageState{} }
func (m *MockClient) RestoreStageState(s sim.StageState) {}

func (m *MockClient) SpawnAirTraffic(reqID uint32, title, tailNum string, lat, lon, alt, hdg float64) (uint32, error) {
	m.nextID++
	id := m.nextID
	m.Spawns = append(m.Spawns, SpawnCall{reqID, title, lat, lon, alt, hdg})
	return id, nil
}

func (m *MockClient) RemoveObject(objectID, reqID uint32) error {
	m.Removes = append(m.Removes, objectID)
	return nil
}

func (m *MockClient) SetObjectPosition(objectID uint32, lat, lon, alt, pitch, bank, hdg float64) error {
	m.Moves = append(m.Moves, MoveCall{objectID, lat, lon, alt})
	return nil
}

func TestSetTarget_SpawnsBeacons(t *testing.T) {
	mock := &MockClient{
		Tel: sim.Telemetry{
			Latitude: 45.0, Longitude: -73.0, AltitudeMSL: 3000, AltitudeAGL: 3000, Heading: 90,
		},
	}
	cfg := &config.BeaconConfig{
		Enabled:           true,
		FormationEnabled:  true,
		FormationDistance: config.Distance(2000),
		FormationCount:    3,
		MinSpawnAltitude:  config.Distance(304.8),
		AltitudeFloor:     config.Distance(609.6),
	}

	svc := NewService(mock, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	// Set Target
	err := svc.SetTarget(context.Background(), 45.0, -72.0) // Target East
	if err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// Check Spawns
	// Expect 1 Target + 3 Formation = 4
	if len(mock.Spawns) != 4 {
		t.Errorf("Expected 4 spawns, got %d", len(mock.Spawns))
	}

	// Validate Target Spawn
	// reqIDSpawnTarget = 100
	var foundTarget bool
	for _, s := range mock.Spawns {
		if s.ReqID == 100 {
			foundTarget = true
			if s.Lat != 45.0 || s.Lon != -72.0 {
				t.Errorf("Target spawned at wrong loc: %v", s)
			}
		}
	}
	if !foundTarget {
		t.Error("Target beacon not spawned")
	}
}

func TestUpdateLoop_FormationLogic(t *testing.T) {
	mock := &MockClient{
		Tel: sim.Telemetry{
			Latitude: 45.0, Longitude: -73.0, AltitudeMSL: 3000, AltitudeAGL: 3000, Heading: 90,
		},
	}
	cfg := &config.BeaconConfig{
		Enabled:           true,
		FormationEnabled:  true,
		FormationDistance: config.Distance(2000),
		FormationCount:    3,
		MinSpawnAltitude:  config.Distance(304.8),
		AltitudeFloor:     config.Distance(609.6),
	}
	svc := NewService(mock, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	if err := svc.SetTarget(context.Background(), 45.0, -72.0); err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// Simulate Update
	// User moves closer
	mockTel := &simconnect.TelemetryData{
		Latitude:    45.0,
		Longitude:   -72.5, // Halfway
		AltitudeMSL: 1000,
		Heading:     90,
		GroundSpeed: 100,
		OnGround:    0,
	}

	svc.updateStep(mockTel)

	// Check Moves
	if len(mock.Moves) == 0 {
		t.Error("Expected moves after update")
	}

	// Test Despawn Trigger
	// Trigger is config.FormationDistanceKm * 1.5 = 3.0km
	// Move user very close (within 3km)
	// At lat 45, 1 deg lon is approx 78km.
	// Target at -72.0.
	// User at -72.03 -> approx 2.3km away (< 3.0km)
	mockTel.Longitude = -72.03

	svc.updateStep(mockTel)

	// Check Removes
	// Should remove 3 formation balloons
	if len(mock.Removes) != 3 {
		t.Errorf("Expected 3 removes (formation), got %d", len(mock.Removes))
	}
}

func TestSetTarget_LowAGL(t *testing.T) {
	mock := &MockClient{
		Tel: sim.Telemetry{
			Latitude: 45.0, Longitude: -73.0, AltitudeMSL: 1000, AltitudeAGL: 500, Heading: 90,
		},
	}
	cfg := &config.BeaconConfig{
		Enabled:           true,
		FormationEnabled:  true,
		FormationDistance: config.Distance(2000),
		FormationCount:    3,
		MinSpawnAltitude:  config.Distance(304.8),
		AltitudeFloor:     config.Distance(609.6),
	}
	svc := NewService(mock, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	// Set Target
	err := svc.SetTarget(context.Background(), 45.0, -72.0)
	if err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// Expect ONLY 1 Target (No formation)
	if len(mock.Spawns) != 1 {
		t.Errorf("Expected 1 spawn (target only), got %d", len(mock.Spawns))
	}

	// Validate Target Altitude
	// MSL=1000, AGL=500 (<MinSpawnAltitude=304.8m ~1000ft) -> Target should be MSL+~1000 = ~2000
	// Use tolerance for float comparison
	target := mock.Spawns[0]
	expectedAlt := 1000.0 + (304.8 * 3.28084) // MSL + minSpawnAlt in feet
	if target.Alt < expectedAlt-1.0 || target.Alt > expectedAlt+1.0 {
		t.Errorf("Expected target at ~%.1f (MSL+minSpawn), got %.1f", expectedAlt, target.Alt)
	}
}

func TestSetTarget_HighAGL(t *testing.T) {
	mock := &MockClient{
		Tel: sim.Telemetry{
			Latitude: 45.0, Longitude: -73.0, AltitudeMSL: 5000, AltitudeAGL: 3000, Heading: 90,
		},
	}
	cfg := &config.BeaconConfig{
		Enabled:           true,
		FormationEnabled:  true,
		FormationDistance: config.Distance(2000),
		FormationCount:    3,
		MinSpawnAltitude:  config.Distance(304.8),
		AltitudeFloor:     config.Distance(609.6),
	}
	svc := NewService(mock, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	// Set Target
	err := svc.SetTarget(context.Background(), 45.0, -72.0)
	if err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// Expect 1 Target + 3 Formation = 4
	if len(mock.Spawns) != 4 {
		t.Errorf("Expected 4 spawns (target+formation), got %d", len(mock.Spawns))
	}

	// Validate Target Altitude
	// MSL=5000, AGL=2000 (>MinSpawnAltitudeFt=1000) -> Target should be MSL = 5000
	for _, s := range mock.Spawns {
		if s.ReqID == 100 { // Target
			if s.Alt != 5000.0 {
				t.Errorf("Expected target at 5000.0 (MSL), got %.1f", s.Alt)
			}
		}
	}
}

func TestUpdateLoop_AltitudeLock(t *testing.T) {
	mock := &MockClient{
		Tel: sim.Telemetry{
			Latitude: 45.0, Longitude: -73.0, AltitudeMSL: 3000, AltitudeAGL: 3000, Heading: 90,
		},
	}
	cfg := &config.BeaconConfig{
		Enabled:           true,
		FormationEnabled:  true,
		FormationDistance: config.Distance(2000),
		FormationCount:    3,
		MinSpawnAltitude:  config.Distance(304.8),
		AltitudeFloor:     config.Distance(609.6),
	}
	svc := NewService(mock, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	// 1. Spawn High (3000ft AGL) -> Logic follows MSL
	if err := svc.SetTarget(context.Background(), 45.0, -72.0); err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// 2. Descend to 2500 MSL / 2500 AGL (Still > AltitudeFloorFt=2000)
	// Should follow
	mockTel := &simconnect.TelemetryData{
		Latitude:    45.0,
		Longitude:   -73.0,
		AltitudeMSL: 2500,
		AltitudeAGL: 2500,
		Heading:     90,
	}
	svc.updateStep(mockTel)

	// Check latest target pos (ID 1)
	found := false
	for i := len(mock.Moves) - 1; i >= 0; i-- {
		if mock.Moves[i].ID == 1 {
			found = true
			if mock.Moves[i].Alt != 2500.0 {
				t.Errorf("Expected 2500.0, got %.1f", mock.Moves[i].Alt)
			}
			break
		}
	}
	if !found {
		t.Error("No move for ID 1 found")
	}

	// 3. Descend below 2000ft AGL -> Lock
	// E.g. 1500 MSL / 1500 AGL
	mockTel.AltitudeMSL = 1500
	mockTel.AltitudeAGL = 1500
	svc.updateStep(mockTel)

	// Target should LOCK at previous (2500), NOT update to 1500
	found = false
	for i := len(mock.Moves) - 1; i >= 0; i-- {
		if mock.Moves[i].ID == 1 {
			found = true
			if mock.Moves[i].Alt != 2500.0 {
				t.Errorf("Expected holding at 2500.0, got %.1f", mock.Moves[i].Alt)
			}
			break
		}
	}
	if !found {
		t.Error("No move for ID 1 found")
	}

	// 4. Climb back above 2000ft AGL -> Follow
	mockTel.AltitudeMSL = 4000
	mockTel.AltitudeAGL = 4000
	svc.updateStep(mockTel)

	found = false
	for i := len(mock.Moves) - 1; i >= 0; i-- {
		if mock.Moves[i].ID == 1 {
			found = true
			if mock.Moves[i].Alt != 4000.0 {
				t.Errorf("Expected climb to 4000.0, got %.1f", mock.Moves[i].Alt)
			}
			break
		}
	}
	if !found {
		t.Error("No move for ID 1 found")
	}
}

func TestComputeFormationOffsets(t *testing.T) {
	tests := []struct {
		count    int
		expected []float64
	}{
		{1, []float64{0.0}},
		{3, []float64{-200.0, 0.0, 200.0}},
		{5, []float64{-400.0, -200.0, 0.0, 200.0, 400.0}},
		{0, []float64{0.0}},                      // Clamped to 1
		{10, []float64{-400, -200, 0, 200, 400}}, // Clamped to 5
	}

	for _, tt := range tests {
		got := computeFormationOffsets(tt.count)
		if len(got) != len(tt.expected) {
			t.Errorf("Count %d: expected length %d, got %d", tt.count, len(tt.expected), len(got))
		}
		for i, v := range got {
			if math.Abs(v-tt.expected[i]) > 0.1 {
				t.Errorf("Count %d: index %d, expected %.1f, got %.1f", tt.count, i, tt.expected[i], v)
			}
		}
	}
}

func TestSetTarget_OnGround(t *testing.T) {
	mock := &MockClient{
		Tel: sim.Telemetry{
			Latitude: 45.0, Longitude: -73.0, AltitudeMSL: 10, AltitudeAGL: 0, Heading: 90, IsOnGround: true,
		},
	}
	cfg := &config.BeaconConfig{
		Enabled:          true,
		FormationEnabled: true,
		MinSpawnAltitude: config.Distance(304.8),
	}
	svc := NewService(mock, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	// Set Target while on ground
	err := svc.SetTarget(context.Background(), 45.0, -72.0)
	if err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// Expect ZERO spawns
	if len(mock.Spawns) != 0 {
		t.Errorf("Expected 0 spawns while on ground, got %d", len(mock.Spawns))
	}

	// Ensure system is still marked active
	if !svc.active {
		t.Error("Service should be active even if spawns are suppressed")
	}
}
