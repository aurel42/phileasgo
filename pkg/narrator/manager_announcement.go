package narrator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/sim"
)

// AnnouncementManager orchestrates the lifecycle of flight announcements.
type AnnouncementManager struct {
	mu           sync.RWMutex
	narrator     *AIService
	registry     map[string]Announcement
	triggerFlags map[string]bool // ID -> true if play was requested while generating
}

func NewAnnouncementManager(n *AIService) *AnnouncementManager {
	return &AnnouncementManager{
		narrator:     n,
		registry:     make(map[string]Announcement),
		triggerFlags: make(map[string]bool),
	}
}

func (m *AnnouncementManager) Register(a Announcement) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry[a.ID()] = a
}

// Tick evaluates all registered announcements against current telemetry.
func (m *AnnouncementManager) Tick(ctx context.Context, t *sim.Telemetry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, a := range m.registry {
		status := a.Status()

		switch status {
		case StatusIdle:
			// 1. Check if we should play NOW (Skip generation if we missed the window)
			if a.ShouldPlay(t) {
				slog.Debug("Announcement: Skipping (Play window reached while Idle)", "id", id)
				a.SetStatus(StatusDone)
				continue
			}

			// 2. Check if we should start generating
			if a.ShouldGenerate(t) {
				slog.Info("Announcement: Starting generation", "id", id)
				a.SetStatus(StatusGenerating)
				m.enqueueGeneration(ctx, a, t)
			}

		case StatusGenerating:
			// If playback is triggered while still generating, mark it for immediate play on completion.
			if a.ShouldPlay(t) {
				if !m.triggerFlags[id] {
					slog.Debug("Announcement: Playback triggered while generating", "id", id)
					m.triggerFlags[id] = true
				}
			}

		case StatusHeld:
			// Move to playback if condition met
			if a.ShouldPlay(t) {
				m.triggerPlayback(a)
			}
		}
	}
}

func (m *AnnouncementManager) enqueueGeneration(ctx context.Context, a Announcement, t *sim.Telemetry) {
	id := a.ID()

	// Create job with callback
	job := &generation.Job{
		Type:      a.Type(),
		Telemetry: t,
		CreatedAt: time.Now(),
		OnComplete: func(n *model.Narrative) {
			m.onResult(id, n)
		},
	}

	// Custom prompt data if needed
	if data, err := a.GetPromptData(t); err == nil {
		// We'll need to handle the prompt rendering here or in AIService.
		// For Phase 1, we assume the specific implementation handles its own prompt logic
		// OR we extend the Job struct further.
		_ = data
	}

	m.narrator.enqueueGeneration(job)
}

func (m *AnnouncementManager) onResult(id string, n *model.Narrative) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.registry[id]
	if !ok {
		return
	}

	a.SetHeldNarrative(n)
	a.SetStatus(StatusHeld)

	// If a playback trigger happened while we were generating, play now!
	if m.triggerFlags[id] {
		slog.Info("Announcement: Immediate playback (pending trigger)", "id", id)
		delete(m.triggerFlags, id)
		m.triggerPlayback(a)
	}
}

func (m *AnnouncementManager) triggerPlayback(a Announcement) {
	n := a.GetHeldNarrative()
	if n != nil {
		m.narrator.enqueuePlayback(n, true)
		go m.narrator.ProcessPlaybackQueue(context.Background())
	}
	a.SetStatus(StatusTriggered)
}

func (m *AnnouncementManager) ResetSession(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, a := range m.registry {
		a.SetStatus(StatusIdle)
		a.SetHeldNarrative(nil)
		delete(m.triggerFlags, id)
	}
}
