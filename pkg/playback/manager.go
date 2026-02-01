package playback

import (
	"log/slog"
	"phileasgo/pkg/model"
	"sync"
)

// Manager manages the playback queue for narratives.
type Manager struct {
	mu    sync.RWMutex
	queue []*model.Narrative
}

// NewManager creates a new playback queue manager.
func NewManager() *Manager {
	return &Manager{
		queue: make([]*model.Narrative, 0),
	}
}

// Enqueue adds a narrative to the playback queue.
func (m *Manager) Enqueue(n *model.Narrative, priority bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Max queue size check (e.g. 5)
	if len(m.queue) >= 5 && !priority {
		slog.Info("PlaybackQueue: Queue full, dropping low priority item", "title", n.Title)
		return
	}

	if priority {
		// Prepend
		m.queue = append([]*model.Narrative{n}, m.queue...)
	} else {
		// Append
		m.queue = append(m.queue, n)
	}
	slog.Debug("PlaybackQueue: Enqueued narrative", "title", n.Title, "priority", priority, "queue_len", len(m.queue))
}

// Pop retrieves and removes the next narrative from the queue.
func (m *Manager) Pop() *model.Narrative {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) == 0 {
		return nil
	}
	n := m.queue[0]
	m.queue = m.queue[1:]
	return n
}

// Peek returns the head of the queue without removing it.
func (m *Manager) Peek() *model.Narrative {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.queue) == 0 {
		return nil
	}
	return m.queue[0]
}

// Count returns the number of items in the queue.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queue)
}

// Clear clears the queue.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = make([]*model.Narrative, 0)
}

// CanEnqueue checks if a narrative of the given type can be enqueued based on current queue state.
func (m *Manager) CanEnqueue(nType model.NarrativeType, manual bool) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Auto POI/Essay: Only allowed if queue is empty
	if !manual && (nType == model.NarrativeTypePOI || nType == model.NarrativeTypeEssay) && len(m.queue) > 0 {
		return false
	}

	return m.checkLimits(nType, manual)
}

// checkLimits checks specific count limits for types.
func (m *Manager) checkLimits(nType model.NarrativeType, manual bool) bool {
	for _, n := range m.queue {
		if n.Type != nType {
			continue
		}

		switch nType {
		case model.NarrativeTypePOI:
			if manual && n.Manual {
				return false
			}
		case model.NarrativeTypeScreenshot, model.NarrativeTypeDebriefing, model.NarrativeTypeEssay, model.NarrativeTypeBorder:
			return false
		}
	}
	return true
}

// Promote promotes a narrative with the given POI ID to the front of the queue.
// Returns true if found and promoted.
func (m *Manager) Promote(poiID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, n := range m.queue {
		if n.POI != nil && n.POI.WikidataID == poiID {
			// Remove from current position
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			// Prepend
			m.queue = append([]*model.Narrative{n}, m.queue...)
			return true
		}
	}
	return false
}

// HasPOI checks if a POI with the given ID is in the queue.
func (m *Manager) HasPOI(poiID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, n := range m.queue {
		if n.POI != nil && n.POI.WikidataID == poiID {
			return true
		}
	}
	return false
}

// HasAuto checks if there are any automatic narratives in the queue.
func (m *Manager) HasAuto() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, n := range m.queue {
		if !n.Manual && (n.Type == model.NarrativeTypePOI || n.Type == model.NarrativeTypeEssay) {
			return true
		}
	}
	return false
}
