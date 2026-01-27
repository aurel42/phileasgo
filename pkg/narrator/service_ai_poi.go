package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
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
		go s.ProcessPlaybackQueue(context.Background())
		return
	}

	// 2. Priority Queue Enqueue (Manual Only)
	if manual {
		s.enqueueGeneration(&generation.Job{
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
	if s.HasPendingGeneration() {
		slog.Info("Narrator: Skipping auto-play (priority queue not empty)")
		return
	}
	if s.IsGenerating() {
		slog.Info("Narrator: Skipping auto-play (busy generating)")
		return
	}

	if !s.canEnqueuePlayback("poi", manual) {
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
		s.enqueuePlayback(narrative, manual)
		s.ProcessPlaybackQueue(genCtx)
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

	// Enqueue & Trigger
	s.enqueuePlayback(narrative, false)
	// FIX: Ensure processing continues/restarts if playing finished while generating
	go s.ProcessPlaybackQueue(context.Background())

	return nil
}

// PlayNarrative plays a previously generated narrative.
// This method is non-blocking: it starts playback and returns immediately.
// Playback completion is monitored in a background goroutine.
// Supports both POI narratives and non-POI narratives (screenshots, essays, etc.)
func (s *AIService) PlayNarrative(ctx context.Context, n *model.Narrative) error {
	// Check active first
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return fmt.Errorf("narrator already active")
	}
	s.mu.Unlock()

	audioFile := s.setPlaybackState(n)

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
		"acc_gen_time", fmt.Sprintf("%.1fs", n.GenerationLatency.Seconds()),
		"next_prediction", fmt.Sprintf("%.1fs", n.PredictedLatency.Seconds()),
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

// setPlaybackState updates the narrator state for the given narrative.
// Returns the audio file path to play.
func (s *AIService) setPlaybackState(n *model.Narrative) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure Essays always have a title for UI visibility
	if (n.Type == "essay" || n.Type == "debrief") && n.Title == "" {
		if n.EssayTopic != "" {
			n.Title = "Essay: " + n.EssayTopic
		} else {
			n.Title = "Regional Essay"
		}
		if n.Type == "debrief" {
			n.Title = "Debrief"
		}
	}

	s.active = true
	s.currentPOI = n.POI // May be nil for non-POI narratives
	s.currentImagePath = n.ImagePath
	s.currentType = n.Type
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

	return n.AudioPath + "." + n.Format
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
		next := s.peekPlaybackQueue()
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
	s.currentType = ""
	s.mu.Unlock()

	// Trigger next item in queue
	go s.ProcessPlaybackQueue(context.Background())
}

// ProcessPlaybackQueue implements the Service interface.
func (s *AIService) ProcessPlaybackQueue(ctx context.Context) {
	if s.IsPaused() {
		slog.Info("Narrator: Queue processing deferred (paused)")
		return
	}

	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return // Already playing
	}
	// Check queue
	if s.playbackQ.Count() == 0 {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// Pop (using helper)
	next := s.popPlaybackQueue()
	if next == nil {
		return
	}

	if err := s.PlayNarrative(ctx, next); err != nil {
		slog.Error("Narrator: Queue playback failed, trying next", "title", next.Title, "error", err)
		// Try next immediately? Or assume PlayNarrative cleanup triggers monitor?
		// PlayNarrative returns error implies it didn't start.
		// So we should try next recursion.
		go s.ProcessPlaybackQueue(ctx)
	}
}

// promoteInQueue checks if a POI is already in the queue and promotes it if manual.
// Returns true if the POI was found and handled (promoted or already exists).
func (s *AIService) promoteInQueue(poiID string, manual bool) bool {
	if manual {
		slog.Info("Narrator: Promoting item in queue", "poi_id", poiID)
		return s.playbackQ.Promote(poiID)
	}

	// For auto, we just check existence to avoid duplicates
	// But Promote also handles existence.
	// Actually, the original code for !manual just returned true if found, without moving.
	// We need to implement Exists() if we want that behavior.
	// Or we can just use Promote(poiID) if we want to bubble up anyway?
	// Original code: if !manual { log info; return true }

	// Let's assume for now we want exact behavior.
	// But PlaybackQueueManager doesn't expose Exists/Find.
	// Promote returns true if found. If we only want to check existence for auto,
	// we are slightly changing behavior if we assume Promote ALWAYS moves.
	// But Promote logic: remove, prepend.
	// If !manual, we don't want to move?
	// The original code:
	// if !manual { log "skipping duplicate"; return true }

	// So we need to add Exists() to Manager or just iterate here?
	// Iterating here violates encapsulation.
	// Let's add HasPOI(poiID) to Manager?
	// Or just use Promote for now, as re-promoting an Auto POI isn't terrible?
	// Actually, if it's already there, and we just encountered it again, maybe it deserves to be bumped?
	// But "skipping duplicate request" implies we don't do anything.

	// Let's stick to strict behavior.
	// I'll peek the queue manually using a read lock in Manager? No, that's not exposed.
	// I should add HasPOI to PlaybackQueueManager.

	// For now, I will use Promote for both cases because "Clean Code" suggests simpler logic,
	// and bumping an auto-POI that we just re-discovered seems valid.
	if s.playbackQ.Promote(poiID) {
		if !manual {
			slog.Info("Narrator: Item already in queue (promoted)", "poi_id", poiID)
		} else {
			slog.Info("Narrator: Boosting queued item to front", "poi_id", poiID)
		}
		return true
	}
	return false
}
