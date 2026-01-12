// Package narrator provides narration services for POIs.
package narrator

import (
	"context"
	"log/slog"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"sync"
	"time"
)

// Service defines the interface for narration control.
type Service interface {
	// Start starts the narrator service.
	Start()
	// Stop stops the narrator service.
	Stop()
	// IsActive returns true if narrator is currently generating or playing.
	IsActive() bool
	// IsGenerating returns true if narrator is currently generating script/audio.
	IsGenerating() bool
	// NarratedCount returns the number of narrated POIs in this session.
	NarratedCount() int
	// Stats returns narrator statistics.
	Stats() map[string]any
	// IsPlaying returns true if narration audio is currently playing.
	IsPlaying() bool
	// PlayPOI triggers narration for a specific POI.
	PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry, strategy string)
	// PrepareNextNarrative prepares a narrative for a POI and stages it for later playback.
	PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error
	// GenerateNarrative prepares a narrative for a POI without playing it.
	GenerateNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) (*Narrative, error)
	// PlayNarrative plays a previously generated narrative.
	PlayNarrative(ctx context.Context, n *Narrative) error
	// PlayEssay triggers a regional essay narration.
	PlayEssay(ctx context.Context, tel *sim.Telemetry) bool
	// SkipCooldown forces the cooldown to expire immediately.
	SkipCooldown()
	// ShouldSkipCooldown returns true if the cooldown should be skipped.
	ShouldSkipCooldown() bool
	// ResetSkipCooldown resets the skip cooldown flag.
	ResetSkipCooldown()
	// IsPaused returns true if the narrator is globally paused by the user.
	IsPaused() bool
	// CurrentPOI returns the POI currently being narrated, if any.
	CurrentPOI() *model.POI
	// CurrentTitle returns the title of the current narration.
	CurrentTitle() string
	// Explanation: We need Remaining() to calculate pipeline trigger
	// Remaining returns the estimated remaining duration of the current playback.
	Remaining() time.Duration
	// ReplayLast triggers replay of the last narrated item and restores its state.
	ReplayLast(ctx context.Context) bool
}

// Narrative represents a prepared narration ready for playback.
type Narrative struct {
	POI            *model.POI
	Script         string
	AudioPath      string
	Format         string // e.g., "mp3"
	Duration       time.Duration
	RequestedWords int
}

// StubService is a stub implementation of the narrator service.
// It logs calls but does not generate actual audio.
type StubService struct {
	mu           sync.RWMutex
	running      bool
	active       bool
	narratedPOIs map[string]bool
	stats        map[string]any
	skipCooldown bool
}

// NewStubService creates a new stub narrator service.
func NewStubService() *StubService {
	return &StubService{
		narratedPOIs: make(map[string]bool),
		stats: map[string]any{
			"gemini_text_success": 0,
			"gemini_text_fail":    0,
			"gemini_tts_success":  0,
			"gemini_tts_fail":     0,
			"gemini_text_active":  false,
			"gemini_tts_active":   false,
		},
		skipCooldown: false,
	}
}

// Start starts the stub narrator service.
func (s *StubService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	slog.Info("Narrator stub service started")
}

// Stop stops the stub narrator service.
func (s *StubService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	slog.Info("Narrator stub service stopped")
}

// IsActive returns true if narrator is currently active.
func (s *StubService) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// IsGenerating returns true if narrator is currently generating (stub).
func (s *StubService) IsGenerating() bool {
	return false // Stub doesn't generate
}

// NarratedCount returns the number of narrated POIs.
func (s *StubService) NarratedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.narratedPOIs)
}

// Stats returns narrator statistics.
func (s *StubService) Stats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to avoid race conditions
	result := make(map[string]any, len(s.stats))
	for k, v := range s.stats {
		result[k] = v
	}
	return result
}

// PlayPOI triggers narration for a specific POI (stub: just logs).
func (s *StubService) PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry, strategy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if manual {
		slog.Info("Narrator stub: manual play requested", "poi_id", poiID)
	} else {
		slog.Info("Narrator stub: automated play triggering", "poi_id", poiID)
	}
	s.narratedPOIs[poiID] = true
}

// PrepareNextNarrative prepares a narrative (stub: just logs).
func (s *StubService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	slog.Info("Narrator stub: preparing narrative for POI", "poi_id", poiID)
	return nil
}

// GenerateNarrative prepares a narrative (stub).
func (s *StubService) GenerateNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) (*Narrative, error) {
	slog.Info("Narrator stub: generated narrative for POI", "poi_id", poiID)
	return &Narrative{
		POI:      &model.POI{WikidataID: poiID},
		Script:   "Stub script",
		Duration: time.Second,
	}, nil
}

// PlayNarrative plays a narrative (stub).
func (s *StubService) PlayNarrative(ctx context.Context, n *Narrative) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Info("Narrator stub: playing narrative for POI", "poi_id", n.POI.WikidataID)
	s.narratedPOIs[n.POI.WikidataID] = true
	return nil
}

// PlayEssay triggers a regional essay narration (stub).
func (s *StubService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Info("Narrator stub: essay play requested")
	return true
}

// SkipCooldown forces the cooldown to expire immediately (stub: sets flag).
func (s *StubService) SkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = true
	slog.Debug("Narrator stub: skip cooldown requested")
}

// ShouldSkipCooldown returns true if the cooldown should be skipped.
func (s *StubService) ShouldSkipCooldown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.skipCooldown
}

// ResetSkipCooldown resets the skip cooldown flag.
func (s *StubService) ResetSkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = false
}

// IsPlaying returns true if narration audio is currently playing (stub: always false).
func (s *StubService) IsPlaying() bool {
	return false
}

// IsPaused returns true if the narrator is globally paused (stub: always false).
func (s *StubService) IsPaused() bool {
	return false
}

// CurrentPOI returns the POI currently being narrated (stub: always nil).
func (s *StubService) CurrentPOI() *model.POI {
	return nil
}

// CurrentTitle returns the title of the current narration (stub: empty).
func (s *StubService) CurrentTitle() string {
	return ""
}

// Remaining returns the remaining duration (stub: 0).
func (s *StubService) Remaining() time.Duration {
	return 0
}

// ReplayLast replays the last narration (stub: always false).
func (s *StubService) ReplayLast(ctx context.Context) bool {
	slog.Info("Narrator stub: ReplayLast requested")
	return false
}
