package core

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

type mockResettable struct {
	resetCalled bool
}

func (m *mockResettable) ResetSession(ctx context.Context) {
	m.resetCalled = true
}

func TestScheduler_TeleportDetection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Ticker.TelemetryLoop = config.Duration(10 * time.Millisecond)
	cfg.Sim.TeleportThreshold = config.Distance(100.0 * 1000.0) // 100km

	// Reusing mockSimClient from scheduler_test.go (in package core)
	// It has SetTelemetry helper
	mockSim := &mockSimClient{}

	sched := NewScheduler(cfg, mockSim, nil, nil)

	mr1 := &mockResettable{}
	mr2 := &mockResettable{}
	sched.AddResettable(mr1)
	sched.AddResettable(mr2)

	// 1. Tick 1: Initial position (London)
	mockSim.SetTelemetry(&sim.Telemetry{
		Latitude:  51.5074,
		Longitude: -0.1278,
	})
	sched.tick(context.Background())

	// Verify no reset yet (first tick just sets lastPos)
	if mr1.resetCalled || mr2.resetCalled {
		t.Error("Reset called on first tick")
	}

	// 2. Tick 2: Small movement (Heathrow, ~20km) -> No Reset
	// 1 degree lat is ~111km. 0.1 degree ~11km.
	mockSim.SetTelemetry(&sim.Telemetry{
		Latitude:  51.4700, // Small change
		Longitude: -0.4543,
	})
	sched.tick(context.Background())
	if mr1.resetCalled {
		t.Error("Reset called on small movement")
	}

	// 3. Tick 3: Teleport (New York) -> Reset Triggered!
	// Distance > 100km
	mockSim.SetTelemetry(&sim.Telemetry{
		Latitude:  40.7128,
		Longitude: -74.0060,
	})
	sched.tick(context.Background())

	if !mr1.resetCalled {
		t.Error("Reset NOT called on teleport (mr1)")
	}
	if !mr2.resetCalled {
		t.Error("Reset NOT called on teleport (mr2)")
	}
}
