// Package apisession provides a generic, thread-safe session store for API
// handlers that need per-client state. Clients identify themselves with an
// opaque session ID (typically a UUID generated client-side).
package apisession

import (
	"sync"
	"time"
)

// cleanupInterval is how often Get() triggers lazy eviction of expired entries.
const cleanupInterval = 100

type entry[T any] struct {
	value      *T
	lastAccess time.Time
}

// Store is a typed, thread-safe session store. Each unique session ID maps to
// one instance of T, created on first access via the newFn factory.
type Store[T any] struct {
	mu       sync.Mutex
	entries  map[string]*entry[T]
	ttl      time.Duration
	newFn    func() *T
	getCalls int
}

// New creates a Store that evicts sessions inactive longer than ttl.
// newFn is called to initialise state when a session ID is seen for the first time.
func New[T any](ttl time.Duration, newFn func() *T) *Store[T] {
	return &Store[T]{
		entries: make(map[string]*entry[T]),
		ttl:     ttl,
		newFn:   newFn,
	}
}

// Get returns the state for the given session, creating it if needed.
// Each call refreshes the session's last-access timestamp.
func (s *Store[T]) Get(id string) *T {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.getCalls++
	if s.getCalls%cleanupInterval == 0 {
		s.cleanupLocked()
	}

	e, ok := s.entries[id]
	if !ok {
		e = &entry[T]{value: s.newFn()}
		s.entries[id] = e
	}
	e.lastAccess = time.Now()
	return e.value
}

// Cleanup evicts all sessions that have been inactive longer than the TTL.
func (s *Store[T]) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
}

func (s *Store[T]) cleanupLocked() {
	cutoff := time.Now().Add(-s.ttl)
	for id, e := range s.entries {
		if e.lastAccess.Before(cutoff) {
			delete(s.entries, id)
		}
	}
}

// Len returns the number of active sessions.
func (s *Store[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
