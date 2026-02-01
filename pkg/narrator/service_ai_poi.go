package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/generation"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// PlayPOI starts the generation process for a POI.
func (s *AIService) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
	s.initAssembler()

	if tel == nil && s.sim != nil {
		if t, err := s.sim.GetTelemetry(ctx); err == nil {
			tel = &t
		} else {
			slog.Warn("Narrator: Failed to fetch telemetry for PlayPOI", "error", err)
		}
	}

	if manual {
		slog.Info("Narrator: Manual generation requested", "poi_id", poiID)
		s.playPOIManual(poiID, strategy, tel)
	} else {
		p, err := s.poiMgr.GetPOI(ctx, poiID)
		if err != nil {
			slog.Error("Narrator: Failed to get POI for automated narration", "poi_id", poiID, "error", err)
			return
		}
		s.playPOIAutomated(ctx, p, tel, strategy)
	}
}

func (s *AIService) playPOIManual(poiID, strategy string, tel *sim.Telemetry) {
	s.enqueueGeneration(&generation.Job{
		Type:      model.NarrativeTypePOI,
		POIID:     poiID,
		Manual:    true,
		Strategy:  strategy,
		CreatedAt: time.Now(),
		Telemetry: tel,
	})
	go s.ProcessGenerationQueue(context.Background())
}

func (s *AIService) playPOIAutomated(ctx context.Context, p *model.POI, tel *sim.Telemetry, strategy string) {
	// Synchronously claim the generation slot
	if !s.claimGeneration(p) {
		return
	}

	// 4. Async Generation (Auto)
	go func() {
		genCtx := context.Background()
		done := false
		defer func() {
			if !done {
				s.releaseGeneration()
			}
		}()

		promptData := s.promptAssembler.ForPOI(genCtx, p, tel, strategy, s.getSessionState())
		prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
		if err != nil {
			slog.Error("Narrator: Failed to render prompt", "error", err)
			return
		}

		req := GenerationRequest{
			Type:          model.NarrativeTypePOI,
			Prompt:        prompt,
			Title:         p.DisplayName(),
			SafeID:        strings.ReplaceAll(p.WikidataID, "/", "_"),
			POI:           p,
			MaxWords:      promptData["MaxWords"].(int),
			Manual:        false,
			SkipBusyCheck: true,
		}

		done = true
		narrative, err := s.GenerateNarrative(genCtx, &req)
		if err != nil {
			slog.Error("Narrator: Generation failed", "poi_id", p.WikidataID, "error", err)
			return
		}

		s.enqueuePlayback(narrative, false)
	}()
}

// PrepareNextNarrative prepares a narrative for a POI and stages it for later playback.
func (s *AIService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	p, err := s.poiMgr.GetPOI(ctx, poiID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("POI not found")
	}

	pd := s.promptAssembler.ForPOI(ctx, p, tel, strategy, s.getSessionState())
	prompt, err := s.prompts.Render("narrator/script.tmpl", pd)
	if err != nil {
		return err
	}

	req := GenerationRequest{
		Type:          model.NarrativeTypePOI,
		Prompt:        prompt,
		Title:         p.DisplayName(),
		SafeID:        strings.ReplaceAll(p.WikidataID, "/", "_"),
		POI:           p,
		MaxWords:      pd["MaxWords"].(int),
		Manual:        false,
		SkipBusyCheck: true,
	}

	narrative, err := s.GenerateNarrative(ctx, &req)
	if err != nil {
		return err
	}

	s.enqueuePlayback(narrative, false)
	return nil
}
