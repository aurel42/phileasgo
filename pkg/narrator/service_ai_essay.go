package narrator

import (
	"context"
	"log/slog"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// PlayEssay triggers a regional essay narration.
func (s *AIService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	if s.essayH == nil {
		return false
	}

	// Constraints
	if s.HasPendingGeneration() {
		return false
	}

	s.mu.Lock()
	if s.generating {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	slog.Info("Narrator: Triggering Essay")

	topic, err := s.essayH.SelectTopic()
	if err != nil {
		slog.Error("Narrator: Failed to select essay topic", "error", err)
		return false
	}

	go s.narrateEssay(context.Background(), topic, tel)
	return true
}

func (s *AIService) narrateEssay(ctx context.Context, topic *EssayTopic, tel *sim.Telemetry) {
	s.initAssembler()

	slog.Info("Narrator: Narrating Essay", "topic", topic.Name)

	if !s.claimGeneration(nil) {
		return
	}
	defer s.releaseGeneration()

	// Gather Context
	if tel == nil {
		t, _ := s.sim.GetTelemetry(ctx)
		tel = &t
	}

	loc := s.geoSvc.GetLocation(tel.Latitude, tel.Longitude)
	region := loc.CityName
	if loc.CityName != "Unknown" {
		region = "Near " + loc.CityName
	}

	pd := s.promptAssembler.ForGeneric(ctx, tel, s.getSessionState())
	pd["TargetCountry"] = loc.CountryCode
	pd["TargetRegion"] = region

	prompt, err := s.essayH.BuildPrompt(ctx, topic, &pd)
	if err != nil {
		slog.Error("Narrator: Failed to render essay prompt", "error", err)
		return
	}

	req := GenerationRequest{
		Type:          model.NarrativeTypeEssay,
		Prompt:        prompt,
		Title:         topic.Name,
		SafeID:        "essay_" + topic.ID,
		EssayTopic:    topic,
		MaxWords:      s.promptAssembler.ApplyWordLengthMultiplier(topic.MaxWords),
		Manual:        false,
		SkipBusyCheck: true,
		TwoPass:       s.cfg.AppConfig().Narrator.TwoPassScriptGeneration,
		PromptData:    pd,
	}

	narrative, err := s.GenerateNarrative(ctx, &req)
	if err != nil {
		slog.Error("Narrator: Essay generation failed", "error", err)
		return
	}

	s.enqueuePlayback(narrative, false)
}
