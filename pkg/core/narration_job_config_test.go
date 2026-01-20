package core

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

func TestNarrationJob_EssayConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 10.0
	cfg.Narrator.Essay.Enabled = true // Default ON

	// Mock dependencies
	mockN := &mockNarratorService{}
	pm := &mockPOIManager{best: nil, lat: 48.0, lon: -123.0} // Valid consistency

	tests := []struct {
		name         string
		essayEnabled bool
		altitudeAGL  float64
		expectFire   bool
	}{
		{
			name:         "Enabled + High Altitude -> Fire",
			essayEnabled: true,
			altitudeAGL:  3000,
			expectFire:   true,
		},
		{
			name:         "Disabled + High Altitude -> No Fire",
			essayEnabled: false,
			altitudeAGL:  3000,
			expectFire:   false,
		},
		{
			name:         "Enabled + Low Altitude -> No Fire",
			essayEnabled: true,
			altitudeAGL:  1000,
			expectFire:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update config for this run
			cfg.Narrator.Essay.Enabled = tt.essayEnabled
			simC := &mockJobSimClient{state: sim.StateActive}
			job := NewNarrationJob(cfg, mockN, pm, simC, nil, nil, nil)

			tel := &sim.Telemetry{
				AltitudeAGL: tt.altitudeAGL,
				IsOnGround:  false,
				Latitude:    48.0,
				Longitude:   -123.0,
			}
			job.lastTime = time.Time{} // expired cooldown
			job.takeoffTime = time.Now().Add(-10 * time.Minute)

			if got := job.CanPrepareEssay(tel); got != tt.expectFire {
				t.Errorf("CanPrepareEssay() = %v, want %v", got, tt.expectFire)
			}
		})
	}
}
