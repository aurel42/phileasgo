package session

import (
	"testing"
)

func TestManager(t *testing.T) {
	m := NewManager()

	// Initial state
	if m.NarratedCount() != 0 {
		t.Errorf("expected 0 count, got %d", m.NarratedCount())
	}

	// Add narration
	m.IncrementCount()
	m.AddNarration("Q123", "Eiffel Tower", "A tall iron tower.")
	if m.NarratedCount() != 1 {
		t.Errorf("expected 1 count, got %d", m.NarratedCount())
	}

	state := m.GetState()
	if state.LastSentence != "A tall iron tower." {
		t.Errorf("expected last sentence mismatch")
	}
	if state.TripSummary != "[Eiffel Tower]: A tall iron tower." {
		t.Errorf("expected trip summary mismatch: %s", state.TripSummary)
	}

	// Reset
	m.Reset()
	if m.NarratedCount() != 0 {
		t.Errorf("expected 0 count after reset")
	}
	if m.GetState().TripSummary != "" {
		t.Errorf("expected empty summary after reset")
	}
}
