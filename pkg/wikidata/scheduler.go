package wikidata

import (
	"math"
	"sort"
)

// Scheduler determines the next tile to fetch.
type Scheduler struct {
	grid      *Grid
	maxDistKm float64
}

// NewScheduler creates a new scheduler.
func NewScheduler(maxDistKm float64) *Scheduler {
	return &Scheduler{
		grid:      NewGrid(),
		maxDistKm: maxDistKm,
	}
}

// Candidate represents a potential tile to fetch.
type Candidate struct {
	Tile HexTile
	Lat  float64
	Lon  float64
	Dist float64
}

// GetCandidates returns a list of tiles sorted by priority (distance/cone) to check for fetching.
// Returns a list of candidates.
func (s *Scheduler) GetCandidates(lat, lon, heading float64, isAirborne bool) []Candidate {
	startTile := s.grid.TileAt(lat, lon)

	// Spiral search to find candidates within maxRadius
	// 1. Generate Spiral
	// 2. Filter by max radius
	// 3. Apply Cone filter if airborne (unless it's the start tile)

	visited := make(map[HexTile]bool)
	queue := []HexTile{startTile}
	visited[startTile] = true

	var candidates []Candidate

	// Pre-calculate limit
	limitDist := s.maxDistKm + spacingKm

	// We use a simple BFS for spiral
	head := 0
	for head < len(queue) {
		curr := queue[head]
		head++

		cLat, cLon := s.grid.TileCenter(curr)
		dist := DistKm(lat, lon, cLat, cLon)

		if dist > limitDist {
			continue // Stop expanding this branch
		}

		// Add neighbors to queue
		for _, n := range s.grid.Neighbors(curr) {
			if !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}

		// Filter Logic
		isValid := true
		if dist > s.maxDistKm {
			isValid = false
		} else if isAirborne {
			// Cone Check: +/- 30 degrees (Total 60)
			// Exception: Always include the tile we are currently ON
			if curr != startTile {
				bearing := calculateBearing(lat, lon, cLat, cLon)
				diff := math.Abs(bearing - heading)
				if diff > 180 {
					diff = 360 - diff
				}
				if diff > 30.0 { // 30 degrees half-arc for 60 degree cone
					isValid = false
				}
			}
		}

		if isValid {
			candidates = append(candidates, Candidate{
				Tile: curr,
				Lat:  cLat,
				Lon:  cLon,
				Dist: dist,
			})
		}
	}

	// Sort by Distance
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Dist < candidates[j].Dist
	})

	return candidates
}

func calculateBearing(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := (lon2 - lon1) * math.Pi / 180.0
	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0

	y := math.Sin(dLon) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(dLon)
	brng := math.Atan2(y, x)

	brngDeg := brng * 180.0 / math.Pi
	return math.Mod(brngDeg+360.0, 360.0) // Normalize to 0-360
}
