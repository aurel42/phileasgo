package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"phileasgo/pkg/model"
	"time"
)

func (s *AIService) handleGenerationState(req *GenerationRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.SkipBusyCheck {
		// Caller already claimed the lock (e.g. from the queue worker or detached worker)
		s.generating = true
		return nil
	}

	if s.generating {
		return fmt.Errorf("narrator already generating")
	}
	s.generating = true
	return nil
}

// claimGeneration synchronously claims the busy state for the narrator.
// Returns true if claimed, false if already busy.
func (s *AIService) claimGeneration(p *model.POI) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generating {
		return false
	}
	s.generating = true
	s.generatingPOI = p
	return true
}

// releaseGeneration releases the busy state for the narrator.
func (s *AIService) releaseGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generating = false
	s.generatingPOI = nil
}

// IsActive returns true if narrator is currently active (generating or playing).
func (s *AIService) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active || s.generating || s.playbackQ.Count() > 0 || s.genQ.Count() > 0
}

// IsGenerating returns true if narrator is currently generating script/audio.
func (s *AIService) IsGenerating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generating || s.genQ.Count() > 0
}

// HasStagedAuto returns true if an automatic POI or Essay is currently generating or in the playback queue.
func (s *AIService) HasStagedAuto() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Is actively generating an auto-job?
	// Note: s.generatingPOI tracks the current POI being generated, but we rely on queue checks below.

	// Simplest check: if we are generating OR have items in generation queue OR have auto items in playback queue.
	if s.generating || s.genQ.Count() > 0 {
		return true
	}

	if s.playbackQ.HasAuto() {
		return true
	}

	return false
}

// IsPlaying returns true if narrator is currently playing audio (or checking busy state).
func (s *AIService) IsPlaying() bool {
	return s.audio.IsBusy()
}

// IsPaused returns true if the narrator is globally paused by the user.
func (s *AIService) IsPaused() bool {
	return s.audio.IsUserPaused()
}

// CurrentPOI returns the POI currently being narrated, if any.
func (s *AIService) CurrentPOI() *model.POI {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Regular POI
	if s.currentPOI != nil {
		return s.currentPOI
	}

	// 2. Pseudo POI for Screenshots
	if s.currentType == model.NarrativeTypeScreenshot && s.currentImagePath != "" {
		return &model.POI{
			WikidataID:   "screenshot-" + s.currentImagePath, // Unique-ish ID
			NameEn:       "Visual Analysis",
			NameUser:     "Visual Analysis",
			Category:     "Photograph",
			ThumbnailURL: "/api/images/serve?path=" + s.currentImagePath, // Use the serve endpoint as thumbnail
			Icon:         "camera",                                       // Hypothetical icon
			Score:        50.0,                                           // Max score for visibility
			Lat:          s.currentLat,
			Lon:          s.currentLon,
			IsVisible:    true,
		}
	}

	return nil
}

// CurrentImagePath returns the file path of the message for the current narration.
func (s *AIService) CurrentImagePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentImagePath
}

// ClearCurrentImage clears the current image path from state.
func (s *AIService) ClearCurrentImage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentImagePath = ""
}

// IsPOIBusy returns true if the POI is currently generating, queued, or playing.
func (s *AIService) IsPOIBusy(poiID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Check Generating
	if s.generatingPOI != nil && s.generatingPOI.WikidataID == poiID {
		return true
	}

	// 2. Check Playing
	if s.currentPOI != nil && s.currentPOI.WikidataID == poiID {
		return true
	}

	// 3. Check Playback Queue
	if s.playbackQ.HasPOI(poiID) {
		return true
	}

	// 4. Check Generation Queue
	// 4. Check Generation Queue
	if s.genQ.HasPOI(poiID) {
		return true
	}

	return false
}

// GetPreparedPOI returns the POI being prepared for pipeline playback, if any.
func (s *AIService) GetPreparedPOI() *model.POI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Check playbackQueue[0] or actively generating POI
	if next := s.playbackQ.Peek(); next != nil && next.POI != nil {
		return next.POI
	}
	return s.generatingPOI
}

// CurrentTitle returns the title of the current narration.
func (s *AIService) CurrentTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentPOI != nil {
		return s.currentPOI.DisplayName()
	}
	// Check for pseudo-POI logic (though CurrentPOI handles it, sometimes we just want the string)
	if s.currentType == model.NarrativeTypeScreenshot {
		return "Visual Analysis"
	}
	if s.currentEssayTitle != "" {
		return s.currentEssayTitle
	}
	if s.currentTopic != nil {
		return "Essay about " + s.currentTopic.Name
	}
	return ""
}

// CurrentType returns the type of the current narration.
func (s *AIService) CurrentType() model.NarrativeType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentType
}

// Remaining returns the remaining duration of the current narration.
func (s *AIService) Remaining() time.Duration {
	return s.audio.Remaining()
}

func (s *AIService) ReplayLast(ctx context.Context) bool {
	// 1. Check Audio Replay Capability
	// Pass finalizePlayback as callback
	if !s.audio.ReplayLastNarration(s.finalizePlayback) {
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
	case s.lastImagePath != "":
		slog.Info("Narrator: Replaying last Screenshot", "image", s.lastImagePath)
		s.currentImagePath = s.lastImagePath
		s.currentType = model.NarrativeTypeScreenshot
		s.currentLat = s.lastLat
		s.currentLon = s.lastLon
		s.active = true
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

	// 3. No manual monitoring needed, callback handles cleanup
	return true
}

// finalizePlayback handles state cleanup and queue processing when audio finishes.
func (s *AIService) finalizePlayback() {
	// Pacing: Wait before clearing active state to prevent back-to-back narration bombardment
	time.Sleep(3 * time.Second)

	// 1. Cleanup State
	s.mu.Lock()
	s.active = false
	s.currentPOI = nil
	s.currentTopic = nil
	s.currentEssayTitle = ""
	s.currentImagePath = ""
	s.currentType = ""
	s.mu.Unlock()

	// 2. Beacon Check (Switch to next target)
	if s.beaconSvc != nil {
		next := s.playbackQ.Peek()
		s.mu.RLock()
		generating := s.generatingPOI
		s.mu.RUnlock()

		if next != nil && next.POI != nil {
			slog.Info("Narrator: Switching marker to next queued POI", "qid", next.POI.WikidataID)
			_ = s.beaconSvc.SetTarget(context.Background(), next.POI.Lat, next.POI.Lon)
		} else if generating != nil {
			slog.Info("Narrator: Switching marker to currently generating POI", "qid", generating.WikidataID)
			_ = s.beaconSvc.SetTarget(context.Background(), generating.Lat, generating.Lon)
		}
	}

	// 3. Trigger Next
	slog.Info("Narrator: Playback finalized, checking queue")
	go s.ProcessPlaybackQueue(context.Background())
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
