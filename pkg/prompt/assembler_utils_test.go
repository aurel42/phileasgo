package prompt

import (
	"strings"
	"testing"

	"phileasgo/pkg/sim"
)

func TestGenerateFlightStatusSentence(t *testing.T) {
	tests := []struct {
		name string
		tel  *sim.Telemetry
		want []string // Substrings to expect
	}{
		{
			name: "Sitting on Ground",
			tel: &sim.Telemetry{
				IsOnGround:         true,
				GroundSpeed:        1.5,
				PredictedLatitude:  10.12345,
				PredictedLongitude: 20.67891,
			},
			want: []string{"sitting on the ground", "10.1235, 20.6789"},
		},
		{
			name: "Taxiing on Ground",
			tel: &sim.Telemetry{
				IsOnGround:         true,
				GroundSpeed:        5.0,
				PredictedLatitude:  10.0,
				PredictedLongitude: 20.0,
			},
			want: []string{"taxiing on the ground"},
		},
		{
			name: "Flying Low Rounding",
			tel: &sim.Telemetry{
				IsOnGround:        false,
				AltitudeAGL:       840,
				GroundSpeed:       100,
				Heading:           90,
				PredictedLatitude: 10, PredictedLongitude: 20,
			},
			want: []string{"cruising about 800 ft", "moving at 100 knots"},
		},
		{
			name: "Flying High Rounding",
			tel: &sim.Telemetry{
				IsOnGround:        false,
				AltitudeAGL:       5400,
				GroundSpeed:       120,
				Heading:           180,
				PredictedLatitude: 10, PredictedLongitude: 20,
			},
			want: []string{"cruising about 5000 ft"},
		},
		{
			name: "Flying High Rounding Up",
			tel: &sim.Telemetry{
				IsOnGround:        false,
				AltitudeAGL:       5600,
				GroundSpeed:       120,
				Heading:           180,
				PredictedLatitude: 10, PredictedLongitude: 20,
			},
			want: []string{"cruising about 6000 ft"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateFlightStatusSentence(tt.tel)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("GenerateFlightStatusSentence() = %q, want substring %q", got, w)
				}
			}
		})
	}
}
