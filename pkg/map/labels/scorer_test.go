package labels

import (
	"math"
	"phileasgo/pkg/geo"
	"testing"
)

func TestCalculateImportance(t *testing.T) {
	s := NewScorer()

	tests := []struct {
		name     string
		pop      int
		cityName string
		wantMin  float64 // Minimum expected score
	}{
		{"Small Name, High Pop", 100000, "Ulm", 10000.0},          // 100k / 9 = 11111
		{"Long Name, High Pop", 100000, "Mönchengladbach", 390.0}, // 100k / 256 = 390.6 (len=16, 16^2=256)
		{"Tiny Pop, Short Name", 1000, "A", 1000.0},               // 1000 / 1 = 1000
		{"Zero Length Name", 1000, "", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.CalculateImportance(tt.pop, tt.cityName)
			if got < tt.wantMin {
				t.Errorf("CalculateImportance(%d, %s) = %f; want >= %f", tt.pop, tt.cityName, got, tt.wantMin)
			}
		})
	}
}

func TestCalculateDirectionalWeight(t *testing.T) {
	s := NewScorer()

	// Aircraft at (0,0) facing North (0 deg)
	acLat, acLon := 0.0, 0.0
	heading := 0.0

	tests := []struct {
		name    string
		cityLat float64
		cityLon float64
		want    float64
		desc    string
	}{
		{"Directly Ahead", 1.0, 0.0, 1.0, "North of AC"},
		{"Directly Behind", -1.0, 0.0, 0.2, "South of AC (should be penalized)"},
		{"Abeam East", 0.0, 1.0, 1.0, "East of AC (Neutral/Ahead logic)"}, // Dot product is 0, so >=0 rule applies -> 1.0
		{"Abeam West", 0.0, -1.0, 1.0, "West of AC"},
		{"Behind Left", -1.0, -1.0, 0.43, "South West (Partial Penalty)"}, // Dot approx -0.707
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.CalculateDirectionalWeight(tt.cityLat, tt.cityLon, acLat, acLon, heading)

			// For specific "Behind" cases, we check approximate values
			if tt.name == "Directly Behind" {
				// 1.0 + (-1.0 * 0.8) = 0.2
				if math.Abs(got-0.2) > 0.01 {
					t.Errorf("Directly Behind = %f; want 0.2", got)
				}
			} else if tt.name == "Directly Ahead" {
				if got != 1.0 {
					t.Errorf("Directly Ahead = %f; want 1.0", got)
				}
			}
		})
	}
}

func TestCalculateFinalScore(t *testing.T) {
	s := NewScorer()
	city := geo.City{Name: "Ulm", Population: 100000, Lat: 10, Lon: 10}

	// Aircraft approaching Ulm
	scoreAhead := s.CalculateFinalScore(&city, 9, 10, 0) // South of Ulm, facing North (Towards)

	// Aircraft passed Ulm
	scoreBehind := s.CalculateFinalScore(&city, 11, 10, 0) // North of Ulm, facing North (Away)

	if scoreAhead <= scoreBehind {
		t.Errorf("Score Ahead (%f) should be > Score Behind (%f)", scoreAhead, scoreBehind)
	}
}
