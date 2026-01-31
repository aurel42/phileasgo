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
	s.initAssembler()
	enabled := s.cfg.Narrator.Debrief.Enabled
	summary := s.session().GetState().TripSummary

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
	if !s.canEnqueuePlayback("debrief", true) {
		slog.Info("Narrator: Debrief skipped (queue constraints)")
		return false
	}

	// 1.5 Sync Checks
	if s.HasPendingGeneration() {
		slog.Info("Narrator: Debrief skipped (priority jobs pending)")
		return false
	}

	slog.Info("Narrator: Generating Landing Debrief...")

	// 2. Build Prompt
	data := s.promptAssembler.NewPromptData(s.getSessionState())
	data["MaxWords"] = s.promptAssembler.ApplyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthLongWords)
	data["TripSummary"] = summary // Use the local copy we took with RLock

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
			MaxWords: s.promptAssembler.ApplyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthLongWords),
			Manual:   true, // Debriefs are treated as high priority / manual-like
		}

		slog.Info("Narrator: Generating Debrief", "max_words", req.MaxWords)

		narrative, err := s.GenerateNarrative(genCtx, &req)
		if err != nil {
			slog.Error("Narrator: Debrief generation failed", "error", err)
			return
		}

		// Enqueue (High Priority)
		s.enqueuePlayback(narrative, true)
		go s.ProcessPlaybackQueue(genCtx)
	}()

	return true
}
