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

	// Immediate Visual Update (Marker Preview)
	// We do this before generation so the user sees the target while the LLM thinks.
	// Note: We need the coordinates. If we don't have the POI object yet, we might need to fetch it.
	// But PlayPOI is usually called with an ID. We need to fetch it to get coords.
	// Optimization: If we miss cache, it might delay slightly, but fetching context is fast.
	p, err := s.poiMgr.GetPOI(ctx, poiID)
	if err == nil && p != nil && s.beaconSvc != nil {
		_ = s.beaconSvc.SetTarget(context.Background(), p.Lat, p.Lon)
	}

	var narrative *Narrative

	// 0. Check Staging (Pipeline Optimization)
	s.mu.Lock()
	if s.stagedNarrative != nil {
		// Use staged narrative unconditionally if available
		slog.Info("Narrator: Using staged narrative (Zero Latency)", "poi_id", s.stagedNarrative.POI.WikidataID)

		if s.stagedNarrative.POI.WikidataID != poiID {
			slog.Warn("Narrator: Priority Override - Playing prepared POI instead of requested",
				"staged", s.stagedNarrative.POI.WikidataID,
				"requested", poiID)
		}

		narrative = s.stagedNarrative
		s.stagedNarrative = nil

	}
	s.mu.Unlock()

	// 1. Generation Phase (if not staged)
	if narrative == nil {
		narrative, err = s.GenerateNarrative(ctx, poiID, strategy, tel)
		if err != nil {
			slog.Error("Narrator: Generation failed", "poi_id", poiID, "error", err)
			return
		}
	}

	// 2. Playback Phase
	if err := s.PlayNarrative(ctx, narrative); err != nil {
		slog.Error("Narrator: Playback failed", "poi_id", poiID, "error", err)
	}
}

// PrepareNextNarrative prepares a narrative for a POI and stages it for later playback.
func (s *AIService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	slog.Info("Narrator: Pipeline preparing next narrative", "poi_id", poiID)

	narrative, err := s.GenerateNarrative(ctx, poiID, strategy, tel)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.stagedNarrative = narrative
	s.mu.Unlock()
	return nil
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
		s.generatingPOI = nil
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

	// Update state to show we are generating THIS POI
	s.mu.Lock()
	s.generatingPOI = p
	s.mu.Unlock()

	// Gather Context & Build Prompt
	promptData := s.buildPromptData(ctx, p, tel, strategy)
	slog.Info("Narrator: Generating script",
		"name", p.DisplayName(),
		"qid", p.WikidataID,
		"relative_dominance", promptData.DominanceStrategy,
		"predicted_delay", s.AverageLatency(),
	)

	prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// Generate LLM Script
	script, err := s.generateScript(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Plausibility Check: Script too long may indicate reasoning leak
	wordCount := len(strings.Fields(script))
	limit := promptData.MaxWords + 50
	if wordCount > limit {
		slog.Warn("Narrator: Script exceeded limit, attempting rescue",
			"requested", promptData.MaxWords,
			"actual", wordCount,
			"limit", limit)

		// Attempt LLM-based rescue
		rescuedScript, err := s.rescueScript(ctx, script)
		if err != nil {
			slog.Error("Narrator: Script rescue failed", "error", err)
			return nil, fmt.Errorf("script rescue failed: %w", err)
		}

		slog.Info("Narrator: Script rescue successful",
			"original_words", wordCount,
			"rescued_words", len(strings.Fields(rescuedScript)))
		script = rescuedScript
	}

	// Save to history (This was in narratePOI, ok to do here as it's part of "creation")
	p.Script = script
	// REMOVED: s.addScriptToHistory(p.WikidataID, p.DisplayName(), script) - Moved to PlayNarrative

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
// This method is non-blocking: it starts playback and returns immediately.
// Playback completion is monitored in a background goroutine.
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

	audioFile := n.AudioPath + "." + n.Format

	// Start playback (synchronous to catch immediate errors)
	if err := s.audio.Play(audioFile, false); err != nil {
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		// Reset state on error
		s.mu.Lock()
		s.active = false
		s.currentPOI = nil
		s.mu.Unlock()
		return fmt.Errorf("audio playback failed: %w", err)
	}

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

	// Update History (Trip Summary)
	s.addScriptToHistory(n.POI.WikidataID, n.POI.DisplayName(), n.Script)

	// Update stats
	s.mu.Lock()
	s.narratedCount++
	s.mu.Unlock()

	// Non-blocking: Monitor playback completion in background
	go s.monitorPlayback(n)

	return nil
}

// monitorPlayback waits for audio to finish and cleans up state.
func (s *AIService) monitorPlayback(n *Narrative) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if !s.audio.IsBusy() {
			slog.Info("Narrator: Playback ended", "name", n.POI.DisplayName(), "qid", n.POI.WikidataID)
			break
		}
	}

	// Update Beacon Target immediately after playback
	if s.beaconSvc != nil {
		s.mu.RLock()
		next := s.stagedNarrative
		s.mu.RUnlock()

		if next != nil {
			slog.Info("Narrator: Switching marker to next staged POI", "qid", next.POI.WikidataID)
			// Use background context as the original ctx might be cancelled
			_ = s.beaconSvc.SetTarget(context.Background(), next.POI.Lat, next.POI.Lon)
		}
		// Else: Do nothing. Keep marker on the last played POI.
	}

	// Wait before clearing active (prevent rapid re-trigger)
	time.Sleep(3 * time.Second)
	s.mu.Lock()
	s.active = false
	s.currentPOI = nil
	s.mu.Unlock()
}

// rescueScript attempts to extract a clean script from contaminated LLM output.
// It uses a secondary LLM call to identify and remove chain-of-thought reasoning.
func (s *AIService) rescueScript(ctx context.Context, script string) (string, error) {
	prompt, err := s.prompts.Render("context/rescue_script.tmpl", map[string]any{
		"Script": script,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render rescue prompt: %w", err)
	}

	// Use script_rescue profile (cheap model)
	rescued, err := s.llm.GenerateText(ctx, "script_rescue", prompt)
	if err != nil {
		return "", fmt.Errorf("rescue LLM call failed: %w", err)
	}

	// Check for explicit failure signal
	if strings.TrimSpace(rescued) == "RESCUE_FAILED" {
		return "", fmt.Errorf("LLM could not extract valid script")
	}

	return strings.TrimSpace(rescued), nil
}
