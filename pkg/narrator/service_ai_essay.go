package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"phileasgo/pkg/sim"
)

// PlayEssay triggers a regional essay narration.
func (s *AIService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	if s.essayH == nil {
		return false
	}

	// 1. Synchronous state update to prevent races
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return false
	}
	s.active = true
	s.generating = true
	s.mu.Unlock()

	slog.Info("Narrator: Triggering Essay")

	topic, err := s.essayH.SelectTopic()
	if err != nil {
		slog.Error("Narrator: Failed to select essay topic", "error", err)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.mu.Unlock()
		return false
	}

	go s.narrateEssay(context.Background(), topic, tel)
	return true
}

func (s *AIService) narrateEssay(ctx context.Context, topic *EssayTopic, tel *sim.Telemetry) {
	// active is already set true by PlayEssay
	s.mu.Lock()
	s.currentTopic = topic
	s.currentEssayTitle = "" // Reset title until generated
	s.lastPOI = nil          // Clear last POI since this is new
	s.lastEssayTopic = topic // Set for replay
	s.lastEssayTitle = ""    // Will update if generated
	s.mu.Unlock()

	defer func() {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		s.active = false
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

	// Save to history
	s.addScriptToHistory("", topic.Name, script)

	// Parse Title if present (Format: "TITLE: ...")
	lines := strings.Split(script, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "TITLE:") {
			title := strings.TrimSpace(strings.TrimPrefix(firstLine, "TITLE:"))
			s.mu.Lock()
			s.currentEssayTitle = title
			s.lastEssayTitle = title // Capture for replay
			s.mu.Unlock()

			// Remove title line from script for TTS
			script = strings.Join(lines[1:], "\n")
		}
	}

	// Synthesis
	cacheDir := os.TempDir()
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("phileas_essay_%s_%d", topic.ID, time.Now().UnixNano()))
	format, err := s.tts.Synthesize(ctx, script, "", outputPath)
	if err != nil {
		slog.Error("Narrator: TTS essay synthesis failed", "error", err)
		return
	}

	audioFile := outputPath + "." + format

	// Playback
	if err := s.audio.Play(audioFile, false); err != nil {
		slog.Error("Narrator: Playback failed", "path", audioFile, "error", err)
		return
	}

	s.mu.Lock()
	s.generating = false
	s.mu.Unlock()

	// Wait for finish
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.audio.Stop()
			return
		case <-ticker.C:
			if !s.audio.IsBusy() {
				return
			}
		}
	}
}
