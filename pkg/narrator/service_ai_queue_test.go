package narrator

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/narrator/playback"
	"testing"
)

func TestAIService_QueueManagement(t *testing.T) {
	svc := &AIService{
		playbackQ: playback.NewManager(),
		genQ:      generation.NewManager(),
	}

	// 1. Enqueue Auto POI
	svc.enqueuePlayback(&model.Narrative{Title: "Auto", Manual: false, Type: model.NarrativeTypePOI}, false)
	if svc.playbackQ.Count() != 1 {
		t.Error("expected 1 item in queue")
	}

	// 2. Enqueue Priority Manual POI
	svc.enqueuePlayback(&model.Narrative{Title: "Manual", Manual: true, Type: model.NarrativeTypePOI}, true)
	if svc.playbackQ.Count() != 2 || svc.playbackQ.Peek().Title != "Manual" {
		t.Error("priority item should be at the front")
	}

	// 3. Peek Queue
	n := svc.peekPlaybackQueue()
	if n == nil || n.Title != "Manual" {
		t.Error("peek failed")
	}

	// 4. Pop Queue
	n2 := svc.popPlaybackQueue()
	if n2 == nil || n2.Title != "Manual" || svc.playbackQ.Count() != 1 {
		t.Error("pop failed")
	}

	// 5. Manual Override Getters
	svc.pendingManualID = "Q1"
	svc.pendingManualStrategy = "short"
	if !svc.HasPendingManualOverride() {
		t.Error("expected pending override")
	}
	id, _, ok := svc.GetPendingManualOverride()
	if !ok || id != "Q1" {
		t.Errorf("expected Q1, got %s (ok=%v)", id, ok)
	}
	if svc.HasPendingManualOverride() {
		t.Error("override should be cleared after get")
	}

	// 6. Reset Session
	svc.playbackQ.Clear()
	svc.enqueuePlayback(&model.Narrative{Type: model.NarrativeTypePOI}, false)
	svc.ResetSession(context.Background())
	if svc.playbackQ.Count() != 0 {
		t.Error("expected empty queue after reset")
	}

	// 7. promoteInQueue - Boost case
	svc.playbackQ.Enqueue(&model.Narrative{POI: &model.POI{WikidataID: "Q1"}, Type: model.NarrativeTypePOI}, false)
	svc.playbackQ.Enqueue(&model.Narrative{POI: &model.POI{WikidataID: "Q2"}, Type: model.NarrativeTypePOI}, false)

	if !svc.promoteInQueue("Q2", true) {
		t.Error("expected true (found Q2)")
	}
	if svc.playbackQ.Peek().POI.WikidataID != "Q2" {
		t.Error("Q2 should be boosted to front")
	}

	// 8. handleManualQueueAndOverride - Generating case
	svc.generating = true
	if !svc.handleManualQueueAndOverride("Q3", "long", true, true) {
		t.Error("expected true (enqueued priority job)")
	}
	if svc.genQ.Count() != 1 {
		t.Errorf("expected 1 generation job, got %d", svc.genQ.Count())
	}
}

func TestAIService_CanEnqueue(t *testing.T) {
	svc := &AIService{
		playbackQ: playback.NewManager(),
	}
	if !svc.canEnqueuePlayback(model.NarrativeTypePOI, true) {
		t.Error("canEnqueue failed")
	}
}
