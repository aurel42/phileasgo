package generation

import (
	"phileasgo/pkg/model"
	"testing"
)

func TestManager_EnqueueDequeue(t *testing.T) {
	m := NewManager()

	if m.HasPending() {
		t.Error("expected empty queue")
	}

	job := &Job{Type: model.NarrativeTypePOI}
	m.Enqueue(job)

	if !m.HasPending() {
		t.Error("expected pending items")
	}
	if m.Count() != 1 {
		t.Errorf("expected count 1, got %d", m.Count())
	}

	popped := m.Pop()
	if popped != job {
		t.Error("expected popped job to match enqueued job")
	}

	if m.HasPending() {
		t.Error("expected empty queue after pop")
	}
}

func TestManager_Clear(t *testing.T) {
	m := NewManager()
	m.Enqueue(&Job{Type: model.NarrativeTypePOI})
	m.Enqueue(&Job{Type: model.NarrativeTypeBorder})

	if m.Count() != 2 {
		t.Fatalf("expected 2 items, got %d", m.Count())
	}

	m.Clear()
	if m.HasPending() {
		t.Error("expected empty queue after clear")
	}
}

func TestManager_HasPOI(t *testing.T) {
	m := NewManager()
	m.Enqueue(&Job{Type: model.NarrativeTypePOI, POIID: "Q1"})
	m.Enqueue(&Job{Type: model.NarrativeTypePOI, POIID: "Q2"})

	if !m.HasPOI("Q1") {
		t.Error("expected queue to contain Q1")
	}
	if !m.HasPOI("Q2") {
		t.Error("expected queue to contain Q2")
	}
	if m.HasPOI("Q3") {
		t.Error("expected queue to NOT contain Q3")
	}
}
