package generation

import (
	"log/slog"
	"sync"
)

// Manager manages the generation queue for narrator jobs.
type Manager struct {
	mu    sync.RWMutex
	queue []*Job
}

// NewManager creates a new generation queue manager.
func NewManager() *Manager {
	return &Manager{
		queue: make([]*Job, 0),
	}
}

// Enqueue adds a job to the queue.
// For generation, we just append to the end. Priority is handled by the nature of the job/caller,
// or if we need priority insertion, we can add it. Current logic was simple append.
func (m *Manager) Enqueue(job *Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, job)
	slog.Info("GenerationQueue: Enqueued job", "type", job.Type, "poi_id", job.POIID, "queue_len", len(m.queue))
}

// Pop retrieves and removes the next job from the queue.
func (m *Manager) Pop() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) == 0 {
		return nil
	}
	job := m.queue[0]
	m.queue = m.queue[1:]
	return job
}

// Peek returns the head of the queue without removing it.
func (m *Manager) Peek() *Job {
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

// Clear empties the queue.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = make([]*Job, 0)
}

// HasPending returns true if the queue is not empty.
func (m *Manager) HasPending() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queue) > 0
}

// HasPOI returns true if the queue contains a job for the given POI.
func (m *Manager) HasPOI(poiID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, job := range m.queue {
		if job.POIID == poiID {
			return true
		}
	}
	return false
}
