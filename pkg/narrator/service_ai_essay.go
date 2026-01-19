package narrator

import (
	"context"
	"log/slog"
	"strings"

	"phileasgo/pkg/sim"
)

// PlayEssay triggers a regional essay narration.
func (s *AIService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	if s.essayH == nil {
		return false
	}

	// 1. Constraints
	if !s.canEnqueue("essay", false) {
		return false
	}

	s.mu.Lock()
	if s.generating {
		s.mu.Unlock()
		return false
	}
	s.generating = true
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

	pd := NarrationPromptData{
		TourGuideName:    "Ava", // TODO: Config
		FemalePersona:    "Intelligent, fascinating",
		FemaleAccent:     "Neutral",
		TargetLanguage:   s.cfg.Narrator.TargetLanguage,
		TargetCountry:    loc.CountryCode,
		TargetRegion:     region,
		Lat:              tel.Latitude,
		Lon:              tel.Longitude,
		UnitsInstruction: s.fetchUnitsInstruction(),
		TripSummary:      s.getTripSummary(),
	}
	pd.TTSInstructions = s.fetchTTSInstructions(&pd)

	prompt, err := s.essayH.BuildPrompt(ctx, topic, &pd)
	if err != nil {
		slog.Error("Narrator: Failed to render essay prompt", "error", err)
		return
	}

	// Generate Script
	script, err := s.llm.GenerateText(ctx, "essay", prompt)
	if err != nil {
		slog.Error("Narrator: LLM essay script generation failed", "error", err)
		return
	}
	// Filter markdown artifacts
	script = strings.ReplaceAll(script, "*", "")

	// Parse Title if present (Format: "TITLE: ...")
	essayTitle := topic.Name
	lines := strings.Split(script, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "TITLE:") {
			essayTitle = strings.TrimSpace(strings.TrimPrefix(firstLine, "TITLE:"))
			s.mu.Lock()
			s.currentEssayTitle = essayTitle
			s.lastEssayTitle = essayTitle // Capture for replay
			s.mu.Unlock()

			// Remove title line from script for TTS
			script = strings.Join(lines[1:], "\n")
		}
	}

	// Synthesize audio using shared method
	audioPath, format, err := s.synthesizeAudio(ctx, script, "essay_"+topic.ID)
	if err != nil {
		s.handleTTSError(err)
		slog.Error("Narrator: TTS essay synthesis failed", "error", err)
		return
	}

	// Create Narrative for the essay
	narrative := &Narrative{
		Type:           "essay",
		POI:            nil, // Essays don't have a POI
		Title:          essayTitle,
		Script:         script,
		AudioPath:      audioPath,
		Format:         format,
		RequestedWords: s.cfg.Narrator.NarrationLengthLongWords,
	}

	// Enqueue (Automated, Low Priority)
	s.enqueue(narrative, false)

	go s.processQueue(context.Background())
}
