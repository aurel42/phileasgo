package labels

import (
	"math"
	"phileasgo/pkg/geo"
)

// Constants for Heuristic Scoring
const (
	TargetLabelCount   = 100
	MinSeparationRatio = 0.3
	DirectionalBias    = 0.8 // 0.0=Neutral, 1.0=Strong logic to ignore behind

	// Population Thresholds for Tiering
	PopCity    = 10000
	PopTown    = 1000
	PopVillage = 100
)

// Scorer handles the mathematical heuristics for selecting "Global Anchor" labels.
type Scorer struct{}

// NewScorer creates a new Scorer.
func NewScorer() *Scorer {
	return &Scorer{}
}

// CalculateImportance computes the raw importance of a city based on Population and Name Length.
// Importance = Population / NameLength
// This penalizes long names (which clutter the map) while rewarding population density.
func (s *Scorer) CalculateImportance(population int, name string) float64 {
	length := float64(len(name))
	if length == 0 {
		return 0
	}
	// Linear penalty for name length.
	// E.g., Pop=100k, Len=5 -> 100000 / 5 = 20000
	// E.g., Pop=100k, Len=10 -> 100000 / 10 = 10000
	return float64(population) / length
}

// CalculateDirectionalWeight computes a multiplier (0.1 - 1.0) based on whether the city is ahead of the aircraft.
// It uses the dot product of the aircraft's heading vector and the vector to the city.
func (s *Scorer) CalculateDirectionalWeight(cityLat, cityLon, acLat, acLon, headingDeg float64) float64 {
	// 1. Convert Heading to Radians
	headingRad := headingDeg * (math.Pi / 180.0)

	// 2. Aircraft Heading Vector (Normalized)
	// In standard map coords: N=0 (0,1), E=90 (1,0)
	// math.Sin/Cos usually take 0 as East.
	// Navigation: Y axis is North. X axis is East.
	// Heading 0 (N) -> dx=0, dy=1
	// Heading 90 (E) -> dx=1, dy=0
	acVecX := math.Sin(headingRad)
	acVecY := math.Cos(headingRad)

	// 3. Vector to City (Simplified Equirectangular)
	// For "ahead check", strictly accurate Great Circle bearing isn't required.
	// Local approximation is sufficient and faster.
	dy := cityLat - acLat
	dx := (cityLon - acLon) * math.Cos(acLat*(math.Pi/180.0))

	// Normalize City Vector
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist == 0 {
		return 1.0 // City is exactly underneath
	}
	cityVecX := dx / dist
	cityVecY := dy / dist

	// 4. Dot Product
	// 1.0 = Directly Ahead
	// 0.0 = Abeam (Side)
	// -1.0 = Directly Behind
	dot := (acVecX * cityVecX) + (acVecY * cityVecY)

	// 5. Clamp and Bias
	// We want to severely penalize backend labels (-1.0) but keep abeam labels (0.0) reasonable.
	// Formula: Map [-1, 1] to [0.1, 1.0] ???
	// Actually:
	// If dot > 0 (Ahead): Weight = 1.0
	// If dot < 0 (Behind): Weight scales down to 0.1
	if dot >= 0 {
		return 1.0
	}

	// For behind items: linear fade from 1.0 (at abeam) to 0.1 (behind)
	// dot is negative here.
	weight := 1.0 + (dot * DirectionalBias) // e.g. 1.0 + (-1.0 * 0.8) = 0.2

	if weight < 0.1 {
		return 0.1
	}

	return weight
}

// CalculateFinalScore combines all factors.
func (s *Scorer) CalculateFinalScore(city *geo.City, acLat, acLon, heading float64) float64 {
	imp := s.CalculateImportance(city.Population, city.Name)
	dir := s.CalculateDirectionalWeight(city.Lat, city.Lon, acLat, acLon, heading)
	return imp * dir
}
