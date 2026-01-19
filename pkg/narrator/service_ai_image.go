package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"phileasgo/pkg/sim"
	"strings"
	"time"
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
		"City":        city,
		"Lat":         fmt.Sprintf("%.3f", tel.Latitude),
		"Lon":         fmt.Sprintf("%.3f", tel.Longitude),
		"Alt":         fmt.Sprintf("%.0f", tel.AltitudeAGL),
		"MaxWords":    s.cfg.Narrator.NarrationLengthLongWords,
		"TripSummary": s.getTripSummary(),
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
	screenshotTitle := "Reviewing Screenshot"
	// TODO: Use better title or timestamp?

	// Queue Constraints
	if !s.canEnqueue("screenshot", true) {
		slog.Info("Narrator: Screenshot skipped (queue constraints)")
		return
	}

	// 5. Create Narrative
	narrative := &Narrative{
		Type:           "screenshot", // Or "image"? Check service_ai.go checks "screenshot"
		POI:            nil,
		Title:          screenshotTitle,
		Script:         text,
		AudioPath:      audioPath,
		ImagePath:      imagePath,
		Format:         format,
		Duration:       time.Duration(len(strings.Split(text, " "))) * 300 * time.Millisecond,
		RequestedWords: s.cfg.Narrator.NarrationLengthShortWords,
		Manual:         true,
	}

	// 8. Enqueue (High Priority)
	s.enqueue(narrative, true)

	slog.Info("Narrator: Screenshot queued", "title", screenshotTitle)

	// 9. Trigger processing
	go s.processQueue(context.Background())
}
