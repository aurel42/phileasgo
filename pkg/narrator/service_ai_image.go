package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"phileasgo/pkg/sim"
	"strings"
)

// PlayImage handles the analysis and narration of a screenshot.
// It generates a description using Gemini (multimodal), synthesizes audio,
// and queues it for playback via the standard narrator pipeline.
// Screenshots do NOT interrupt already-playing narrations; they queue behind current playback.
func (s *AIService) PlayImage(ctx context.Context, imagePath string, tel *sim.Telemetry) {
	if s.IsPaused() {
		slog.Info("Narrator: Skipping screenshot (paused)")
		return
	}

	// 1. Mark as generating to block other narrations
	s.mu.Lock()
	if s.generating {
		s.mu.Unlock()
		slog.Warn("Narrator: Skipping screenshot (already generating)")
		return
	}
	s.generating = true
	s.mu.Unlock()

	// Ensure we clear the generating flag eventually
	defer func() {
		s.mu.Lock()
		s.generating = false
		s.mu.Unlock()
	}()

	slog.Info("Narrator: Analyzing screenshot...", "path", imagePath)

	// 2. Gather Context (City)
	var city string
	if s.geoSvc != nil && tel != nil {
		info := s.geoSvc.GetLocation(tel.Latitude, tel.Longitude)
		city = info.CityName
	}

	// 3. Prepare Prompt
	data := map[string]any{
		"City":     city,
		"Lat":      fmt.Sprintf("%.3f", tel.Latitude),
		"Lon":      fmt.Sprintf("%.3f", tel.Longitude),
		"Alt":      fmt.Sprintf("%.0f", tel.AltitudeAGL),
		"MaxWords": s.cfg.Narrator.NarrationLengthLongWords,
	}

	prompt, err := s.prompts.Render("narrator/screenshot.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render screenshot prompt", "error", err)
		return
	}

	// 4. Call LLM (Multimodal)
	text, err := s.llm.GenerateImageText(ctx, "screenshot", prompt, imagePath)
	if err != nil {
		slog.Error("Narrator: Gemini analysis failed", "error", err)
		return
	}

	if text == "" {
		slog.Warn("Narrator: Gemini returned empty description")
		return
	}
	slog.Debug("Narrator: Screenshot described", "text", text)

	// 5. Check if we are still valid/active? (Maybe user paused in between?)
	if s.IsPaused() {
		return
	}

	// Filter markdown artifacts that don't sound good in TTS
	text = strings.ReplaceAll(text, "*", "")

	// 6. Synthesize audio using shared method
	audioPath, format, err := s.synthesizeAudio(ctx, text, "screenshot")
	if err != nil {
		s.handleTTSError(err)
		return
	}

	// 7. Create a Narrative for the screenshot
	screenshotTitle := fmt.Sprintf("Screenshot: %s", filepath.Base(imagePath))
	narrative := &Narrative{
		Type:           "screenshot",
		POI:            nil, // Screenshots don't have a POI
		Title:          screenshotTitle,
		Script:         text,
		AudioPath:      audioPath,
		Format:         format,
		RequestedWords: s.cfg.Narrator.NarrationLengthShortWords,
	}

	// 8. Stage the narrative for queue
	s.mu.Lock()
	s.stagedNarrative = narrative
	s.mu.Unlock()

	slog.Info("Narrator: Screenshot staged for playback", "title", screenshotTitle)

	// 9. If no narration is currently active, play immediately
	//    Otherwise, it will be picked up by the next cycle
	if !s.audio.IsBusy() && !s.IsActive() {
		if err := s.PlayNarrative(ctx, narrative); err != nil {
			slog.Error("Narrator: Failed to play screenshot narrative", "error", err)
		}
		// Clear staged since we played it
		s.mu.Lock()
		if s.stagedNarrative == narrative {
			s.stagedNarrative = nil
		}
		s.mu.Unlock()
	}
}
