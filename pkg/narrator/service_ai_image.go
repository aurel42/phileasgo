package narrator

import (
	"context"
	"log/slog"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/sim"
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

	// 4. Enqueue for Priority Generation
	s.enqueueGeneration(&generation.Job{
		Type:      model.NarrativeTypeScreenshot,
		ImagePath: imagePath,
		Manual:    true,
		CreatedAt: time.Now(),
		Telemetry: tel,
	})

	// 5. Kick the worker
	go s.ProcessGenerationQueue(context.Background())
}
