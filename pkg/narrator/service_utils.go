package narrator

import (
	"log/slog"
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
