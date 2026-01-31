package narrator

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/sim"
)

// PlayBorder triggers a border crossing announcement.
func (s *AIService) PlayBorder(ctx context.Context, from, to string, tel *sim.Telemetry) bool {
	s.initAssembler()
	s.mu.RLock()
	// enabled := s.cfg.Narrator.Border.Enabled // TODO: Add to config if needed, default true for now
	enabled := true
	s.mu.RUnlock()

	if !enabled {
		return false
	}

	// Pause Respect
	if s.audio != nil && s.audio.IsUserPaused() {
		return false
	}

	// Queue Constraints
	if !s.canEnqueuePlayback("border", true) {
		slog.Info("Narrator: Border announcement skipped (queue constraints)")
		return false
	}

	// Sync Checks (Wait for priority jobs)
	if s.HasPendingGeneration() {
		slog.Info("Narrator: Border announcement skipped (priority jobs pending)")
		return false
	}

	slog.Info("Narrator: Border announcement requested", "from", from, "to", to)

	// Enqueue for Generation (High Priority)
	s.enqueueGeneration(&generation.Job{
		Type:      model.NarrativeTypeBorder,
		From:      from,
		To:        to,
		Manual:    true,
		CreatedAt: time.Now(),
		Telemetry: tel,
	})
	go s.ProcessGenerationQueue(context.Background())

	return true
}
