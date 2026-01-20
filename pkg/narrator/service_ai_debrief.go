package narrator

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// PlayDebrief triggers a post-landing debrief narration.
func (s *AIService) PlayDebrief(ctx context.Context, tel *sim.Telemetry) bool {
	s.mu.RLock()
	enabled := s.cfg.Narrator.Debrief.Enabled
	summary := s.tripSummary
	s.mu.RUnlock()

	if !enabled {
		return false
	}

	// if active { ... } // removed busy check

	// Double check summary length
	if len(summary) < 50 {
		slog.Info("Narrator: Debrief requested but trip summary is too short.", "length", len(summary))
		return false
	}

	// Queue Constraints
	if !s.canEnqueue("debrief", true) {
		slog.Info("Narrator: Debrief skipped (queue constraints)")
		return false
	}

	// 1.5 Sync Checks
	if s.HasPendingPriority() {
		slog.Info("Narrator: Debrief skipped (priority jobs pending)")
		return false
	}

	slog.Info("Narrator: Generating Landing Debrief...")

	// 2. Build Prompt
	data := struct {
		TourGuideName string
		Persona       string
		Accent        string
		TripSummary   string
		MaxWords      int
		Language_name string
		Language_code string
	}{
		TourGuideName: "Ava", // TODO: Config
		Persona:       "Intelligent, fascinating",
		Accent:        "Neutral",
		TripSummary:   summary,
		MaxWords:      s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthLongWords),
		Language_name: "English",
		Language_code: "en-US",
	}

	prompt, err := s.prompts.Render("narrator/debrief.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render debrief template", "error", err)
		return false
	}

	// 3. Generate Async (Unified Pipeline)
	go func() {
		// Use a detached context for generation
		genCtx := context.Background()

		req := GenerationRequest{
			Type:     model.NarrativeTypeDebrief,
			Prompt:   prompt,
			Title:    "Debrief",
			SafeID:   "debrief_" + time.Now().Format("20060102_150405"),
			MaxWords: s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthLongWords),
			Manual:   true, // Debriefs are treated as high priority / manual-like
		}

		slog.Info("Narrator: Generating Debrief", "max_words", req.MaxWords)

		narrative, err := s.GenerateNarrative(genCtx, &req)
		if err != nil {
			slog.Error("Narrator: Debrief generation failed", "error", err)
			return
		}

		// Enqueue (High Priority)
		s.enqueue(narrative, true)
		go s.processQueue(genCtx)
	}()

	return true
}
