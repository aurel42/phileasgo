package narrator

import (
	"phileasgo/pkg/sim"
	"testing"
)

func TestDetermineFlightStage(t *testing.T) {
	tests := []struct {
		name      string
		telemetry *sim.Telemetry
		expected  string
	}{
		{
			name:      "Nil Telemetry",
			telemetry: nil,
			expected:  "Unknown",
		},
		{
			name: "On Ground",
			telemetry: &sim.Telemetry{
				IsOnGround: true,
			},
			expected: "Ground",
		},
		{
			name: "Takeoff/Climb",
			telemetry: &sim.Telemetry{
				IsOnGround:    false,
				AltitudeAGL:   1500,
				VerticalSpeed: 500,
			},
			expected: "Takeoff/Climb",
		},
		{
			name: "Approach/Landing",
			telemetry: &sim.Telemetry{
				IsOnGround:    false,
				AltitudeAGL:   1500,
				VerticalSpeed: -500,
			},
			expected: "Approach/Landing",
		},
		{
			name: "Low Altitude Cruise",
			telemetry: &sim.Telemetry{
				IsOnGround:    false,
				AltitudeAGL:   1500,
				VerticalSpeed: 0,
			},
			expected: "Cruise",
		},
		{
			name: "High Altitude Cruise",
			telemetry: &sim.Telemetry{
				IsOnGround:  false,
				AltitudeAGL: 5000,
			},
			expected: "Cruise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineFlightStage(tt.telemetry)
			if got != tt.expected {
				t.Errorf("determineFlightStage() = %v, want %v", got, tt.expected)
			}
		})
	}
}
