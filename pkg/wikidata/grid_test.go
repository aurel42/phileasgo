package wikidata

import (
	"testing"
)

func TestGrid_TileAt(t *testing.T) {
	g := NewGrid()

	// 1. Check stability (same coord = same cell)
	lat, lon := 48.0, -122.0
	t1 := g.TileAt(lat, lon)
	t2 := g.TileAt(lat, lon)

	if t1.Index != t2.Index {
		t.Errorf("TileAt output unstable: %s vs %s", t1.Index, t2.Index)
	}

	// 2. Check nearby points (H3 Res 5 edge ~8.5km)
	// A point 1km away should be in the same cell (statistically likely unless on edge)
	// Let's verify center logic instead.

	latC, lonC := g.TileCenter(t1)
	tCenter := g.TileAt(latC, lonC)
	if tCenter.Index != t1.Index {
		t.Errorf("Center of tile %s maps to tile %s", t1.Index, tCenter.Index) // Should match
	}
}

func TestGrid_Neighbors(t *testing.T) {
	g := NewGrid()
	// Pick a known coordinate
	center := g.TileAt(48.0, -122.0)
	neighbors := g.Neighbors(center)

	if len(neighbors) != 6 {
		t.Errorf("Expected 6 neighbors, got %d", len(neighbors))
	}

	// Verify neighbors are distinct and not the center
	seen := make(map[string]bool)
	seen[center.Index] = true

	for _, n := range neighbors {
		if seen[n.Index] {
			t.Errorf("Duplicate neighbor or center returned: %s", n.Index)
		}
		seen[n.Index] = true
	}
}
