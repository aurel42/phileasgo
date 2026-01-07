package wikidata

import (
	"math"

	"github.com/uber/h3-go/v4"
)

const (
	// H3 Resolution 5
	h3Resolution = 5
)

// Grid handles H3 grid calculations.
type Grid struct{}

// NewGrid creates a new Grid instance.
func NewGrid() *Grid {
	return &Grid{}
}

// TileAt returns the H3 cell for the given coordinate.
func (g *Grid) TileAt(lat, lon float64) HexTile {
	ll := h3.NewLatLng(lat, lon)
	cell, err := h3.LatLngToCell(ll, h3Resolution)
	if err != nil {
		return HexTile{} // Handle error gracefully (empty index)
	}
	return HexTile{Index: cell.String()}
}

// TileCenter returns the Lat/Lon center of a hex tile.
func (g *Grid) TileCenter(tile HexTile) (lat, lon float64) {
	if tile.Index == "" {
		return 0, 0
	}
	cell := h3.CellFromString(tile.Index)
	if cell == 0 {
		return 0, 0
	}

	ll, err := h3.CellToLatLng(cell)
	if err != nil {
		return 0, 0
	}
	return ll.Lat, ll.Lng
}

// Neighbors returns the immediate neighbors (k=1) for a given tile.
func (g *Grid) Neighbors(t HexTile) []HexTile {
	if t.Index == "" {
		return nil
	}
	cell := h3.CellFromString(t.Index)
	if cell == 0 {
		return nil
	}

	// GridDisk(1) returns the origin cell plus its 6 neighbors
	disk, err := h3.GridDisk(cell, 1) // Returns ([]Cell, error)
	if err != nil {
		return nil
	}

	var neighbors []HexTile
	for _, c := range disk {
		// Filter out the origin cell
		if c == cell {
			continue
		}
		neighbors = append(neighbors, HexTile{Index: c.String()})
	}
	return neighbors
}

// TileRadius returns the distance in km from the cell center to its farthest vertex.
// This is used to determine the exact SPARQL query radius needed to cover the tile.
func (g *Grid) TileRadius(t HexTile) float64 {
	if t.Index == "" {
		return 0
	}
	cell := h3.CellFromString(t.Index)
	if cell == 0 {
		return 0
	}

	// 1. Get Center
	centerLL, err := h3.CellToLatLng(cell)
	if err != nil {
		return 0
	}

	// 2. Get Boundary (Vertices)
	boundary, err := h3.CellToBoundary(cell)
	if err != nil {
		return 0
	}

	// 3. Find Max Distance
	maxDist := 0.0
	for _, v := range boundary {
		dist := DistKm(centerLL.Lat, centerLL.Lng, v.Lat, v.Lng)
		if dist > maxDist {
			maxDist = dist
		}
	}
	return maxDist
}

// DistKm calculates approximate distance between two points (Haversine approx for small distances).
func DistKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * 111.132
	dLon := (lon2 - lon1) * 111.132 * math.Cos((lat1+lat2)*math.Pi/360.0)
	return math.Sqrt(dLat*dLat + dLon*dLon)
}
