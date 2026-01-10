package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/tts"
)

// PlayPOI triggers narration for a specific POI.
func (s *AIService) PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry, strategy string) {
	if manual {
		slog.Info("Narrator: Manual play requested", "poi_id", poiID)
	} else {
		slog.Info("Narrator: Automated play triggering", "poi_id", poiID)
	}

	// 1. Synchronous state update to prevent races
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		slog.Warn("Narrator: PlayPOI rejected - already active", "poi_id", poiID, "manual", manual)
		return
	}
	s.active = true
	s.generating = true
	s.mu.Unlock()

	// Fetch POI from manager
	p, err := s.poiMgr.GetPOI(context.Background(), poiID)
	if err != nil {
		slog.Error("Narrator: Failed to fetch POI", "poi_id", poiID, "error", err)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.mu.Unlock()
		return
	}
	if p == nil {
		slog.Warn("Narrator: POI not found in manager", "poi_id", poiID)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.mu.Unlock()
		return
	}

	slog.Info("Narrator: Starting narration", "poi_id", poiID, "name", p.DisplayName())
	go s.narratePOI(context.Background(), p, tel, time.Now(), strategy)
}

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

func (s *AIService) ReplayLast(ctx context.Context) bool {
	// 1. Check Audio Replay Capability
	if !s.audio.ReplayLastNarration() {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 2. Restore State for UI
	switch {
	case s.lastPOI != nil:
		slog.Info("Narrator: Replaying last POI", "title", s.lastPOI.NameEn)
		s.currentPOI = s.lastPOI
		s.active = true // Mark active so UI shows "PLAYING"
	case s.lastEssayTopic != nil:
		slog.Info("Narrator: Replaying last Essay", "title", s.lastEssayTitle)
		s.currentTopic = s.lastEssayTopic
		s.currentEssayTitle = s.lastEssayTitle
		s.active = true
	default:
		// Audio replayed but we have no state?
		slog.Warn("Narrator: Replaying audio but no state to restore")
		return true
	}

	// 3. Launch Monitor to clear state when done
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !s.audio.IsBusy() {
					s.mu.Lock()
					s.active = false
					s.currentPOI = nil
					s.currentTopic = nil
					s.currentEssayTitle = ""
					s.mu.Unlock()
					return
				}
			}
		}
	}()

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
		ScriptHistory:    s.getScriptHistory(),
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
		if strings.Contains(err.Error(), "gemini client not configured") {
			slog.Error("FATAL: Gemini client not configured. Application cannot proceed.", "error", err)
			os.Exit(1) //nolint:gocritic
		}
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

// SkipCooldown forces the cooldown to expire (not strictly needed by AIService itself, but by the job).
func (s *AIService) SkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = true
	slog.Info("Narrator: Skip cooldown requested")
}

// ShouldSkipCooldown returns true if the cooldown should be skipped.
func (s *AIService) ShouldSkipCooldown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.skipCooldown
}

// ResetSkipCooldown resets the skip cooldown flag.
func (s *AIService) ResetSkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = false
}

func (s *AIService) narratePOI(ctx context.Context, p *model.POI, tel *sim.Telemetry, startTime time.Time, strategy string) {
	// active is already set true by PlayPOI
	s.mu.Lock()
	s.currentPOI = p
	s.lastPOI = p          // Capture for replay
	s.lastEssayTopic = nil // Clear essay since this is new
	s.lastEssayTitle = ""
	s.mu.Unlock()

	defer func() {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.currentPOI = nil
		s.mu.Unlock()
	}()

	// NOTE: LastPlayed is now set AFTER successful TTS (not here) to avoid "consuming" POI on failure

	// 1. Gather Context & Build Prompt
	promptData := s.buildPromptData(ctx, p, tel, strategy)

	slog.Info("Narrator: Narrating POI", "name", p.DisplayName(), "qid", p.WikidataID, "relative_dominance", promptData.DominanceStrategy)

	prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
	if err != nil {
		slog.Error("Narrator: Failed to render prompt", "error", err)
		return
	}

	// 2. Optional: Marker Spawning (Before LLM to give immediate visual feedback)
	if s.beaconSvc != nil {
		// Spawn target beacon at POI. Altitude 0 (ground) for now.
		_ = s.beaconSvc.SetTarget(ctx, p.Lat, p.Lon)
	}

	// 3. Generate LLM Script
	script, err := s.generateScript(ctx, prompt)
	if err != nil {
		if strings.Contains(err.Error(), "gemini client not configured") {
			slog.Error("FATAL: Gemini client not configured. Application cannot proceed.", "error", err)
			os.Exit(1) //nolint:gocritic
		}
		slog.Error("Narrator: LLM script generation failed", "error", err)
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		return
	}

	// Save to history
	p.Script = script
	s.addScriptToHistory(p.WikidataID, p.DisplayName(), script)

	// 4. TTS Synthesis
	// Sanitize filename
	safeID := strings.ReplaceAll(p.WikidataID, "/", "_")

	audioPath, format, err := s.synthesizeAudio(ctx, script, safeID)
	if err != nil {
		s.handleTTSError(err)
		return
	}

	// Mark as played NOW (after successful TTS) to prevent re-selection
	p.LastPlayed = time.Now()
	if err := s.st.SavePOI(ctx, p); err != nil {
		slog.Error("Narrator: Failed to save narrated POI state", "qid", p.WikidataID, "error", err)
	}

	audioFile := audioPath + "." + format

	// 5. Update Latency before Playback
	latency := time.Since(startTime)
	s.updateLatency(latency)

	// 6. Playback
	if err := s.audio.Play(audioFile, false); err != nil {
		slog.Error("Narrator: Playback failed", "path", audioFile, "error", err)
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		return
	}

	s.mu.Lock()
	s.generating = false
	s.mu.Unlock()

	// Log Stats
	genWords := len(strings.Fields(script))
	duration := s.audio.Duration()
	slog.Info("Narrator: Narration stats",
		"name", p.DisplayName(),
		"qid", p.WikidataID,
		"max_words_requested", promptData.MaxWords,
		"words_received", genWords,
		"audio_duration", duration,
	)

	// Block until playback finishes so state remains active
	// We check every 100ms
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	func() {
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
	}()

	s.mu.Lock()
	s.narratedCount++
	s.mu.Unlock()
}

func (s *AIService) generateScript(ctx context.Context, prompt string) (string, error) {
	return s.llm.GenerateText(ctx, "narration", prompt)
}

func (s *AIService) synthesizeAudio(ctx context.Context, script, safeID string) (audioPath, format string, err error) {
	// Use system temp directory instead of persistent cache
	cacheDir := os.TempDir()

	// Use unique name to avoid conflicts and persistence
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("phileas_narration_%s_%d", safeID, time.Now().UnixNano()))

	ttsProvider := s.getTTSProvider()
	format, err = ttsProvider.Synthesize(ctx, script, "", outputPath)
	if err != nil {
		return "", "", err
	}
	return outputPath, format, nil
}

func (s *AIService) handleTTSError(err error) {
	slog.Error("Narrator: TTS synthesis failed", "error", err)

	// Check if this is a fatal error that should trigger fallback
	if tts.IsFatalError(err) && !s.isUsingFallbackTTS() {
		s.activateFallback()
		slog.Warn("Narrator: Skipping current POI (script incompatible with fallback TTS). Will retry with next POI.")
	}

	if s.beaconSvc != nil {
		s.beaconSvc.Clear()
	}
	// Do NOT set LastPlayed - POI can be retried
}
