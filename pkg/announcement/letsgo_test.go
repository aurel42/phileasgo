package announcement

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
)

func TestLetsgo_Triggers(t *testing.T) {
	dp := &mockDP{}
	a := NewLetsgo(config.DefaultConfig(), dp, dp)

	tests := []struct {
		name         string
		stage        string
		speed        float64
		alt          float64
		lastTakeoff  time.Time
		expectedGen  bool
		expectedPlay bool
	}{
		{"Ground Slow", sim.StageTaxi, 10, 0, time.Time{}, false, false},
		{"Takeoff Speed", sim.StageTakeOff, 45, 10, time.Now(), true, false},
		{"Climb Low", sim.StageClimb, 100, 100, time.Now(), true, false},
		{"Climb High", sim.StageClimb, 120, 600, time.Now(), true, true},
		{"Cruise", sim.StageCruise, 200, 5000, time.Now().Add(-1 * time.Hour), false, false},
		// New Case: Mid-Air restart (Simulates flight stage 'Climb' but long duration)
		{"Climb Long Duration", sim.StageClimb, 120, 5000, time.Now().Add(-10 * time.Minute), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock provider response
			dp.GetLastTransitionFunc = func(s string) time.Time {
				if s == sim.StageTakeOff {
					return tt.lastTakeoff
				}
				return time.Time{}
			}

			tel := &sim.Telemetry{
				FlightStage: tt.stage,
				GroundSpeed: tt.speed,
				AltitudeAGL: tt.alt,
			}
			// Note: expectedPlay depends on expectedGen failing if logic was purely sequential in loop,
			// but here we test methods independently.
			// ShouldGenerate checks duration.
			if got := a.ShouldGenerate(tel); got != tt.expectedGen {
				t.Errorf("ShouldGenerate for %s: expected %v, got %v", tt.name, tt.expectedGen, got)
			}
			// ShouldPlay checks altitude > 500
			if got := a.ShouldPlay(tel); got != tt.expectedPlay {
				t.Errorf("ShouldPlay for %s: expected %v, got %v", tt.name, tt.expectedPlay, got)
			}
		})
	}
}

func TestLetsgo_GetPromptData(t *testing.T) {
	dp := &mockDP{
		AssembleGenericFunc: func(ctx context.Context, tel *sim.Telemetry) prompt.Data {
			return prompt.Data{"Language": "en"}
		},
	}
	a := NewLetsgo(config.DefaultConfig(), dp, dp)
	data, err := a.GetPromptData(&sim.Telemetry{})
	if err != nil {
		t.Fatal(err)
	}
	d := data.(prompt.Data)
	if d["Language"] != "en" {
		t.Error("Expected Language en")
	}
}
