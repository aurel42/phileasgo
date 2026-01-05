package wikidata

import (
	"testing"
)

func TestGrid_TileAt(t *testing.T) {
	g := NewGrid()

	tests := []struct {
		name     string
		lat, lon float64
		wantRow  int
		wantCol  int
	}{
		{"Origin", 0, 0, 0, 0},
		{"Near Origin Positive", 0.05, 0.05, 0, 0},
		// rowHeight ~ 15km (1.5 * 10). 0.05 deg lat is ~5km.
		{"Next Row North", 0.2, 0.0, 1, -1},
		// 0.2 deg * 111 = 22km. > 15km row height?
		// rowHeight = 15km. latStep = 15/111 = ~0.135 deg.
		// So 0.2 should be row 1 or 2. 0.2/0.135 = 1.48 -> round to 1.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tile := g.TileAt(tt.lat, tt.lon)
			if tile.Row != tt.wantRow || tile.Col != tt.wantCol {
				t.Errorf("TileAt(%v, %v) = (%v, %v), want (%v, %v)",
					tt.lat, tt.lon, tile.Row, tile.Col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestGrid_Neighbors(t *testing.T) {
	g := NewGrid()
	center := HexTile{Row: 0, Col: 0}
	neighbors := g.Neighbors(center)

	if len(neighbors) != 6 {
		t.Errorf("Expected 6 neighbors, got %d", len(neighbors))
	}

	// Verify one expected neighbor (Eve row logic: {0, -1} -> {0, -1})
	found := false
	for _, n := range neighbors {
		if n.Row == 0 && n.Col == -1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected neighbor (0, -1) not found")
	}
}
