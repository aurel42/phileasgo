package terrain

import (
	"fmt"
	"log/slog"

	"phileasgo/pkg/geo"
)

// LOSChecker performs Line-of-Sight calculations.
type LOSChecker struct {
	elevation *ElevationProvider
}

// NewLOSChecker creates a new LOS checker.
func NewLOSChecker(e *ElevationProvider) *LOSChecker {
	return &LOSChecker{
		elevation: e,
	}
}

// IsVisible determines if there is a direct line-of-sight between two points.
// alt1Ft and alt2Ft are in FEET (MSL).
// stepSizeKM is the sampling resolution (e.g., 0.5 km).
func (l *LOSChecker) IsVisible(p1, p2 geo.Point, alt1Ft, alt2Ft, stepSizeKM float64) bool {
	if l.elevation == nil {
		return true // Fail open if no elevation data
	}

	distMters := geo.Distance(p1, p2)
	distKM := distMters / 1000.0

	if distKM < stepSizeKM {
		return true // Too close to be blocked
	}

	const earthRadiusKM = 6371.0
	const feetToMeters = 0.3048

	h1 := alt1Ft * feetToMeters
	h2 := alt2Ft * feetToMeters

	steps := int(distKM / stepSizeKM)
	if steps < 2 {
		steps = 2 // At least 2 samples
	}

	dLat := p2.Lat - p1.Lat
	dLon := p2.Lon - p1.Lon

	for i := 1; i < steps; i++ {
		t := float64(i) / float64(steps)

		lat := p1.Lat + dLat*t
		lon := p1.Lon + dLon*t

		groundElevM, err := l.elevation.GetElevation(lat, lon)
		if err != nil {
			slog.Debug("LOS Elevation lookup failed", "lat", lat, "lon", lon, "err", err)
			continue
		}

		lerpAlt := h1 + (h2-h1)*t

		x := distKM * t
		drop := (x * (distKM - x)) / (2 * earthRadiusKM) * 1000.0
		rayAlt := lerpAlt - drop

		// RELAXED LOS: Add a 50m tolerance to the check.
		// The ground must be strictly HIGHER than the ray + 50m to block it.
		// This accounts for ETOPO1 resolution inaccuracies and "grazing" shots.
		if float64(groundElevM) > rayAlt+50.0 {
			slog.Debug("LOS blocked by terrain",
				"step", i, "of", steps,
				"sample_lat", fmt.Sprintf("%.4f", lat),
				"sample_lon", fmt.Sprintf("%.4f", lon),
				"ground_m", groundElevM,
				"ray_alt_m", fmt.Sprintf("%.0f", rayAlt),
				"dist_km", fmt.Sprintf("%.1f", distKM))
			return false
		}
	}

	return true
}

// GetElevation returns the ground elevation in meters at the given coordinates.
func (l *LOSChecker) GetElevation(lat, lon float64) (float64, error) {
	if l.elevation == nil {
		return 0, nil
	}
	val, err := l.elevation.GetElevation(lat, lon)
	if err != nil {
		return 0, err
	}
	return float64(val), nil
}
