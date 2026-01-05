package wikidata

import (
	"math"
)

const (
	// Grid Configuration
	radiusKm  = 10.0
	spacingKm = radiusKm * 1.73205080757 // radius * sqrt(3)

	// Earth Constants
	degPerKm = 1.0 / 111.132
)

// Grid handles the hexagonal grid calculations.
type Grid struct {
	rowHeightKm float64
	latStepDeg  float64
}

// NewGrid creates a new Grid instance.
func NewGrid() *Grid {
	// Vertical spacing between rows = 1.5 * Radius
	rowHeight := 1.5 * radiusKm
	return &Grid{
		rowHeightKm: rowHeight,
		latStepDeg:  rowHeight * degPerKm,
	}
}

// TileAt returns the hex tile containing the given coordinate.
func (g *Grid) TileAt(lat, lon float64) HexTile {
	// Row calculation: Lat = row * lat_step
	row := int(math.Round(lat / g.latStepDeg))

	// Col calculation depends on lat for longitude spacing
	centerLat := g.rowLat(row)
	lonStep := g.lonStep(centerLat)

	// Odd rows offset by half a step
	offset := 0.0
	if row%2 != 0 {
		offset = lonStep / 2.0
	}

	col := int(math.Round((lon - offset) / lonStep))

	return HexTile{Row: row, Col: col}
}

// TileCenter returns the Lat/Lon center of a hex tile.
func (g *Grid) TileCenter(tile HexTile) (lat, lon float64) {
	lat = g.rowLat(tile.Row)
	lon = g.lonStep(lat) * float64(tile.Col)

	if tile.Row%2 != 0 {
		lon += g.lonStep(lat) / 2
	}
	return lat, lon
}

// Neighbors returns the 6 existing neighbors for a given tile.
// Note: This logic assumes infinite grid, which is effectively true for world map.
// If needed we can cap at row limits.
func (g *Grid) Neighbors(t HexTile) []HexTile {
	var neighbors []HexTile

	// Hex grid neighbor offsets depend on row parity
	isOdd := (t.Row % 2) != 0

	var offsets [][2]int
	if !isOdd {
		// Even row
		offsets = [][2]int{
			{0, -1}, {0, 1},
			{-1, -1}, {-1, 0},
			{1, -1}, {1, 0},
		}
	} else {
		// Odd row
		offsets = [][2]int{
			{0, -1}, {0, 1},
			{-1, 0}, {-1, 1},
			{1, 0}, {1, 1},
		}
	}

	for _, o := range offsets {
		neighbors = append(neighbors, HexTile{Row: t.Row + o[0], Col: t.Col + o[1]})
	}
	return neighbors
}

// -- Helpers --

func (g *Grid) rowLat(row int) float64 {
	return float64(row) * g.latStepDeg
}

func (g *Grid) lonStep(lat float64) float64 {
	// Clamp lat to avoid poles div/0 issues
	safeLat := math.Max(-89.0, math.Min(89.0, lat))
	rad := safeLat * (math.Pi / 180.0)

	// Horizontal spacing for hex centers = sqrt(3) * R = spacingKm
	return spacingKm / (111.132 * math.Cos(rad))
}

// DistKm calculates approximate distance between two points.
func DistKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * 111.132
	dLon := (lon2 - lon1) * 111.132 * math.Cos((lat1+lat2)*math.Pi/360.0)
	return math.Sqrt(dLat*dLat + dLon*dLon)
}
