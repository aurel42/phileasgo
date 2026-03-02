package beacon

import (
	"log/slog"
	"math"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/sim/simconnect"
)

func TestService_InternalLogic(t *testing.T) {
	mockClient := &MockClient{}
	logger := slog.Default()
	prov := config.NewProvider(&config.Config{}, nil)
	s := NewService(mockClient, logger, prov)

	// Test SetElevationProvider
	s.SetElevationProvider(nil)

	// Test Clear (when not active)
	s.Clear()
	if s.active {
		t.Error("Service should not be active after Clear")
	}

	// Test isIdle (baseline)
	if !s.isIdle() {
		t.Error("Service should be idle when not active")
	}

	// Activate and test active/idle state
	s.active = true
	s.spawnedBeacons = append(s.spawnedBeacons, SpawnedBeacon{ID: 1})
	if s.isIdle() {
		t.Error("Service should not be idle when active with beacons")
	}

	// Test Clear (when active)
	s.Clear()
	if s.active {
		t.Error("Service should not be active after Clear")
	}
	if len(s.spawnedBeacons) != 0 {
		t.Error("Beacons should be cleared")
	}

	// Test retryInterval
	if s.retryInterval() != 60*time.Second {
		t.Errorf("Expected 60s retry interval, got %v", s.retryInterval())
	}

	// Test isBeaconStale
	tel := &simconnect.TelemetryData{
		Latitude:  10.0,
		Longitude: 10.0,
		Heading:   0.0,
	}
	// 1. Not stale: Distance 0, even if behind (diff doesn't matter much)
	if s.isBeaconStale(tel, 0, 0) {
		t.Error("Expected beacon at distance 0 NOT to be stale")
	}

	// 2. Not stale: Distance 10km (< 50km)
	if s.isBeaconStale(tel, math.Pi, 10.0) {
		t.Error("Expected beacon at distance 10km NOT to be stale")
	}

	// 3. Stale: Distance 60km and behind (180 deg)
	if !s.isBeaconStale(tel, math.Pi, 60.0) {
		t.Error("Expected beacon at distance 60km and behind to be stale")
	}

	// Test SetDLLPath
	s.SetDLLPath("test.dll")
	if s.dllPath != "test.dll" {
		t.Errorf("Expected dllPath test.dll, got %s", s.dllPath)
	}
}
