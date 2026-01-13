package narrator

import (
	"fmt"
	"math"
	"strings"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// calculateNavInstruction generates the navigation phrase based on 4.5km rules.
func (s *AIService) calculateNavInstruction(p *model.POI, tel *sim.Telemetry) string {
	// Source coordinates: Use predicted if available (1 min ahead), else current
	latSrc, lonSrc := tel.Latitude, tel.Longitude
	if tel.PredictedLatitude != 0 || tel.PredictedLongitude != 0 {
		latSrc, lonSrc = tel.PredictedLatitude, tel.PredictedLongitude
	}

	pSrc := geo.Point{Lat: latSrc, Lon: lonSrc}
	pTarget := geo.Point{Lat: p.Lat, Lon: p.Lon}

	// Distance
	distMeters := geo.Distance(pSrc, pTarget)
	distKm := distMeters / 1000.0

	// Rule 1: Universal Threshold 4.5km
	if distKm < 4.5 {
		// Rule 2: No distance below 4.5km (Ground & Airborne)
		// Rule 2b: No direction if on ground below 4.5km

		// If ground:
		if tel.IsOnGround {
			return "" // No direction, no distance.
		}

		// If airborne:
		// Relative sectors only. No distance.
		return s.formatAirborneRelative(pSrc, pTarget, tel.Heading)
	}

	// >= 4.5km: Show Distance + Direction (Cardinal Ground, Clock Airborne)

	// Distance String
	var distStr string
	unitSys := strings.ToLower(s.cfg.Narrator.Units)

	if unitSys == "metric" || unitSys == "hybrid" {
		val := humanRound(distKm)
		distStr = fmt.Sprintf("about %.0f kilometers", val)
	} else {
		// Imperial
		distNm := distMeters * 0.000539957 // nautical miles actually? or statute miles?
		// "miles" usually implies statute miles in common parlance, but aviation uses NM.
		// logic uses 0.000539957 which IS Nautical Miles (1m = 0.000539957 NM).
		// Prompt says "miles". Let's assume we want NM but called "miles" or just "miles" as statute?
		// Existing code said "miles" with NM conversion. I'll stick to that but apply rounding.
		// Wait, user might want Statute Miles for "human" passengers?
		// 1 km = 0.621371 miles.
		// If I stick to existing conversion (NM), I just round that.
		val := humanRound(distNm)
		distStr = fmt.Sprintf("about %.0f miles", val)
	}

	if tel.IsOnGround {
		return s.formatGroundCardinal(pSrc, pTarget, distStr)
	}
	return s.formatAirborneClock(pSrc, pTarget, tel.Heading, distStr)
}

func (s *AIService) formatGroundCardinal(pSrc, pTarget geo.Point, distStr string) string {
	bearing := geo.Bearing(pSrc, pTarget)
	normBearing := math.Mod(bearing+360, 360)
	dirs := []string{"North", "North-East", "East", "South-East", "South", "South-West", "West", "North-West"}
	idx := int((normBearing+22.5)/45.0) % 8
	direction := fmt.Sprintf("to the %s", dirs[idx])

	return capitalizeStart(fmt.Sprintf("%s, %s away", direction, distStr))
}

func (s *AIService) formatAirborneRelative(pSrc, pTarget geo.Point, userHdg float64) string {
	bearing := geo.Bearing(pSrc, pTarget)
	relBearing := math.Mod(bearing-userHdg+360, 360)

	var direction string
	// 345-15: Straight Ahead
	// 15-135: Right
	// 135-225: Behind
	// 225-345: Left

	switch {
	case relBearing >= 345 || relBearing <= 15:
		direction = "straight ahead"
	case relBearing > 15 && relBearing <= 135:
		direction = "on your right"
	case relBearing > 135 && relBearing <= 225:
		direction = "behind you"
	case relBearing > 225 && relBearing < 345:
		direction = "on your left"
	}

	return capitalizeStart(direction)
}

func (s *AIService) formatAirborneClock(pSrc, pTarget geo.Point, userHdg float64, distStr string) string {
	bearing := geo.Bearing(pSrc, pTarget)
	relBearing := math.Mod(bearing-userHdg+360, 360)

	clock := int((relBearing + 15) / 30)
	if clock == 0 {
		clock = 12
	}
	direction := fmt.Sprintf("at your %d o'clock", clock)

	return capitalizeStart(fmt.Sprintf("%s, %s away", direction, distStr))
}

func capitalizeStart(s string) string {
	if s == "" {
		return ""
	}
	// Basic uppercase
	return strings.ToUpper(s[:1]) + s[1:]
}

// humanRound rounds the distance based on "human" perception rules.
// < 10 -> round to 1
// 10-100 -> round to 5
// > 100 -> round to 10
// Same rules apply for KM and Miles (magnitude is similar enough for this heuristic).
func humanRound(val float64) float64 {
	if val < 10 {
		return math.Round(val)
	}
	if val < 100 {
		return math.Round(val/5.0) * 5.0
	}
	return math.Round(val/10.0) * 10.0
}
