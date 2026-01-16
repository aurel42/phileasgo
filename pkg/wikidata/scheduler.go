package wikidata

import (
	"math"
	"sort"
)

const spacingKm = 5.6 // Approx center-to-center distance for H3 Res 6

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
	Tile        HexTile
	Lat         float64
	Lon         float64
	Dist        float64
	Cost        float64
	IsRedundant bool
}

// GetCandidates returns a list of tiles sorted by priority (Cost = Dist + RedundancyPenalty)
// to check for fetching.
func (s *Scheduler) GetCandidates(lat, lon, heading float64, isAirborne bool, recentTiles map[string]bool) []Candidate {
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

		// Process Candidate
		if cand, ok := s.processCandidate(curr, cLat, cLon, dist, lat, lon, heading, isAirborne, startTile, recentTiles); ok {
			candidates = append(candidates, cand)
		}
	}

	// Sort by Cost (lowest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Cost < candidates[j].Cost
	})

	return candidates
}

func (s *Scheduler) processCandidate(curr HexTile, cLat, cLon, dist, lat, lon, heading float64, isAirborne bool, startTile HexTile, recentTiles map[string]bool) (Candidate, bool) {
	// Filter Logic
	if dist > s.maxDistKm {
		return Candidate{}, false
	}

	deviation := 0.0
	if isAirborne {
		// Cone Check: +/- 60 degrees (Total 120)
		if curr != startTile && dist > 5.0 {
			bearing := calculateBearing(lat, lon, cLat, cLon)
			diff := math.Abs(bearing - heading)
			if diff > 180 {
				diff = 360 - diff
			}
			if diff > 60.0 { // 60 degrees half-arc for 120 degree cone
				return Candidate{}, false
			}
			deviation = diff
		}
	}

	c := Candidate{
		Tile: curr,
		Lat:  cLat,
		Lon:  cLon,
		Dist: dist,
	}

	// 2. Redundancy Check (Proximity to Cache)
	isCloseToCache := s.checkRedundancy(curr, recentTiles)

	redundancy := 0.0
	if isCloseToCache {
		redundancy = 1.0
		c.IsRedundant = true
	}

	// Cost Formula:
	// Base: Distance
	// Penalty 1: Redundancy (Dist + 5km penalty)
	// Penalty 2: Heading Deviation (Bonus for being dead-ahead)
	//            Factor 0.1 means 10 degrees off = +1km "virtual distance"
	//            This prioritizes tiles in front over closer ones to the side.
	c.Cost = c.Dist + (redundancy * (c.Dist*1.0 + 5.0)) + (deviation * 0.1)

	return c, true
}

func (s *Scheduler) checkRedundancy(curr HexTile, recentTiles map[string]bool) bool {
	if recentTiles[curr.Key()] {
		return true
	}
	// Check Neighbors
	for _, n := range s.grid.Neighbors(curr) {
		if recentTiles[n.Key()] {
			return true
		}
	}
	return false
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
