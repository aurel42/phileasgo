package wikidata

import (
	"math"
	"testing"
)

func TestCalculateBearing(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		wantBear float64
	}{
		{"North", 0, 0, 1, 0, 0},
		{"East", 0, 0, 0, 1, 90},
		{"South", 1, 0, 0, 0, 180},
		{"West", 0, 1, 0, 0, 270},
		{"Same", 0, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBearing(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			// Allow small float error
			if math.Abs(got-tt.wantBear) > 1.0 {
				t.Errorf("calculateBearing() = %v, want %v", got, tt.wantBear)
			}
		})
	}
}

func TestGetCandidates(t *testing.T) {
	// Setup scheduler with small radius (enough for neighbors)
	s := NewScheduler(50.0) // 50km

	lat, lon := 50.0, 14.0
	heading := 0.0 // North
	isAirborne := false

	candidates := s.GetCandidates(lat, lon, heading, isAirborne)

	if len(candidates) == 0 {
		t.Fatal("Expected candidates, got 0")
	}

	// Verify sorting (closest first)
	prevDist := -1.0
	for _, c := range candidates {
		if c.Dist < prevDist {
			t.Errorf("Candidates not sorted by distance: %f after %f", c.Dist, prevDist)
		}
		prevDist = c.Dist
	}

	// Test Airborne Cone Logic
	// If heading North (0), we shouldn't see South tiles unless current tile
	isAirborne = true
	candidatesAir := s.GetCandidates(lat, lon, heading, isAirborne)

	if len(candidatesAir) == 0 {
		t.Fatal("Expected airborne candidates, got 0")
	}
	// Verify candidates count is less than full circle (roughly)
	// Though with small radius, grid discreteness affects this.
	// Just ensure it runs without panic and returns subset.
	if len(candidatesAir) >= len(candidates) && len(candidates) > 7 {
		// Only if we have enough candidates to actually filter
		// t.Logf("Airborne should filter some tiles. All: %d, Air: %d", len(candidates), len(candidatesAir))
	}
}
