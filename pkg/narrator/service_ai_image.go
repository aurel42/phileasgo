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
// It generates a description using Gemini (multimodal), synthesizes audio,
// and queues it for playback via the standard narrator pipeline.
// Screenshots do NOT interrupt already-playing narrations; they queue behind current playback.
// PlayImage handles the analysis and narration of a screenshot.
func (s *AIService) PlayImage(ctx context.Context, imagePath string, tel *sim.Telemetry) {
	if s.IsPaused() {
		slog.Info("Narrator: Skipping screenshot (paused)")
		return
	}

	s.enqueuePriority(&GenerationJob{
		Type:      model.NarrativeTypeScreenshot,
		ImagePath: imagePath,
		Manual:    true,
		CreatedAt: time.Now(),
		Telemetry: tel,
	})
	slog.Info("Narrator: Screenshot generation enqueued", "path", imagePath)
}

// GenerateScreenshotNarrative generates a narrative from a screenshot path.
// It handles LLM analysis and TTS synthesis.
func (s *AIService) GenerateScreenshotNarrative(ctx context.Context, imagePath string, tel *sim.Telemetry) (*Narrative, error) {
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
		return nil, fmt.Errorf("failed to render screenshot prompt: %w", err)
	}

	// 4. Call LLM (Multimodal)
	text, err := s.llm.GenerateImageText(ctx, "screenshot", prompt, imagePath)
	if err != nil {
		return nil, fmt.Errorf("Gemini analysis failed: %w", err)
	}

	if text == "" {
		return nil, fmt.Errorf("Gemini returned empty description")
	}
	slog.Debug("Narrator: Screenshot described", "text", text)

	// Filter markdown artifacts that don't sound good in TTS
	text = strings.ReplaceAll(text, "*", "")

	// 6. Synthesize audio using shared method
	audioPath, format, err := s.synthesizeAudio(ctx, text, "screenshot")
	if err != nil {
		s.handleTTSError(err)
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	// 7. Create a Narrative for the screenshot
	screenshotTitle := "Reviewing Screenshot"

	// Queue Constraints Check (still useful here?)
	// If called from ProcessPriorityQueue, we bypassed this.
	// If called from PlayImage (legacy/direct), we might want it?
	// PlayImage legacy had check before enqueue.
	// Here we just generate. Enqueuing is caller's job.

	narrative := &Narrative{
		Type:           "screenshot",
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

	return narrative, nil
}
