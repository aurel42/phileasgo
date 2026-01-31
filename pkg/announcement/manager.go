package announcement

import (
	"context"
	"log/slog"
	"sync"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// GenerationProvider defines the dependency on the AI service for creating narratives.
type GenerationProvider interface {
	EnqueueAnnouncement(ctx context.Context, a Item, t *sim.Telemetry, onComplete func(*model.Narrative))
	Play(n *model.Narrative)
}

// Manager orchestrates the lifecycle of multiple flight announcements.
type Manager struct {
	mu       sync.RWMutex
	narrator GenerationProvider
	registry map[string]Item
}

func NewManager(n GenerationProvider) *Manager {
	return &Manager{
		narrator: n,
		registry: make(map[string]Item),
	}
}

func (m *Manager) Register(a Item) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry[a.ID()] = a
}

// Tick evaluates all registered announcements against current telemetry.
func (m *Manager) Tick(ctx context.Context, t *sim.Telemetry) {
	m.mu.Lock()
	var toGenerate []Item

	for id, a := range m.registry {
		status := a.Status()
		// Only log on status change is handled by the Announcement implementations if needed,
		// or we can remove this periodic log entirely as requested.

		switch status {
		case StatusIdle:
			// 1. Check if we should start generating
			if a.ShouldGenerate(t) {
				slog.Info("Announcement: Starting generation", "id", id)
				a.SetStatus(StatusGenerating)
				toGenerate = append(toGenerate, a)
				continue
			}

			// 2. Check if we missed the play window before generation started
			if a.ShouldPlay(t) {
				slog.Debug("Announcement: Missed window (Play triggered while Idle)", "id", id)
				if a.IsRepeatable() {
					// Repeatable announcements just wait for the next cycle
					continue
				}
				a.SetStatus(StatusMissed)
				continue
			}

		case StatusGenerating:
			// Just wait for completion. ShouldPlay will be checked in onResult.
			continue

		case StatusHeld:
			// Trigger playback if condition met
			if a.ShouldPlay(t) {
				m.triggerPlayback(a)
			}

		case StatusTriggered:
			if !a.IsRepeatable() {
				a.SetStatus(StatusDone)
			}
			// Waiting for reset
		}
	}
	m.mu.Unlock()

	// Execute generators outside the lock to prevent deadlocks on callback
	for _, a := range toGenerate {
		m.enqueueGeneration(ctx, a, t)
	}
}
func (m *Manager) enqueueGeneration(ctx context.Context, a Item, t *sim.Telemetry) {
	id := a.ID()
	m.narrator.EnqueueAnnouncement(ctx, a, t, func(n *model.Narrative) {
		m.onResult(id, n, t)
	})
}

func (m *Manager) onResult(id string, n *model.Narrative, t *sim.Telemetry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.registry[id]
	if !ok {
		slog.Warn("Announcement: Result for unregistered ID", "id", id)
		return
	}

	a.SetHeldNarrative(n)
	a.SetStatus(StatusHeld)

	// If the condition for playing is already met (or was met while generating), play now.
	if a.ShouldPlay(t) {
		m.triggerPlayback(a)
	}
}

func (m *Manager) triggerPlayback(a Item) {
	slog.Info("Announcement: Triggering playback", "id", a.ID(), "title", a.Title())
	m.narrator.Play(a.GetHeldNarrative())
	a.SetStatus(StatusTriggered)
}

func (m *Manager) ResetSession(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.registry {
		a.Reset()
	}
}
