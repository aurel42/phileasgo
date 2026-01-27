package playback

import (
	"phileasgo/pkg/model"
	"testing"
)

func TestManager_Enqueue(t *testing.T) {
	m := NewManager()

	n1 := &model.Narrative{ID: "1", Title: "First"}
	m.Enqueue(n1, false)

	if m.Count() != 1 {
		t.Errorf("expected count 1, got %d", m.Count())
	}

	n2 := &model.Narrative{ID: "2", Title: "Second"}
	m.Enqueue(n2, false)

	if m.Peek().ID != "1" {
		t.Errorf("expected First at head")
	}

	// Priority
	n3 := &model.Narrative{ID: "3", Title: "Priority"}
	m.Enqueue(n3, true)

	if m.Peek().ID != "3" {
		t.Errorf("expected Priority at head")
	}
}

func TestManager_Pop(t *testing.T) {
	m := NewManager()
	m.Enqueue(&model.Narrative{ID: "1"}, false)
	m.Enqueue(&model.Narrative{ID: "2"}, false)

	n := m.Pop()
	if n.ID != "1" {
		t.Errorf("expected 1, got %s", n.ID)
	}

	if m.Count() != 1 {
		t.Errorf("expected count 1, got %d", m.Count())
	}

	n = m.Pop()
	if n.ID != "2" {
		t.Errorf("expected 2, got %s", n.ID)
	}

	n = m.Pop()
	if n != nil {
		t.Errorf("expected nil from empty queue")
	}
}

func TestManager_CanEnqueue(t *testing.T) {
	m := NewManager()

	// 1. Auto POI allowed when empty
	if !m.CanEnqueue(model.NarrativeTypePOI, false) {
		t.Error("should allow auto POI when empty")
	}

	// 2. Enqueue Auto POI
	m.Enqueue(&model.Narrative{Type: model.NarrativeTypePOI, Manual: false}, false)

	// 3. Auto POI NOT allowed when not empty
	if m.CanEnqueue(model.NarrativeTypePOI, false) {
		t.Error("should NOT allow auto POI when not empty")
	}

	// 4. Manual POI allowed (max 1)
	if !m.CanEnqueue(model.NarrativeTypePOI, true) {
		t.Error("should allow manual POI")
	}
	m.Enqueue(&model.Narrative{Type: model.NarrativeTypePOI, Manual: true}, true)

	// 5. Manual POI NOT allowed (max 1 reached)
	if m.CanEnqueue(model.NarrativeTypePOI, true) {
		t.Error("should NOT allow second manual POI")
	}

	// 6. Limits for other types
	m.Clear()
	if !m.CanEnqueue(model.NarrativeTypeScreenshot, true) {
		t.Error("should allow screenshot")
	}
	m.Enqueue(&model.Narrative{Type: model.NarrativeTypeScreenshot}, true)
	if m.CanEnqueue(model.NarrativeTypeScreenshot, true) {
		t.Error("should NOT allow second screenshot")
	}
}

func TestManager_MaxSize(t *testing.T) {
	m := NewManager()

	// Fill queue
	for i := 0; i < 5; i++ {
		m.Enqueue(&model.Narrative{Type: model.NarrativeTypePOI, Manual: true}, false)
	}

	// Should reject low priority
	m.Enqueue(&model.Narrative{ID: "overflow", Manual: true}, false)

	// We need to implement logic such that Enqueue silently drops or we handle it?
	// The original code: "if len(s.playbackQueue) >= 5 && !priority { return }"
	// So count should still be 5
	if m.Count() != 5 {
		t.Errorf("expected 5 items, got %d", m.Count())
	}
}
func TestManager_HasPOI(t *testing.T) {
	m := NewManager()
	n := &model.Narrative{POI: &model.POI{WikidataID: "Q1"}}
	m.Enqueue(n, false)

	if !m.HasPOI("Q1") {
		t.Error("expected HasPOI(Q1) to be true")
	}
	if m.HasPOI("Q2") {
		t.Error("expected HasPOI(Q2) to be false")
	}
}

func TestManager_Promote(t *testing.T) {
	m := NewManager()
	m.Enqueue(&model.Narrative{ID: "1", POI: &model.POI{WikidataID: "Q1"}}, false)
	m.Enqueue(&model.Narrative{ID: "2", POI: &model.POI{WikidataID: "Q2"}}, false)

	if !m.Promote("Q2") {
		t.Fatal("expected Q2 to be promoted")
	}
	if m.Peek().ID != "2" {
		t.Errorf("expected 2 to be at head, got %s", m.Peek().ID)
	}
}

func TestManager_HasAuto(t *testing.T) {
	m := NewManager()
	m.Enqueue(&model.Narrative{Type: model.NarrativeTypePOI, Manual: false}, false)

	if !m.HasAuto() {
		t.Error("expected HasAuto to be true")
	}
}
