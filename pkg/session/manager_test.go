package session

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

type mockSimClient struct {
	sim.Client
	lat, lon float64
}

func (m *mockSimClient) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return sim.Telemetry{
		Latitude:  m.lat,
		Longitude: m.lon,
	}, nil
}

func TestManager(t *testing.T) {
	mockSim := &mockSimClient{lat: 48.8584, lon: 2.2945}
	m := NewManager(mockSim)

	// Initial state
	if m.NarratedCount() != 0 {
		t.Errorf("expected 0 count, got %d", m.NarratedCount())
	}

	// Add narration
	m.IncrementCount()
	m.AddNarration("Q123", "Eiffel Tower", "A tall iron tower. It was built in 1889.")
	if m.NarratedCount() != 1 {
		t.Errorf("expected 1 count, got %d", m.NarratedCount())
	}

	state := m.GetState()
	expected := "It was built in 1889."
	if state.LastSentence != expected {
		t.Errorf("expected last sentence '%s', got '%s'", expected, state.LastSentence)
	}

	// Test complex case
	m.AddNarration("Q124", "Louvre", "Some text. Another sentence! And a question? ")
	if m.GetState().LastSentence != "And a question?" {
		t.Errorf("failed complex sentence extraction: '%s'", m.GetState().LastSentence)
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

	// Add Event without timestamp (should default to now AND have coordinates)
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
	// Verify coordinates
	if state.Events[1].Lat != 48.8584 || state.Events[1].Lon != 2.2945 {
		t.Errorf("expected coordinates (48.8584, 2.2945), got (%.4f, %.4f)", state.Events[1].Lat, state.Events[1].Lon)
	}

	// Reset
	m.Reset()
	if m.NarratedCount() != 0 {
		t.Errorf("expected 0 count after reset")
	}
	if len(m.GetState().Events) != 0 {
		t.Errorf("expected 0 events after reset")
	}

	// ResetSession (Deep Reset)
	m.IncrementCount()
	m.ResetSession(context.Background())
	if m.NarratedCount() != 0 {
		t.Errorf("expected 0 count after ResetSession")
	}
}
