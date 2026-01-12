package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/sim"
)

// PlayPOI triggers narration for a specific POI.
func (s *AIService) PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry, strategy string) {
	if manual {
		slog.Info("Narrator: Manual play requested", "poi_id", poiID)
	} else {
		slog.Info("Narrator: Automated play triggering", "poi_id", poiID)
	}

	// 1. Generation Phase
	narrative, err := s.GenerateNarrative(ctx, poiID, strategy, tel)
	if err != nil {
		slog.Error("Narrator: Generation failed", "poi_id", poiID, "error", err)
		return
	}

	// 2. Playback Phase
	if err := s.PlayNarrative(ctx, narrative); err != nil {
		slog.Error("Narrator: Playback failed", "poi_id", poiID, "error", err)
	}
}

// GenerateNarrative prepares a narrative for a POI without playing it.
func (s *AIService) GenerateNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) (*Narrative, error) {
	// 1. Synchronous state check
	s.mu.Lock()
	if s.generating {
		s.mu.Unlock()
		return nil, fmt.Errorf("narrator already generating")
	}
	s.generating = true
	s.mu.Unlock()

	// Capture start time for latency tracking
	startTime := time.Now()

	defer func() {
		// Update Latency (Total Generation Cost)
		// We do this in defer to catch errors too, though ideally we only care about success or expensive failures?
		// Let's count it all for now as "Cost of utilizing the creation pipeline".
		// Actually, if it errors early, it's fast. That might skew avg down.
		// But if it takes 5s and fails, we want to know that cost.
		s.updateLatency(time.Since(startTime))

		s.mu.Lock()
		s.generating = false
		s.mu.Unlock()
	}()

	// Fetch POI (using background context for consistency with prev implementation, or passed ctx)
	p, err := s.poiMgr.GetPOI(ctx, poiID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch POI: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("POI not found: %s", poiID)
	}

	// Gather Context & Build Prompt
	promptData := s.buildPromptData(ctx, p, tel, strategy)
	slog.Info("Narrator: Generating script", "name", p.DisplayName(), "qid", p.WikidataID, "relative_dominance", promptData.DominanceStrategy)

	prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// Generate LLM Script
	script, err := s.generateScript(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Save to history (This was in narratePOI, ok to do here as it's part of "creation")
	p.Script = script
	s.addScriptToHistory(p.WikidataID, p.DisplayName(), script)

	// TTS Synthesis
	safeID := strings.ReplaceAll(p.WikidataID, "/", "_")
	audioPath, format, err := s.synthesizeAudio(ctx, script, safeID)
	if err != nil {
		s.handleTTSError(err)
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	// If synthesis successful, return Narrative
	return &Narrative{
		POI:            p,
		Script:         script,
		AudioPath:      audioPath,
		Format:         format,
		RequestedWords: promptData.MaxWords,
	}, nil
}

// PlayNarrative plays a previously generated narrative.
func (s *AIService) PlayNarrative(ctx context.Context, n *Narrative) error {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return fmt.Errorf("narrator already active")
	}
	s.active = true
	s.currentPOI = n.POI
	s.lastPOI = n.POI
	s.lastEssayTopic = nil
	s.lastEssayTitle = ""
	s.mu.Unlock()

	defer func() {
		// Wait a bit before clearing active to prevent rapid re-triggering?
		// Previous code had: time.Sleep(3 * time.Second) inside defer!
		// We should replicate that behavior or decision.
		go func() {
			time.Sleep(3 * time.Second)
			s.mu.Lock()
			s.active = false
			s.currentPOI = nil
			s.mu.Unlock()
		}()
	}()

	// Optional: Marker Spawning
	if s.beaconSvc != nil {
		_ = s.beaconSvc.SetTarget(ctx, n.POI.Lat, n.POI.Lon)
	}

	audioFile := n.AudioPath + "." + n.Format

	// Playback
	if err := s.audio.Play(audioFile, false); err != nil {
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		return fmt.Errorf("audio playback failed: %w", err)
	}

	// Update Latency (Setup time only) - MOVED TO GenerateNarrative
	// s.updateLatency(time.Since(startTime))

	// Mark as played
	n.POI.LastPlayed = time.Now()
	if err := s.st.SavePOI(ctx, n.POI); err != nil {
		slog.Error("Narrator: Failed to save narrated POI state", "qid", n.POI.WikidataID, "error", err)
	}

	// Log Stats
	genWords := len(strings.Fields(n.Script))
	duration := s.audio.Duration()
	slog.Info("Narrator: Narration stats",
		"name", n.POI.DisplayName(),
		"qid", n.POI.WikidataID,
		"requested_len", n.RequestedWords,
		"words", genWords,
		"audio_duration", duration,
	)

	// Update stats
	s.mu.Lock()
	s.narratedCount++
	s.mu.Unlock()

	// Block until playback finishes (keep active logic)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.audio.Stop()
			return ctx.Err()
		case <-ticker.C:
			if !s.audio.IsBusy() {
				return nil
			}
		}
	}
}
