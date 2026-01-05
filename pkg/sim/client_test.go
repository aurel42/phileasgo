package sim_test

import (
	"encoding/json"
	"testing"

	"phileasgo/pkg/sim"
)

func TestTelemetry_JSONMarshaling(t *testing.T) {

	tests := []struct {
		name  string
		input sim.Telemetry
	}{
		{
			name: "AllFieldsSet",
			input: sim.Telemetry{
				Latitude:      51.5074,
				Longitude:     -0.1278,
				AltitudeMSL:   1000.5,
				Heading:       90.0,
				GroundSpeed:   100.0,
				VerticalSpeed: 500.0,
				IsOnGround:    false,
			},
		},
		{
			name:  "ZeroValues",
			input: sim.Telemetry{
				// No SimTime needed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded sim.Telemetry
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Validate Fields
			if decoded.Latitude != tt.input.Latitude {
				t.Errorf("Latitude: got %v, want %v", decoded.Latitude, tt.input.Latitude)
			}
			if decoded.IsOnGround != tt.input.IsOnGround {
				t.Errorf("IsOnGround: got %v, want %v", decoded.IsOnGround, tt.input.IsOnGround)
			}
			if decoded.IsOnGround != tt.input.IsOnGround {
				t.Errorf("IsOnGround: got %v, want %v", decoded.IsOnGround, tt.input.IsOnGround)
			}
		})
	}
}
