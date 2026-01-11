package visibility

import (
	"fmt"
	"math"
)

// Calculator handles visibility logic
type Calculator struct {
	manager *Manager
}

// NewCalculator creates a new visibility calculator
func NewCalculator(m *Manager) *Calculator {
	return &Calculator{
		manager: m,
	}
}

// CalculateVisibility returns a score from 0.0 to 1.0 representing visibility
// heading: aircraft heading in degrees (0-360)
// altAGL: aircraft altitude above ground level in feet
// bearing: bearing from aircraft to target in degrees (0-360)
// distNM: distance from aircraft to target in nautical miles
// isOnGround: whether the aircraft is on the ground
func (c *Calculator) CalculateVisibility(heading, altAGL, bearing, distNM float64, isOnGround bool) float64 {
	// 1. Get max visible distance for "Medium" size (standard reference for map overlay)
	maxDist := c.manager.GetMaxVisibleDist(altAGL, SizeM)
	if maxDist <= 0 || distNM > maxDist {
		return 0.0
	}

	// 2. Initial score based on distance decay
	score := c.calculateDistanceDecay(distNM, maxDist)

	// 3. Blind Spot & Bearing Logic (Airborne Only)
	if !isOnGround {
		relBearing := normalizeBearing(bearing - heading)

		// Blind spot check
		if isBlindSpot(altAGL, distNM, relBearing) {
			return 0.0 // Totally hidden under nose for overlay
		}

		// Bearing Multipliers
		score *= getBearingMultiplier(relBearing)
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// CalculateVisibilityForSize returns a visibility score for a specific size category.
// CalculateVisibilityForSize returns a visibility score for a specific size category.
// Considers both Real AGL and Effective AGL. Max score is 1.0.
func (c *Calculator) CalculateVisibilityForSize(heading, realAGL, effectiveAGL, bearing, distNM float64, size SizeType, isOnGround bool) float64 {
	scoreReal := c.calculateOverlayScore(heading, realAGL, bearing, distNM, size, isOnGround)

	if !isOnGround && effectiveAGL > realAGL+10.0 {
		scoreEff := c.calculateOverlayScore(heading, effectiveAGL, bearing, distNM, size, isOnGround)
		if scoreEff > scoreReal {
			return scoreEff
		}
	}
	return scoreReal
}

func (c *Calculator) calculateOverlayScore(heading, altAGL, bearing, distNM float64, size SizeType, isOnGround bool) float64 {
	// 1. Get max visible distance for specified size
	maxDist := c.manager.GetMaxVisibleDist(altAGL, size)
	if maxDist <= 0 || distNM > maxDist {
		return 0.0
	}

	// 2. Initial score based on distance decay
	score := c.calculateDistanceDecay(distNM, maxDist)

	// 3. Blind Spot & Bearing Logic (Airborne Only)
	if !isOnGround {
		relBearing := normalizeBearing(bearing - heading)

		// Blind spot check
		if isBlindSpot(altAGL, distNM, relBearing) {
			return 0.0 // Totally hidden under nose for overlay
		}

		// Bearing Multipliers
		score *= getBearingMultiplier(relBearing)
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// CalculatePOIScore calculates the comprehensive visibility score for a POI.
// Returns the score multiplier and a log string explaining the factors.
// Considers both Real AGL and Effective AGL (Valley Depth) and uses the best perspective.
func (c *Calculator) CalculatePOIScore(heading, realAGL, effectiveAGL, bearing, distNM float64, size SizeType, isOnGround bool) (score float64, details string) {
	// 1. Calculate for Real AGL (Actual physics)
	sReal, dReal := c.calculateSingleScore(heading, realAGL, bearing, distNM, size, isOnGround)

	// 2. Calculate for Effective AGL (Valley Prominence)
	// Only if effective is significantly different to avoid redundant calc
	// And only if not on ground (valley logic applies to air)
	if !isOnGround && effectiveAGL > realAGL+10.0 { // +10ft buffer
		sEff, dEff := c.calculateSingleScore(heading, effectiveAGL, bearing, distNM, size, isOnGround)

		if sEff > sReal {
			return sEff, fmt.Sprintf("%s\n[Valley Boost Applied: RealAGL %.0f -> EffAGL %.0f]", dEff, realAGL, effectiveAGL)
		}
	}

	return sReal, dReal
}

// calculateSingleScore computes visibility score for a specific altitude context
func (c *Calculator) calculateSingleScore(heading, altAGL, bearing, distNM float64, size SizeType, isOnGround bool) (score float64, details string) {
	// 1. Get max visible distance
	maxDist := c.manager.GetMaxVisibleDist(altAGL, size)

	// Invisible check
	if maxDist <= 0 {
		return 0.0, fmt.Sprintf("Invisible (%s @ %.0fft)", size, altAGL)
	}
	if distNM > maxDist {
		return 0.0, fmt.Sprintf("Invisible (Too far: %.1f > %.1fnm)", distNM, maxDist)
	}

	// 2. Initial score based on distance decay
	visMult := c.calculateDistanceDecay(distNM, maxDist)
	score = visMult
	details = fmt.Sprintf("Visibility (%s@%.0fft): x%.2f", size, altAGL, visMult)

	// 3. Blind Spot & Bearing Logic (Airborne Only)
	if !isOnGround {
		relBearing := normalizeBearing(bearing - heading)

		// Check Blind Spot
		if isBlindSpot(altAGL, distNM, relBearing) {
			score *= 0.1
			details += "\nBlind Spot: x0.1 (Hidden by airframe)"
			return score, details
		}

		// Bearing Multipliers
		bearingMult := getBearingMultiplier(relBearing)
		if bearingMult != 1.0 {
			score *= bearingMult
			desc := getBearingDescription(relBearing)
			details += fmt.Sprintf("\nBearing: x%.2f (%s)", bearingMult, desc)
		}
	}

	return score, details
}

// Helpers

func (c *Calculator) calculateDistanceDecay(distNM, maxDist float64) float64 {
	ratio := distNM / maxDist
	score := 1.0 - ratio
	if score < 0 {
		score = 0
	}
	return score
}

func isBlindSpot(altAGL, distNM, relBearing float64) bool {
	blindRadius := 1.0 * math.Min((altAGL-50.0)/4950.0, 1.0)
	if altAGL < 500 {
		blindRadius = 0.1
	}

	// Blind spot is under nose (small radius) and Forward (+/- 90 deg)
	return distNM < blindRadius && math.Abs(relBearing) < 90
}

func normalizeBearing(b float64) float64 {
	for b > 180 {
		b -= 360
	}
	for b < -180 {
		b += 360
	}
	return b
}

func getBearingMultiplier(relBearing float64) float64 {
	rb360 := relBearing
	if rb360 < 0 {
		rb360 += 360
	}

	switch {
	case rb360 < 90:
		return 1.0 // Right Front
	case rb360 < 225:
		return 0.0 // Invisible behind aircraft
	case rb360 < 270:
		return 0.5 // Left Rear
	case rb360 < 300:
		return 1.5 // Left Side
	case rb360 < 330:
		return 2.0 // Best visibility (Left Front)
	default: // 330 onwards
		return 1.5 // Forward Left
	}
}

func getBearingDescription(relBearing float64) string {
	rb360 := relBearing
	if rb360 < 0 {
		rb360 += 360
	}

	switch {
	case rb360 < 90:
		return "Right Front"
	case rb360 < 225:
		return "Rear"
	case rb360 < 270:
		return "Left Rear"
	case rb360 < 300:
		return "Left Side"
	case rb360 < 330:
		return "Left Front (Best)"
	default:
		return "Forward Left"
	}
}
