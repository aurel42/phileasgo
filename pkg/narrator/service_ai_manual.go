package narrator

import (
	"context"
	"fmt"
	"log/slog"
)

// EnsurePOILoaded ensures the POI is hydrated and ready for narration.
func (s *AIService) EnsurePOILoaded(ctx context.Context, qid string, lat, lon float64) error {
	p, err := s.poiMgr.GetPOI(ctx, qid)
	if err == nil && p != nil {
		return nil
	}

	slog.Info("Narrator: Feature not found in manager, attempting hydration", "qid", qid)
	if err := s.enricher.EnsurePOIsLoaded(ctx, []string{qid}, lat, lon); err != nil {
		return fmt.Errorf("failed to hydrate feature %s: %w", qid, err)
	}

	// Double check
	p, err = s.poiMgr.GetPOI(ctx, qid)
	if err != nil || p == nil {
		return fmt.Errorf("feature %s still missing after successful hydration", qid)
	}

	return nil
}
