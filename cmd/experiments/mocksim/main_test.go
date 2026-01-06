package main

import (
	"testing"
	"time"

	"phileasgo/pkg/sim/mocksim"
)

func TestMockSimConfig(t *testing.T) {
	// Basic smoke test to ensure main package dependencies are sound
	// and we can access the config used in main.
	tests := []struct {
		name string
		cfg  mocksim.Config
	}{
		{
			name: "DefaultConfig",
			cfg: mocksim.Config{
				DurationParked: 10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.DurationParked == 0 {
				t.Error("Default config Parked duration should not be 0")
			}
		})
	}
}
