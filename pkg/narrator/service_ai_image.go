package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"phileasgo/pkg/sim"
)

// PlayImage handles the analysis and narration of a screenshot.
// It generates a description using Gemini (multimodal) and plays it via TTS.
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
		"MaxWords": s.cfg.Narrator.NarrationLengthShortWords,
	}

	// We use a predefined template name "narrator/screenshot"
	// Assuming promptMgr is available as s.prompts
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
	slog.Info("Narrator: Screenshot described", "text", text)

	// 5. TTS & Playback
	// Check if we are still valid/active? (Maybe user paused in between?)
	if s.IsPaused() {
		return
	}

	// Synthesize
	audioFile, err := s.tts.Synthesize(ctx, text, "system_screenshot", "en-US") // Using default voice or system voice if available
	if err != nil {
		slog.Error("Narrator: TTS failed for screenshot", "error", err)
		return
	}

	// Play
	// We play this as a "System" type or similar, effectively blocking other POIs while playing.
	// Since we are in 'generating' lock, others shouldn't start.
	// But PlayAndWait returns immediately? No, currently Play is async usually.

	// If we use s.audio.Play, it sets IsBusy.
	if err := s.audio.Play(audioFile, false); err != nil {
		slog.Error("Narrator: Audio playback failed", "error", err)
		return
	}

	// Update stats/history if needed?
	// s.recordNarration(...)
}
