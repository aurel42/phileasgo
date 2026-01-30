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

	// 0. Pause Respect
	if s.audio != nil && s.audio.IsUserPaused() {
		return false
	}

	// 1. Constraints
	if s.HasPendingGeneration() {

		return false
	}
	if !s.canEnqueuePlayback("essay", false) {
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
		s.mu.Lock()
		s.generating = false
		s.mu.Unlock()
		return false
	}

	go s.narrateEssay(context.Background(), topic, tel)
	return true
}

func (s *AIService) narrateEssay(ctx context.Context, topic *EssayTopic, tel *sim.Telemetry) {
	s.mu.Lock()
	s.currentTopic = topic
	s.currentEssayTitle = "" // Reset title until generated
	s.lastPOI = nil          // Clear last POI since this is new
	s.lastEssayTopic = topic // Set for replay
	s.lastEssayTitle = ""    // Will update if generated
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.generating = false
		s.currentTopic = nil
		s.currentEssayTitle = ""
		s.mu.Unlock()
	}()

	if s.beaconSvc != nil {
		s.beaconSvc.Clear()
	}

	slog.Info("Narrator: Narrating Essay", "topic", topic.Name)

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

	pd := s.getCommonPromptData()
	s.injectTelemetry(pd, tel)
	s.injectUnits(pd)

	pd["TargetCountry"] = loc.CountryCode
	pd["TargetRegion"] = region
	pd["TTSInstructions"] = s.fetchTTSInstructions(pd)

	prompt, err := s.essayH.BuildPrompt(ctx, topic, &pd)
	if err != nil {
		slog.Error("Narrator: Failed to render essay prompt", "error", err)
		return
	}

	req := GenerationRequest{
		Type:   model.NarrativeTypeEssay,
		Prompt: prompt,
		// Title unknown until parsing
		SafeID:     "essay_" + topic.ID,
		EssayTopic: topic,
		MaxWords:   s.applyWordLengthMultiplier(topic.MaxWords),
		Manual:     false,
	}

	narrative, err := s.GenerateNarrative(ctx, &req)
	if err != nil {
		slog.Error("Narrator: Essay generation failed", "error", err)
		return
	}

	// Capture parsed title for UI state
	s.mu.Lock()
	s.currentEssayTitle = narrative.Title
	s.lastEssayTitle = narrative.Title
	s.mu.Unlock()

	// Enqueue (Automated, Low Priority)
	s.enqueuePlayback(narrative, false)
	go s.ProcessPlaybackQueue(context.Background())
}
