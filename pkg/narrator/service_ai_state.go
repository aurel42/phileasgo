package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"phileasgo/pkg/model"
	"time"
)

// handleGenerationState manages the busy state and cancellation logic.
func (s *AIService) handleGenerationState(manual bool) error {
	s.mu.Lock()
	if s.generating {
		if manual {
			slog.Info("Narrator: Forcing generation, canceling previous job")
			if s.genCancelFunc != nil {
				s.genCancelFunc()
			}
			s.mu.Unlock()

			// Busy wait for cleanup
			timeout := time.After(5 * time.Second)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

		WaitLoop:
			for {
				select {
				case <-timeout:
					return fmt.Errorf("timeout waiting for previous generation to cancel")
				case <-ticker.C:
					s.mu.RLock()
					stillGen := s.generating
					s.mu.RUnlock()
					if !stillGen {
						break WaitLoop
					}
				}
			}
			s.mu.Lock()
		} else {
			s.mu.Unlock()
			return fmt.Errorf("narrator already generating")
		}
	}
	s.generating = true
	s.mu.Unlock()
	return nil
}

// IsActive returns true if narrator is currently active (generating or playing).
func (s *AIService) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active || s.generating || len(s.playbackQueue) > 0 || len(s.generationQueue) > 0
}

// IsGenerating returns true if narrator is currently generating script/audio.
func (s *AIService) IsGenerating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generating || len(s.generationQueue) > 0
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
	return s.currentPOI
}

// CurrentImagePath returns the file path of the message for the current narration.
func (s *AIService) CurrentImagePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentImagePath
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
	for _, n := range s.playbackQueue {
		if n.POI != nil && n.POI.WikidataID == poiID {
			return true
		}
	}

	// 4. Check Generation Queue
	for _, job := range s.generationQueue {
		if job.POIID == poiID {
			return true
		}
	}

	return false
}

// GetPreparedPOI returns the POI being prepared for pipeline playback, if any.
func (s *AIService) GetPreparedPOI() *model.POI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Check playbackQueue[0] or actively generating POI
	if len(s.playbackQueue) > 0 && s.playbackQueue[0].POI != nil {
		return s.playbackQueue[0].POI
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
		// Use Background context for the monitor to ensure it continues
		// even if the triggering request context is cancelled.
		ctx := context.Background()
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
