package announcement

import (
	"phileasgo/pkg/sim"
	"testing"
)

func TestLetsgo_Triggers(t *testing.T) {
	dp := &mockDP{}
	a := NewLetsgo(dp)

	tests := []struct {
		name         string
		stage        string
		speed        float64
		alt          float64
		expectedGen  bool
		expectedPlay bool
	}{
		{"Ground Slow", sim.StageTaxi, 10, 0, false, false},
		{"Takeoff Speed", sim.StageTakeOff, 45, 10, true, false},
		{"Climb Low", sim.StageClimb, 100, 100, true, false},
		{"Climb High", sim.StageClimb, 120, 600, true, true},
		{"Cruise", sim.StageCruise, 200, 5000, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tel := &sim.Telemetry{
				FlightStage: tt.stage,
				GroundSpeed: tt.speed,
				AltitudeAGL: tt.alt,
			}
			if a.ShouldGenerate(tel) != tt.expectedGen {
				t.Errorf("ShouldGenerate for %s: expected %v, got %v", tt.name, tt.expectedGen, !tt.expectedGen)
			}
			if a.ShouldPlay(tel) != tt.expectedPlay {
				t.Errorf("ShouldPlay for %s: expected %v, got %v", tt.name, tt.expectedPlay, !tt.expectedPlay)
			}
		})
	}
}
