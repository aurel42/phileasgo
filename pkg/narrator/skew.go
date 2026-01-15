package narrator

import (
	"math"
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
// When isOnGround is true, always returns StrategyMaxSkew since the airport is the only
// viable POI and deserves full narration.
func DetermineSkewStrategy(p *model.POI, analyzer POIAnalyzer, isOnGround bool) string {
	if p == nil {
		return StrategyUniform // Default if no POI context
	}

	// On ground, force max_skew - airport is the only viable POI
	if isOnGround {
		return StrategyMaxSkew
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
