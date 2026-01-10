package narrator

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
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
