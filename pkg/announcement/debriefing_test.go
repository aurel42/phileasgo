package announcement

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

func TestDebriefing_Triggers(t *testing.T) {
	dp := &mockDP{}
	cfg := config.DefaultConfig()
	a := NewDebriefing(cfg, dp, dp)

	now := time.Now()

	tests := []struct {
		name         string
		stage        string
		takeoffTime  time.Time
		expectedGen  bool
		expectedPlay bool
	}{
		{"Ground Idle", sim.StageParked, time.Time{}, false, true},
		{"Airborne", sim.StageCruise, now.Add(-10 * time.Minute), false, false},
		{"Landed Early", sim.StageLanded, now.Add(-1 * time.Minute), false, false},
		{"Landed Enough", sim.StageLanded, now.Add(-10 * time.Minute), true, false},
		{"Taxi Enough", sim.StageTaxi, now.Add(-10 * time.Minute), true, true},
		{"Hold Enough", sim.StageHold, now.Add(-10 * time.Minute), true, true},
		{"Parked Enough", sim.StageParked, now.Add(-10 * time.Minute), true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp.GetLastTransitionFunc = func(s string) time.Time {
				if s == sim.StageTakeOff {
					return tt.takeoffTime
				}
				return time.Time{}
			}

			tel := &sim.Telemetry{FlightStage: tt.stage}
			if got := a.ShouldGenerate(tel); got != tt.expectedGen {
				t.Errorf("ShouldGenerate for %s: expected %v, got %v", tt.name, tt.expectedGen, got)
			}
			if got := a.ShouldPlay(tel); got != tt.expectedPlay {
				t.Errorf("ShouldPlay for %s: expected %v, got %v", tt.name, tt.expectedPlay, got)
			}
		})
	}

	t.Run("Loop Prevention", func(t *testing.T) {
		tel := &sim.Telemetry{FlightStage: sim.StageTaxi}

		// 1. Initial takeoff 30 mins ago
		takeoff1 := time.Now().Add(-30 * time.Minute)
		dp.GetLastTransitionFunc = func(s string) time.Time {
			if s == sim.StageTakeOff {
				return takeoff1
			}
			return time.Time{}
		}

		// First time: should generate
		if !a.ShouldGenerate(tel) {
			t.Fatal("First ShouldGenerate expected true")
		}

		// Simulate successful generation start
		a.GetPromptData(tel) // sets lastGeneratedAt to Now

		// Second time: should NOT generate (leg has not changed)
		if a.ShouldGenerate(tel) {
			t.Error("Second ShouldGenerate expected false (loop prevention)")
		}

		// 2. New takeoff:
		// We mock lastGeneratedAt to the past to simulate time passing
		a.mu.Lock()
		a.lastGeneratedAt = time.Now().Add(-1 * time.Hour)
		a.mu.Unlock()

		takeoff2 := time.Now().Add(-10 * time.Minute) // Newer than lastGeneratedAt AND > 5 mins old
		dp.GetLastTransitionFunc = func(s string) time.Time {
			if s == sim.StageTakeOff {
				return takeoff2
			}
			return time.Time{}
		}
		if !a.ShouldGenerate(tel) {
			t.Error("ShouldGenerate expected true after new takeoff")
		}
	})

	t.Run("Reset Session", func(t *testing.T) {
		a.Reset()
		// lastGeneratedAt is NOT reset by Reset (Base),
		// but if we were looking for Airborne state, Reset usually happens on Teleport.
		// For now, only new takeoff or deep reset should clear it.
	})
}
