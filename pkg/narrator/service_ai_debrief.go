package narrator

import (
	"context"
	"log/slog"
	"time"

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
		MaxWords:      s.cfg.Narrator.NarrationLengthLongWords,
		Language_name: "English",
		Language_code: "en-US",
	}

	prompt, err := s.prompts.Render("narrator/debrief.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render debrief template", "error", err)
		return false
	}

	// 3. Generate Async
	go func() {
		// Use a detached context for generation
		genCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		s.mu.Lock()
		s.generating = true
		s.genCancelFunc = cancel
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			s.generating = false
			s.genCancelFunc = nil
			s.mu.Unlock()
		}()

		text, err := s.llm.GenerateText(genCtx, "debrief", prompt)
		if err != nil {
			slog.Error("Narrator: Failed to generate debrief", "error", err)
			return
		}

		// 4. Synthesize Audio
		audioPath, format, err := s.synthesizeAudio(genCtx, text, "landing_debrief")
		if err != nil {
			s.handleTTSError(err)
			slog.Error("Narrator: Failed to synthesize debrief audio", "error", err)
			return
		}

		// 5. Play
		narrative := &Narrative{
			Type:           "debrief",
			POI:            nil, // Debriefs don't have a POI
			Title:          "Landing Debrief",
			Script:         text,
			AudioPath:      audioPath,
			Format:         format,
			RequestedWords: s.cfg.Narrator.NarrationLengthLongWords,
		}

		// Use Queue (High Priority)
		s.enqueue(narrative, true)

		// Trigger queue
		go s.processQueue(context.Background())
	}()

	return true
}
