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
// PlayPOI triggers narration for a specific POI.
func (s *AIService) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
	if manual {
		slog.Info("Narrator: Manual play requested", "poi_id", poiID, "enqueue_if_busy", enqueueIfBusy)
	} else {
		slog.Info("Narrator: Automated play triggering", "poi_id", poiID)
	}

	// 0. Queue Check & Override Logic
	if s.handleManualQueueAndOverride(poiID, strategy, manual, enqueueIfBusy) {
		return
	}

	// Immediate Visual Update (Marker Preview)
	p, err := s.poiMgr.GetPOI(ctx, poiID)
	if err == nil && p != nil && s.beaconSvc != nil {
		_ = s.beaconSvc.SetTarget(context.Background(), p.Lat, p.Lon)
	}

	// 1. Check Queue (Deduplication & Re-prioritization)
	s.mu.Lock()
	for i, n := range s.queue {
		if n.POI != nil && n.POI.WikidataID == poiID {
			if manual {
				// Move to front (High Priority)
				s.queue = append(s.queue[:i], s.queue[i+1:]...)
				s.queue = append([]*Narrative{n}, s.queue...)
				slog.Info("Narrator: Boosting queued item to front", "poi_id", poiID)
			} else {
				slog.Info("Narrator: Item already in queue, skipping duplicate request", "poi_id", poiID)
			}
			s.mu.Unlock()
			go s.processQueue(context.Background())
			return
		}
	}
	s.mu.Unlock()

	// 2. Constraints Check
	if !manual && s.IsGenerating() {
		slog.Info("Narrator: Skipping auto-play (busy generating)")
		return
	}

	if !s.canEnqueue("poi", manual) {
		slog.Info("Narrator: Play request rejected by queue constraints", "poi_id", poiID, "manual", manual)
		return
	}

	// 2. Generation Phase
	// Manual force=true will cancel any existing generation
	narrative, err := s.GenerateNarrative(ctx, poiID, strategy, tel, manual)
	if err != nil {
		slog.Error("Narrator: Generation failed", "poi_id", poiID, "error", err)
		return
	}

	// 3. Enqueue & Trigger
	s.enqueue(narrative, manual)

	// Trigger processing in background (if idle)
	go s.processQueue(context.Background())
}

// PrepareNextNarrative prepares a narrative for a POI and stages it for later playback.
func (s *AIService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	slog.Info("Narrator: Pipeline preparing next narrative", "poi_id", poiID)

	narrative, err := s.GenerateNarrative(ctx, poiID, strategy, tel, false)
	if err != nil {
		return err
	}

	s.enqueue(narrative, false)
	return nil
}

// GenerateNarrative prepares a narrative for a POI without playing it.
func (s *AIService) GenerateNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry, force bool) (*Narrative, error) {
	// 1. Synchronous state check & Force Logic
	s.mu.Lock()
	if s.generating {
		if force {
			slog.Info("Narrator: Forcing generation, canceling previous job")
			if s.genCancelFunc != nil {
				s.genCancelFunc() // Cancel the existing context
			}
			s.mu.Unlock()

			// Wait for the previous job to exit (it clears s.generating in defer)
			// Busy wait with timeout
			waitStart := time.Now()
			for {
				s.mu.RLock()
				stillGen := s.generating
				s.mu.RUnlock()
				if !stillGen {
					break
				}
				if time.Since(waitStart) > 5*time.Second {
					return nil, fmt.Errorf("timeout waiting for previous generation to cancel")
				}
				time.Sleep(100 * time.Millisecond)
			}
			s.mu.Lock() // Re-acquire for setting true below
		} else {
			s.mu.Unlock()
			return nil, fmt.Errorf("narrator already generating")
		}
	}
	s.generating = true

	// Create a cancellable context for THIS generation
	genCtx, cancel := context.WithCancel(ctx)
	s.genCancelFunc = cancel

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
		s.genCancelFunc = nil // Clear cancel func
		s.generatingPOI = nil
		s.mu.Unlock()
		cancel() // Ensure context is cancelled on exit
	}()

	// Use genCtx instead of ctx for downstream calls
	ctx = genCtx

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
	limit := promptData.MaxWords + 100
	if wordCount > limit {
		slog.Warn("Narrator: Script exceeded limit, attempting rescue",
			"requested", promptData.MaxWords,
			"actual", wordCount,
			"limit", limit)

		// Attempt LLM-based rescue
		rescuedScript, err := s.rescueScript(ctx, script, promptData.MaxWords)
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

	// Log generation complete (before queue wait for playback)
	genWords := len(strings.Fields(script))
	slog.Debug("Narrator: Generation complete",
		"name", p.DisplayName(),
		"qid", p.WikidataID,
		"requested_len", promptData.MaxWords,
		"words", genWords,
	)

	// If synthesis successful, return Narrative
	return &Narrative{
		Type:           "poi",
		POI:            p,
		Title:          p.DisplayName(),
		Script:         script,
		AudioPath:      audioPath,
		Format:         format,
		RequestedWords: promptData.MaxWords,
	}, nil
}

