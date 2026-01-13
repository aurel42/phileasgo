package narrator

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"phileasgo/pkg/sim"
)

func determineFlightStage(t *sim.Telemetry) string {
	if t == nil {
		return "Unknown"
	}
	if t.IsOnGround {
		return "Ground"
	}
	// Simple heuristics for airborne phases
	if t.AltitudeAGL < 2000 {
		if t.VerticalSpeed > 300 {
			return "Takeoff/Climb"
		}
		if t.VerticalSpeed < -300 {
			return "Approach/Landing"
		}
	}
	return "Cruise"
}

func generateFlightStatusSentence(t *sim.Telemetry) string {
	if t == nil {
		return "The aircraft position is unknown."
	}

	lat := fmt.Sprintf("%.4f", t.PredictedLatitude)
	lon := fmt.Sprintf("%.4f", t.PredictedLongitude)

	if t.IsOnGround {
		action := "sitting"
		if t.GroundSpeed >= 2.0 {
			action = "taxiing"
		}
		return fmt.Sprintf("The aircraft is %s on the ground. Its position is %s, %s.", action, lat, lon)
	}

	// Flying
	alt := t.AltitudeAGL
	var altStr string
	if alt < 1000 {
		rounded := math.Round(alt/100) * 100
		altStr = fmt.Sprintf("%.0f", rounded)
	} else {
		rounded := math.Round(alt/1000) * 1000
		altStr = fmt.Sprintf("%.0f", rounded)
	}

	speed := fmt.Sprintf("%.0f", t.GroundSpeed)
	hdg := fmt.Sprintf("%.0f", t.Heading)

	return fmt.Sprintf("The aircraft is cruising about %s ft over the ground, moving at %s knots in heading %s. Its position is %s, %s.",
		altStr, speed, hdg, lat, lon)
}

func (s *AIService) fetchUnitsInstruction() string {
	unitSys := s.cfg.Narrator.Units
	if unitSys == "" {
		unitSys = "hybrid"
	}
	// Template name: units/hybrid.tmpl
	tmplName := "units/" + strings.ToLower(unitSys) + ".tmpl"

	out, err := s.prompts.Render(tmplName, nil)
	if err != nil {
		slog.Warn("Narrator: Failed to load units template", "system", unitSys, "error", err)
		return ""
	}
	return out
}
