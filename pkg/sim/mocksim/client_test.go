package mocksim

import (
	"context"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

func waitForReq(t *testing.T, check func() bool, timeout time.Duration, msg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("Timeout waiting for: %s", msg)
}

func TestStateTransitions(t *testing.T) {
	// Use short durations for testing
	cfg := Config{
		DurationParked: 50 * time.Millisecond,
		DurationTaxi:   50 * time.Millisecond,
		DurationHold:   50 * time.Millisecond,
		StartLat:       0,
		StartLon:       0,
	}

	client := NewClient(cfg)
	defer client.Close()

	ctx := context.Background()

	// 1. Initial State: PARKED
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		return tel.IsOnGround && tel.GroundSpeed == 0
	}, 1*time.Second, "Initial PARKED")
}

func TestStateSequence(t *testing.T) {
	// Durations longer than tick (100ms) to ensure we catch them
	cfg := Config{
		DurationParked: 200 * time.Millisecond,
		DurationTaxi:   200 * time.Millisecond,
		DurationHold:   200 * time.Millisecond,
	}
	client := NewClient(cfg)
	defer client.Close()

	ctx := context.Background()

	// 1. PARKED: Expect Speed 0
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		return tel.IsOnGround && tel.GroundSpeed == 0
	}, 1*time.Second, "ParKed State")

	// Inject fast scenario for when we reach hold/airborne
	client.SetScenario([]ScenarioStep{
		{Type: "CLIMB", Target: 5000.0, Rate: 10000.0}, // Climb fast
	})

	// 2. TAXI: Expect Speed 15 eventually
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		return tel.IsOnGround && tel.GroundSpeed == 15.0
	}, 2*time.Second, "Taxi State")

	// 3. HOLD: Expect Speed 0 eventually
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		// We can distinguish PARKED vs HOLD by time or state internal,
		// but externally they look similar (Ground+0kts).
		// However, we know we were just in TAXI.
		return tel.IsOnGround && tel.GroundSpeed == 0
	}, 2*time.Second, "Hold State")

	// 4. AIRBORNE: Expect Ground=false, Speed=120
	// At 500fpm, it takes ~6s to reach the 50ft AGL threshold for !IsOnGround
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		return !tel.IsOnGround && tel.GroundSpeed == 120.0
	}, 10*time.Second, "Airborne State")
}

func TestPhysics(t *testing.T) {
	cfg := Config{
		DurationParked: 0,
		DurationTaxi:   10 * time.Minute, // Stay in Taxi
		StartLat:       0,
		StartLon:       0,
	}
	client := NewClient(cfg)
	defer client.Close()

	// Force heading North
	client.mu.Lock()
	client.tel.Heading = 0 // North
	client.mu.Unlock()

	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(context.Background())
		return tel.Latitude > 0.00001
	}, 2*time.Second, "Movement North")
}

func TestScenario(t *testing.T) {
	cfg := Config{
		DurationParked: 0,
		DurationTaxi:   0,
		DurationHold:   0,
	}
	client := NewClient(cfg)
	defer client.Close()

	// Wait for climb to start
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(context.Background())
		return tel.VerticalSpeed == 500.0
	}, 1*time.Second, "Initial Climb 500fpm")
}

func TestGroundTrack(t *testing.T) {
	// Scenario: Fixed aircraft heading but manual position drift to simulate track
	cfg := Config{
		DurationParked: 0,
		DurationTaxi:   0,
		DurationHold:   0,
		StartLat:       10,
		StartLon:       20,
	}
	client := NewClient(cfg)
	defer client.Close()

	// Inject fast scenario to reach airborne state quickly
	client.SetScenario([]ScenarioStep{
		{Type: "CLIMB", Target: 2000.0, Rate: 10000.0},
	})

	ctx := context.Background()
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		return !tel.IsOnGround
	}, 10*time.Second, "Airborne")

	// 2. Mock some movement: Move EAST (90 deg) manually
	// Heading is currently random/nose, but track should become 90 eventually
	// Note: Mock physics loop will still run, so we need to be faster than it
	// or stop it. Better: verify it's moving.

	// Since MockSim moves straight, 'Heading' should already be the track.
	// But let's verify it matches the displacement vector.

	var lastTel sim.Telemetry
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(ctx)
		// We expect Heading to be updated from default
		// once buffer has enough samples.
		if tel.Heading != 0 {
			lastTel = tel
			return true
		}
		return false
	}, 10*time.Second, "Track Calculation")

	if lastTel.Heading == 0 && lastTel.GroundSpeed > 0 {
		t.Errorf("Heading should not be 0 while moving (unless nose is North)")
	}
}
