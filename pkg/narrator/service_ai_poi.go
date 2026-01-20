package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// PlayPOI triggers narration for a specific POI.
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
	if s.promoteInQueue(poiID, manual) {
		go s.processQueue(context.Background())
		return
	}

	// 2. Priority Queue Enqueue (Manual Only)
	if manual {
		s.enqueuePriority(&GenerationJob{
			Type:      model.NarrativeTypePOI,
			POIID:     poiID,
			Manual:    true,
			Strategy:  strategy,
			CreatedAt: time.Now(),
			Telemetry: tel,
		})
		return
	}

	// 3. Auto-Play Constraints (Drains Priority First)
	if s.HasPendingPriority() {
		slog.Info("Narrator: Skipping auto-play (priority queue not empty)")
		return
	}
	if s.IsGenerating() {
		slog.Info("Narrator: Skipping auto-play (busy generating)")
		return
	}

	if !s.canEnqueue("poi", manual) {
		slog.Info("Narrator: Play request rejected by queue constraints", "poi_id", poiID, "manual", manual)
		return
	}

	// 4. Async Generation (Auto)
	go func() {
		// Ensure generation context survives
		genCtx := context.Background()

		// Build Prompt
		promptData := s.buildPromptData(genCtx, p, tel, strategy)
		prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
		if err != nil {
			slog.Error("Narrator: Failed to render prompt", "error", err)
			return
		}

		// Create Request
		req := GenerationRequest{
			Type:     model.NarrativeTypePOI,
			Prompt:   prompt,
			Title:    p.DisplayName(),
			SafeID:   strings.ReplaceAll(p.WikidataID, "/", "_"),
			POI:      p,
			MaxWords: promptData.MaxWords,
			Manual:   manual,
		}

		narrative, err := s.GenerateNarrative(genCtx, &req)
		if err != nil {
			slog.Error("Narrator: Generation failed", "poi_id", poiID, "error", err)
			return
		}

		// Enqueue & Trigger
		s.enqueue(narrative, manual)
		s.processQueue(genCtx)
	}()
}

// PrepareNextNarrative prepares a narrative for a POI and stages it for later playback.
func (s *AIService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	slog.Info("Narrator: Pipeline preparing next narrative", "poi_id", poiID)

	p, err := s.poiMgr.GetPOI(ctx, poiID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("POI not found")
	}

	promptData := s.buildPromptData(ctx, p, tel, strategy)
	prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
	if err != nil {
		return err
	}

	req := GenerationRequest{
		Type:     model.NarrativeTypePOI,
		Prompt:   prompt,
		Title:    p.DisplayName(),
		SafeID:   strings.ReplaceAll(p.WikidataID, "/", "_"),
		POI:      p,
		MaxWords: promptData.MaxWords,
		Manual:   false,
	}

	narrative, err := s.GenerateNarrative(ctx, &req)
	if err != nil {
		return err
	}

	s.enqueue(narrative, false)
	return nil
}

// PlayNarrative plays a previously generated narrative.
// This method is non-blocking: it starts playback and returns immediately.
// Playback completion is monitored in a background goroutine.
// Supports both POI narratives and non-POI narratives (screenshots, essays, etc.)
func (s *AIService) PlayNarrative(ctx context.Context, n *model.Narrative) error {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return fmt.Errorf("narrator already active")
	}
	s.active = true
	s.currentPOI = n.POI // May be nil for non-POI narratives
	s.currentImagePath = n.ImagePath
	s.currentEssayTitle = ""
	if n.Type == "essay" || n.Type == "debrief" {
		s.currentEssayTitle = n.Title
	}
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
	displayID := string(n.Type)
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
func (s *AIService) monitorPlayback(n *model.Narrative) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if s.audio.IsBusy() {
			continue
		}

		// Use Title for non-POI narratives, DisplayName() for POI narratives
		displayName := n.Title
		displayID := string(n.Type)
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
	s.currentTopic = nil
	s.currentEssayTitle = ""
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
		slog.Error("Narrator: Queue playback failed, trying next", "title", next.Title, "error", err)
		// Try next immediately? Or assume PlayNarrative cleanup triggers monitor?
		// PlayNarrative returns error implies it didn't start.
		// So we should try next recursion.
		go s.processQueue(ctx)
	}
}

// promoteInQueue checks if a POI is already in the queue and promotes it if manual.
// Returns true if the POI was found and handled (promoted or already exists).
func (s *AIService) promoteInQueue(poiID string, manual bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, n := range s.queue {
		if n.POI != nil && n.POI.WikidataID == poiID {
			if manual {
				// Move to front (High Priority)
				s.queue = append(s.queue[:i], s.queue[i+1:]...)
				s.queue = append([]*model.Narrative{n}, s.queue...)
				slog.Info("Narrator: Boosting queued item to front", "poi_id", poiID)
			} else {
				slog.Info("Narrator: Item already in queue, skipping duplicate request", "poi_id", poiID)
			}
			return true
		}
	}
	return false
}
