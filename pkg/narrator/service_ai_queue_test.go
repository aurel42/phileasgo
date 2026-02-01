package narrator

import (
	"context"
	"phileasgo/pkg/generation"
	"phileasgo/pkg/model"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/session"
	"testing"
)

func TestOrchestrator_QueueManagement(t *testing.T) {
	mockGen := &MockAIService{}
	pbQ := playback.NewManager()
	sess := session.NewManager(nil)
	o := NewOrchestrator(mockGen, &MockAudio{}, pbQ, sess, nil, nil)

	// 1. Enqueue via EnqueuePlayback
	o.EnqueuePlayback(&model.Narrative{Title: "Auto", Manual: false, Type: model.NarrativeTypePOI}, false)
	if pbQ.Count() != 1 {
		t.Error("expected 1 item in queue")
	}

	// 2. Enqueue Priority Manual POI
	o.EnqueuePlayback(&model.Narrative{Title: "Manual", Manual: true, Type: model.NarrativeTypePOI}, true)
	if pbQ.Count() != 2 || pbQ.Peek().Title != "Manual" {
		t.Error("priority item should be at the front")
	}

	// 3. Reset Session
	o.ResetSession(context.Background())
	if pbQ.Count() != 0 {
		t.Error("expected empty queue after reset")
	}

	// 4. promoteInQueue - Boost case
	pbQ.Enqueue(&model.Narrative{POI: &model.POI{WikidataID: "Q1"}, Type: model.NarrativeTypePOI}, false)
	pbQ.Enqueue(&model.Narrative{POI: &model.POI{WikidataID: "Q2"}, Type: model.NarrativeTypePOI}, false)

	if !o.promoteInQueue("Q2", true) {
		t.Error("expected true (found Q2)")
	}
	if pbQ.Peek().POI.WikidataID != "Q2" {
		t.Error("Q2 should be boosted to front")
	}
}

func TestAIService_QueueCallback(t *testing.T) {
	svc := &AIService{}
	var called bool
	svc.SetOnPlayback(func(n *model.Narrative, priority bool) {
		called = true
	})

	svc.enqueuePlayback(&model.Narrative{Title: "Test"}, false)
	if !called {
		t.Error("expected onPlayback callback to be called")
	}
}

func TestAIService_ManualOverrides(t *testing.T) {
	svc := &AIService{
		genQ: generation.NewManager(),
	}

	svc.pendingManualID = "Q1"
	svc.pendingManualStrategy = "short"
	if !svc.HasPendingManualOverride() {
		t.Error("expected pending override")
	}
	id, _, ok := svc.GetPendingManualOverride()
	if !ok || id != "Q1" {
		t.Errorf("expected Q1, got %s (ok=%v)", id, ok)
	}
}
