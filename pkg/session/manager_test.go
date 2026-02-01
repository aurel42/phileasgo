package session

import (
	"phileasgo/pkg/model"
	"testing"
	"time"
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

	// Add Event with explicit timestamp
	m.AddEvent(&model.TripEvent{
		Timestamp: time.Now(),
		Type:      "test",
		Title:     "Event 1",
		Summary:   "Summ 1",
	})
	state = m.GetState()
	if len(state.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(state.Events))
	}
	if state.Events[0].Title != "Event 1" {
		t.Errorf("expected title mismatch")
	}

	// Add Event without timestamp (should default to now)
	m.AddEvent(&model.TripEvent{
		Type:    "activity",
		Title:   "Event 2",
		Summary: "Auto-timestamped",
	})
	state = m.GetState()
	if len(state.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(state.Events))
	}
	if state.Events[1].Timestamp.IsZero() {
		t.Error("expected auto-generated timestamp, got zero")
	}

	// Reset
	m.Reset()
	if m.NarratedCount() != 0 {
		t.Errorf("expected 0 count after reset")
	}
	if len(m.GetState().Events) != 0 {
		t.Errorf("expected 0 events after reset")
	}
}
