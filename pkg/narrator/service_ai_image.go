package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"strings"
	"time"
)

// PlayImage handles the analysis and narration of a screenshot.
func (s *AIService) PlayImage(ctx context.Context, imagePath string, tel *sim.Telemetry) {
	if s.IsPaused() {
		slog.Info("Narrator: Skipping screenshot (paused)")
		return
	}

	// Gather Context
	if tel == nil {
		t, err := s.sim.GetTelemetry(ctx)
		if err != nil {
			slog.Error("Narrator: Failed to get telemetry for screenshot", "error", err)
			return
		}
		tel = &t
	}

	loc := s.geoSvc.GetLocation(tel.Latitude, tel.Longitude)

	// Prepare Prompt
	data := map[string]any{
		"City":        loc.CityName,
		"Region":      loc.Admin1Name,
		"Country":     loc.CountryCode,
		"MaxWords":    s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthLongWords), // Use LongWords for template context
		"TripSummary": s.getTripSummary(),
		"Lat":         fmt.Sprintf("%.3f", tel.Latitude),
		"Lon":         fmt.Sprintf("%.3f", tel.Longitude),
		"Alt":         fmt.Sprintf("%.0f", tel.AltitudeAGL),
	}

	prompt, err := s.prompts.Render("narrator/screenshot.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render screenshot prompt", "error", err)
		return
	}

	// 4. Generate Async (Unified Pipeline)
	go func() {
		// Use a detached context for generation
		genCtx := context.Background()

		req := GenerationRequest{
			Type:      model.NarrativeTypeScreenshot,
			Prompt:    prompt,
			Title:     "Screenshot Analysis",
			SafeID:    "screenshot_" + time.Now().Format("150405"),
			ImagePath: imagePath,
			MaxWords:  s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthShortWords), // Target for multimodal description
			Manual:    true,                                                                  // Screenshots are user-initiated
		}

		slog.Info("Narrator: Generating Screenshot Narrative", "image", imagePath)

		narrative, err := s.GenerateNarrative(genCtx, &req)
		if err != nil {
			slog.Error("Narrator: Screenshot generation failed", "image", imagePath, "error", err)
			return
		}

		// Filter markdown artifacts
		narrative.Script = strings.ReplaceAll(narrative.Script, "*", "")

		// Enqueue (High Priority)
		s.enqueuePlayback(narrative, true)
		go s.ProcessPlaybackQueue(genCtx)
	}()
}
