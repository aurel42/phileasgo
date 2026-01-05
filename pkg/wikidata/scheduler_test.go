package wikidata

import (
	"testing"
)

func TestScheduler_MaxDist(t *testing.T) {
	// Setup: Max Distance 15km
	// Grid spacing is ~17.3km (radius 10 * sqrt(3))
	// So only the generic start tile (distance 0) should be within 15km.
	// Neighbors are at ~17.3km, so they should be excluded if strict.

	maxDist := 15.0
	s := NewScheduler(maxDist)

	// Fetch candidates
	candidates := s.GetCandidates(0, 0, 0, false)

	// Verify
	for _, c := range candidates {
		if c.Dist > maxDist {
			t.Errorf("Candidate tile %v is at distance %.2f km, which exceeds max distance %.2f km", c.Tile, c.Dist, maxDist)
		}
	}

	// Now try with a larger distance that should include neighbors
	maxDist2 := 20.0
	s2 := NewScheduler(maxDist2)
	candidates2 := s2.GetCandidates(0, 0, 0, false)

	foundNeighbor := false
	for _, c := range candidates2 {
		if c.Dist > 0 && c.Dist <= maxDist2 {
			foundNeighbor = true
		}
		if c.Dist > maxDist2 {
			t.Errorf("Candidate tile %v is at distance %.2f km, which exceeds max distance %.2f km", c.Tile, c.Dist, maxDist2)
		}
	}

	if !foundNeighbor {
		t.Errorf("Expected to find neighbors within %.2f km, but found only start tile or nothing", maxDist2)
	}
}
