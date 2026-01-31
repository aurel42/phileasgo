package session

import (
	"encoding/json"
	"fmt"
	"sync"

	"phileasgo/pkg/prompt"
)

// Manager handles transient flight session context.
type Manager struct {
	mu            sync.RWMutex
	tripSummary   string
	lastSentence  string
	narratedCount int
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{}
}

// AddNarration records a narration in the trip history.
func (m *Manager) AddNarration(id, title, script string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastSentence = script

	// Append to trip summary
	if m.tripSummary != "" {
		m.tripSummary += " "
	}
	m.tripSummary += fmt.Sprintf("[%s]: %s", title, script)

	// Keep summary manageable? (Maybe later)
}

// IncrementCount increases the total narration count.
func (m *Manager) IncrementCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.narratedCount++
}

// SetTripSummary overwrites the current trip summary (e.g. from an LLM).
func (m *Manager) SetTripSummary(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tripSummary = s
}

// GetState returns the current session state for prompt assembly.
func (m *Manager) GetState() prompt.SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return prompt.SessionState{
		TripSummary:  m.tripSummary,
		LastSentence: m.lastSentence,
	}
}

// NarratedCount returns the total number of narrations.
func (m *Manager) NarratedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.narratedCount
}

// Reset clears the session state.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tripSummary = ""
	m.lastSentence = ""
	m.narratedCount = 0
}

// PersistentState represents the serializable part of the session.
type PersistentState struct {
	TripSummary   string  `json:"trip_summary"`
	LastSentence  string  `json:"last_sentence"`
	NarratedCount int     `json:"narrated_count"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
}

// GetPersistentState returns a JSON-encoded representation of the current session state.
func (m *Manager) GetPersistentState(lat, lon float64) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ps := PersistentState{
		TripSummary:   m.tripSummary,
		LastSentence:  m.lastSentence,
		NarratedCount: m.narratedCount,
		Lat:           lat,
		Lon:           lon,
	}

	return json.Marshal(ps)
}

// Restore rehydrates the session state from a JSON-encoded representation.
func (m *Manager) Restore(data []byte) error {
	var ps PersistentState
	if err := json.Unmarshal(data, &ps); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.tripSummary = ps.TripSummary
	m.lastSentence = ps.LastSentence
	m.narratedCount = ps.NarratedCount
	// Lat/Lon are stored for distance check, not needed in active state for now

	return nil
}

// UnmarshalLocation returns the lat/lon from the persisted state.
func UnmarshalLocation(data []byte) (lat, lon float64, err error) {
	var ps PersistentState
	if err := json.Unmarshal(data, &ps); err != nil {
		return 0, 0, err
	}
	return ps.Lat, ps.Lon, nil
}