// PlayNarrative plays a previously generated narrative.
// This method is non-blocking: it starts playback and returns immediately.
// Playback completion is monitored in a background goroutine.
// Supports both POI narratives and non-POI narratives (screenshots, essays, etc.)
func (s *AIService) PlayNarrative(ctx context.Context, n *Narrative) error {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return fmt.Errorf("narrator already active")
	}
	s.active = true
	s.currentPOI = n.POI // May be nil for non-POI narratives
	s.currentImagePath = n.ImagePath
	if n.POI != nil {
		s.lastPOI = n.POI
	}
	if n.ImagePath != "" {
		s.lastImagePath = n.ImagePath
	}
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
		s.currentImagePath = ""
		s.mu.Unlock()
		return fmt.Errorf("audio playback failed: %w", err)
	}

	// POI-specific operations: mark as played and save
	if n.POI != nil {
		n.POI.LastPlayed = time.Now()
		if err := s.st.SavePOI(ctx, n.POI); err != nil {
			slog.Error("Narrator: Failed to save narrated POI state", "qid", n.POI.WikidataID, "error", err)
		}
	} else if s.beaconSvc != nil {
		// Clear beacon for non-POI narratives (e.g. screenshot) to avoid confusing 3D markers
		s.beaconSvc.Clear()
	}

	// Determine display name for logging
	displayName := n.Title
	displayID := n.Type
	if n.POI != nil {
		displayName = n.POI.DisplayName()
		displayID = n.POI.WikidataID
	}

	// Log Stats
	genWords := len(strings.Fields(n.Script))
	duration := s.audio.Duration()
	slog.Info("Narrator: Narration stats",
		"type", n.Type,
		"name", displayName,
		"id", displayID,
		"requested_len", n.RequestedWords,
		"words", genWords,
		"audio_duration", duration,
	)

	// Update History (Trip Summary) - only for POIs and screenshots
	if n.POI != nil {
		s.addScriptToHistory(n.POI.WikidataID, n.POI.DisplayName(), n.Script)
	} else if n.Type == "screenshot" {
		s.addScriptToHistory("screenshot", n.Title, n.Script)
	}

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
		if s.audio.IsBusy() {
			continue
		}

		// Use Title for non-POI narratives, DisplayName() for POI narratives
		displayName := n.Title
		displayID := n.Type
		if n.POI != nil {
			displayName = n.POI.DisplayName()
			displayID = n.POI.WikidataID
		}
		slog.Info("Narrator: Playback ended", "name", displayName, "id", displayID)
		break
	}

	// Update Beacon Target immediately after playback
	// Only relevant for POI narratives (non-POI narratives don't have map markers)
	if s.beaconSvc != nil {
		// Don't lock just to peek, peekQueue handles RLock
		next := s.peekQueue()
		s.mu.RLock()
		generating := s.generatingPOI
		s.mu.RUnlock()

		if next != nil && next.POI != nil {
			slog.Info("Narrator: Switching marker to next queued POI", "qid", next.POI.WikidataID)
			// Use background context as the original ctx might be cancelled
			_ = s.beaconSvc.SetTarget(context.Background(), next.POI.Lat, next.POI.Lon)
		} else if generating != nil {
			// BEACON LAG FIX:
			// If we are actively generating the next one, show its marker usage NOW.
			slog.Info("Narrator: Switching marker to currently generating POI", "qid", generating.WikidataID)
			_ = s.beaconSvc.SetTarget(context.Background(), generating.Lat, generating.Lon)
		}
		// Else: Do nothing. Keep marker on the last played POI.
	}

	// Wait before clearing active (prevent rapid re-trigger)
	time.Sleep(3 * time.Second)
	s.mu.Lock()
	s.active = false
	s.currentPOI = nil
	s.currentImagePath = ""
	s.mu.Unlock()

	// Trigger next item in queue
	go s.processQueue(context.Background())
}

// processQueue attempts to play the next item in the queue.
func (s *AIService) processQueue(ctx context.Context) {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return // Already playing
	}
	// Check queue
	if len(s.queue) == 0 {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// Pop (using helper)
	next := s.popQueue()
	if next == nil {
		return
	}

	if err := s.PlayNarrative(ctx, next); err != nil {
		slog.Error("Narrator: Queue playback failed, trying next", "error", err)
		// Try next immediately? Or assume PlayNarrative cleanup triggers monitor?
		// PlayNarrative returns error implies it didn't start.
		// So we should try next recursion.
		go s.processQueue(ctx)
	}
}

// rescueScript attempts to extract a clean script from contaminated LLM output.
// It uses a secondary LLM call to identify and remove chain-of-thought reasoning.
func (s *AIService) rescueScript(ctx context.Context, script string, maxWords int) (string, error) {
	prompt, err := s.prompts.Render("context/rescue_script.tmpl", map[string]any{
		"Script":   script,
		"MaxWords": maxWords,
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
