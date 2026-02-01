package session

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
)

// Manager handles transient flight session context.
type Manager struct {
	mu            sync.RWMutex
	events        []model.TripEvent
	lastSentence  string
	narratedCount int
	stageData     sim.StageState
	sim           sim.Client
}

// NewManager creates a new session manager.
func NewManager(simClient sim.Client) *Manager {
	return &Manager{
		sim: simClient,
	}
}

// SetStageData updates the flight stage persistence data.
func (m *Manager) SetStageData(s sim.StageState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stageData = s
}

// GetStageData returns the flight stage persistence data.
func (m *Manager) GetStageData() sim.StageState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stageData
}

// AddNarration records a narration in the trip history.
func (m *Manager) AddNarration(id, title, script string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse last sentence (simple heuristic)
	// We want the last non-empty segment ending with . ! or ?
	m.lastSentence = extractLastSentence(script)
	// Note: We'll migrate this to AddEvent in a later step when we have summaries
}

func extractLastSentence(text string) string {
	// Simple implementation: split by punctuation
	// This is not perfect (e.g. "Mr. Smith") but sufficient for continuity context
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}

	// Work backwards
	end := len(runes) - 1
	// Skip trailing whitespace
	for end >= 0 && (runes[end] == ' ' || runes[end] == '\n' || runes[end] == '\t') {
		end--
	}
	if end < 0 {
		return ""
	}

	// 1. Find the end of the last sentence (punctuation)
	// If the text doesn't end with punctuation, use the whole thing? Or assume implicit dot?
	// Let's assume the text is well-formed.

	start := end
	// 2. Find the start of the last sentence
	// Scan backwards until we hit a punctuation mark that IS NOT the one at 'end'
	// punctuation: . ! ?
	// Ignoring tricky cases like "e.g." for now, as this is for LLM context, slight errors are fine.

	// Skip the punctuation at the end if present
	if isPunctuation(runes[end]) {
		start--
	}

	for start >= 0 {
		if isPunctuation(runes[start]) {
			// Found the end of the *previous* sentence
			start++ // Move forward to the start of *our* sentence
			break
		}
		start--
	}

	if start < 0 {
		start = 0
	}

	// Trim leading whitespace from the result
	res := string(runes[start : end+1])
	// Cleanup quotes if they are unbalanced? No, too complex.
	// Just trim spaces.
	return trimWhitespace(res)
}

func isPunctuation(r rune) bool {
	return r == '.' || r == '!' || r == '?'
}

func trimWhitespace(s string) string {
	// Manual trim to avoid import "strings" if not already imported?
	// But strings is standard. Let's use strings.TrimSpace if available, but I need to check imports.
	// session/manager.go imports: encoding/json, sync, time, logging, model, prompt, sim.
	// strings is NOT imported. I should add it or implemented simple trim.
	// Simple trim:
	start := 0
	runes := []rune(s)
	for start < len(runes) && (runes[start] == ' ' || runes[start] == '\n') {
		start++
	}
	return string(runes[start:])
}

// AddEvent adds a structured event to the session history.
func (m *Manager) AddEvent(event *model.TripEvent) {
	// Always attach current aircraft coordinates relative to where we are NOW
	if m.sim != nil {
		if tel, err := m.sim.GetTelemetry(context.Background()); err == nil {
			event.Lat = tel.Latitude
			event.Lon = tel.Longitude
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	m.events = append(m.events, *event)

	// Log to events.log
	logging.LogEvent(event)
}

// IncrementCount increases the total narration count.
func (m *Manager) IncrementCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.narratedCount++
}

// GetState returns the current session state for prompt assembly.
func (m *Manager) GetState() prompt.SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return prompt.SessionState{
		Events:       m.events,
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

	m.events = nil
	m.lastSentence = ""
	m.narratedCount = 0
	m.stageData = sim.StageState{}
}

// PersistentState represents the serializable part of the session.
type PersistentState struct {
	Events        []model.TripEvent `json:"events"`
	LastSentence  string            `json:"last_sentence"`
	NarratedCount int               `json:"narrated_count"`
	Lat           float64           `json:"lat"`
	Lon           float64           `json:"lon"`
	StageData     sim.StageState    `json:"stage_data"`
}

// GetPersistentState returns a JSON-encoded representation of the current session state.
func (m *Manager) GetPersistentState(lat, lon float64) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ps := PersistentState{
		Events:        m.events,
		LastSentence:  m.lastSentence,
		NarratedCount: m.narratedCount,
		Lat:           lat,
		Lon:           lon,
		StageData:     m.stageData,
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

	m.events = ps.Events
	m.lastSentence = ps.LastSentence
	m.narratedCount = ps.NarratedCount
	m.stageData = ps.StageData
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
