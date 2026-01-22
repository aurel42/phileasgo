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
	PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string)
	// PlayImage triggers narration for a local image file.
	PlayImage(ctx context.Context, imagePath string, tel *sim.Telemetry)
	// PrepareNextNarrative prepares a narrative for a POI and stages it for later playback.
	PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error
	// GetPreparedPOI returns the POI currently staged or generating, if any.
	GetPreparedPOI() *model.POI
	// HasStagedAuto returns true if an automatic POI or Essay is currently generating or queued.
	HasStagedAuto() bool
	// HasPendingManualOverride returns true if a user-requested POI is queued.
	HasPendingManualOverride() bool
	// GetPendingManualOverride returns and clears the pending manual override.
	GetPendingManualOverride() (string, string, bool)
	// GenerateNarrative prepares a narrative for a POI without playing it.
	GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error)
	// ProcessPlaybackQueue attempts to play the next item in the ready queue.
	ProcessPlaybackQueue(ctx context.Context)
	// PlayNarrative plays a previously generated narrative.
	PlayNarrative(ctx context.Context, n *model.Narrative) error
	// PlayEssay triggers a regional essay narration.
	PlayEssay(ctx context.Context, tel *sim.Telemetry) bool
	// PlayDebrief triggers a post-landing debrief.
	PlayDebrief(ctx context.Context, tel *sim.Telemetry) bool
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
	// CurrentType returns the type of the current narration.
	CurrentType() model.NarrativeType
	// Explanation: We need Remaining() to calculate pipeline trigger
	// Remaining returns the estimated remaining duration of the current playback.
	Remaining() time.Duration
	// ReplayLast triggers replay of the last narrated item and restores its state.
	ReplayLast(ctx context.Context) bool
	// AverageLatency returns the rolling average of generation time.
	AverageLatency() time.Duration
	// CurrentImagePath returns the file path of the message for the current narration.
	CurrentImagePath() string
	// IsPOIBusy returns true if the POI is currently generating, queued, or playing.
	IsPOIBusy(poiID string) bool
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
func (s *StubService) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if manual {
		slog.Info("Narrator stub: manual play requested", "poi_id", poiID)
	} else {
		slog.Info("Narrator stub: automated play triggering", "poi_id", poiID)
	}
	s.narratedPOIs[poiID] = true
}

// PlayImage triggers narration for a image (stub: just logs).
func (s *StubService) PlayImage(ctx context.Context, imagePath string, tel *sim.Telemetry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Info("Narrator stub: image play requested", "path", imagePath)
}

// PrepareNextNarrative prepares a narrative (stub: just logs).
func (s *StubService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	slog.Info("Narrator stub: preparing narrative for POI", "poi_id", poiID)
	return nil
}

// GetPreparedPOI returns the POI being prepared (stub: nil).
func (s *StubService) GetPreparedPOI() *model.POI {
	return nil
}

// HasStagedAuto returns true if an automatic narration is staged (stub: false).
func (s *StubService) HasStagedAuto() bool {
	return false
}

// HasPendingManualOverride returns true if pending (stub: false).
func (s *StubService) HasPendingManualOverride() bool {
	return false
}

// GetPendingManualOverride returns pending override (stub: none).
func (s *StubService) GetPendingManualOverride() (poiID, strategy string, ok bool) {
	return "", "", false
}

// GenerateNarrative prepares a narrative (stub).
func (s *StubService) GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error) {
	slog.Info("Narrator stub: generated narrative", "type", req.Type)
	return &model.Narrative{
		Type:     req.Type,
		Title:    req.Title,
		Script:   "Stub script",
		Duration: time.Second,
	}, nil
}

// ProcessPlaybackQueue attempts to play the next item in the queue (stub).
func (s *StubService) ProcessPlaybackQueue(ctx context.Context) {
	// Stub doesn't have a queue
}

// PlayNarrative plays a narrative (stub).
func (s *StubService) PlayNarrative(ctx context.Context, n *model.Narrative) error {
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

// PlayDebrief triggers a post-landing debrief (stub).
func (s *StubService) PlayDebrief(ctx context.Context, tel *sim.Telemetry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	slog.Info("Narrator stub: debrief requested")
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

// CurrentType returns the type of the current narration (stub: empty).
func (s *StubService) CurrentType() model.NarrativeType {
	return ""
}

// Remaining returns the remaining duration (stub: 0).
func (s *StubService) Remaining() time.Duration {
	return 0
}

// ReplayLast replays the last narration (stub: always false).
func (s *StubService) ReplayLast(ctx context.Context) bool {
	return false
}

// AverageLatency returns the rolling average of generation time (stub: 0).
func (s *StubService) AverageLatency() time.Duration {
	return 0
}

// CurrentImagePath returns the file path of the message for the current narration (stub: empty).
func (s *StubService) CurrentImagePath() string {
	return ""
}

// IsPOIBusy returns true if the POI is currently generating, queued, or playing (stub: false).
func (s *StubService) IsPOIBusy(poiID string) bool {
	return false
}
