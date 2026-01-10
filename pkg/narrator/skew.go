package narrator

import (
	"math"
	"math/rand"
	"phileasgo/pkg/model"
)

// Skew Strategy Constants
const (
	StrategyMinSkew = "min_skew"
	StrategyMaxSkew = "max_skew"
	StrategyUniform = "uniform"
	StrategyFixed   = "fixed"
)

// POIAnalyzer interface defines what we need to analyze competition.
type POIAnalyzer interface {
	CountScoredAbove(threshold float64, limit int) int
}

// DetermineSkewStrategy determines the skew strategy based on POI competition (density).
func DetermineSkewStrategy(p *model.POI, analyzer POIAnalyzer) string {
	if p == nil {
		return StrategyUniform // Default if no POI context
	}

	// Dynamic Length Logic: Relative Dominance
	// "Rivals" are POIs above a threshold: max(20% of winner's score, 0.5 absolute).
	// The 0.5 floor ensures low-scoring areas still have meaningful competition.
	// Note: CountScoredAbove includes the winner itself if score > 0.
	threshold := math.Max(p.Score*0.2, 0.5)

	// We only need to know if there are > 1 rivals (so limit=2 is sufficient to know if count >= 2)
	rivals := analyzer.CountScoredAbove(threshold, 2)

	// If rivals > 1 (Winner + at least 1 other) -> Skew Short (High Competition)
	if rivals > 1 {
		return StrategyMinSkew
	}
	// Winner is alone -> Skew Long (Lone Wolf)
	return StrategyMaxSkew
}

// SampleSkewedValue picks a value from [minVal, maxVal] using the specified strategy.
// It generates a pool of 3 random values and picks according to strategy.
func SampleSkewedValue(minVal, maxVal int, strategy string) int {
	if maxVal <= minVal {
		return minVal
	}

	if strategy == StrategyFixed {
		return minVal
	}

	// Helper to get a random value in range
	randomVal := func() int {
		steps := (maxVal - minVal) / 10
		if steps <= 0 {
			return minVal
		}
		step := rand.Intn(steps + 1)
		return minVal + step*10
	}

	// Pool Selection
	poolSize := 3
	pool := make([]int, poolSize)
	for i := 0; i < poolSize; i++ {
		pool[i] = randomVal()
	}

	var result int
	switch strategy {
	case StrategyMinSkew:
		// Pick smallest
		smallest := pool[0]
		for _, v := range pool {
			if v < smallest {
				smallest = v
			}
		}
		result = smallest
	case StrategyMaxSkew:
		// Pick largest
		largest := pool[0]
		for _, v := range pool {
			if v > largest {
				largest = v
			}
		}
		result = largest
	default: // StrategyUniform
		// Pick first (effectively random)
		result = pool[0]
	}
	return result
}
