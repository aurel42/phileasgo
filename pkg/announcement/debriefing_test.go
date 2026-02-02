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
		{"Ground Idle", sim.StageParked, time.Time{}, false, false},
		{"Airborne", sim.StageCruise, now.Add(-10 * time.Minute), false, false},
		{"Landed Early", sim.StageLanded, now.Add(-1 * time.Minute), false, false},
		{"Landed Enough", sim.StageLanded, now.Add(-10 * time.Minute), true, false},
		{"Taxi Enough", sim.StageTaxi, now.Add(-10 * time.Minute), true, true},
		{"Hold Enough", sim.StageHold, now.Add(-10 * time.Minute), true, true},
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
}
