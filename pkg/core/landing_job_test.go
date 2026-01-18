package core

import (
	"context"
	"testing"

	"phileasgo/pkg/sim"
)

type mockDebriefer struct {
	playDebriefCalled bool
	lastTel           *sim.Telemetry
}

func (m *mockDebriefer) PlayDebrief(ctx context.Context, tel *sim.Telemetry) bool {
	m.playDebriefCalled = true
	m.lastTel = tel
	return true
}

func TestLandingJob(t *testing.T) {
	tests := []struct {
		name           string
		initialGround  bool
		updates        []*sim.Telemetry
		expectTrigger  bool
		expectTriggerN int // How many times it should trigger
	}{
		{
			name:          "Stay Airborne - No Trigger",
			initialGround: false,
			updates: []*sim.Telemetry{
				{IsOnGround: false, GroundSpeed: 100},
				{IsOnGround: false, GroundSpeed: 100},
			},
			expectTrigger: false,
		},
		{
			name:          "Stay On Ground - No Trigger (Already landed)",
			initialGround: true,
			updates: []*sim.Telemetry{
				{IsOnGround: true, GroundSpeed: 0},
				{IsOnGround: true, GroundSpeed: 0},
			},
			expectTrigger: false,
		},
		{
			name:          "Land Fast - No Trigger yet",
			initialGround: false,
			updates: []*sim.Telemetry{
				{IsOnGround: false, GroundSpeed: 100},
				{IsOnGround: true, GroundSpeed: 50}, // > 15 kts
			},
			expectTrigger: false,
		},
		{
			name:          "Land Slow - Trigger",
			initialGround: false,
			updates: []*sim.Telemetry{
				{IsOnGround: false, GroundSpeed: 100},
				{IsOnGround: true, GroundSpeed: 10}, // < 15 kts
			},
			expectTrigger:  true,
			expectTriggerN: 1,
		},
		{
			name:          "Land, Speed up, Slow down - One Trigger (Cooldown)",
			initialGround: false,
			updates: []*sim.Telemetry{
				{IsOnGround: false, GroundSpeed: 100},
				{IsOnGround: true, GroundSpeed: 10}, // Trigger 1
				{IsOnGround: true, GroundSpeed: 20},
				{IsOnGround: true, GroundSpeed: 10}, // Should be blocked by cooldown
			},
			expectTrigger:  true,
			expectTriggerN: 1,
		},
		{
			name:          "Touch and Go - No Trigger if not slow enough",
			initialGround: false,
			updates: []*sim.Telemetry{
				{IsOnGround: false, GroundSpeed: 100},
				{IsOnGround: true, GroundSpeed: 80},   // Touch
				{IsOnGround: false, GroundSpeed: 120}, // Go
			},
			expectTrigger: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			debriefer := &mockDebriefer{}
			job := NewLandingJob(debriefer)

			// Initial state setup
			// We need to simulate the job running for a bit to establish state
			// For LandingJob, "wasAirborne" is the key.
			// If initialGround is true, we simulate a cycle on ground.
			// If initialGround is false, we simulate a cycle airborne.

			ctx := context.Background()
			initTel := &sim.Telemetry{IsOnGround: tt.initialGround, GroundSpeed: 100}
			if tt.initialGround {
				initTel.GroundSpeed = 0
			}
			job.ShouldFire(initTel) // Prime state only, do not run!

			triggerCount := 0

			// Run Updates
			for _, tel := range tt.updates {
				// We wrap the debrief check by inspecting the mock after Run
				debriefer.playDebriefCalled = false

				if job.ShouldFire(tel) {
					job.Run(ctx, tel)
				}

				if debriefer.playDebriefCalled {
					triggerCount++
				}
			}

			if tt.expectTrigger && triggerCount == 0 {
				t.Error("Expected Debrief to be triggered, but it wasn't")
			}
			if !tt.expectTrigger && triggerCount > 0 {
				t.Errorf("Expected NO trigger, but got %d", triggerCount)
			}
			if tt.expectTrigger && tt.expectTriggerN > 0 && triggerCount != tt.expectTriggerN {
				t.Errorf("Expected %d triggers, got %d", tt.expectTriggerN, triggerCount)
			}
		})
	}
}
