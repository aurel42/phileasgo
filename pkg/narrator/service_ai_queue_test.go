package narrator

import (
	"context"
	"phileasgo/pkg/model"
	"testing"
)

func TestAIService_QueueManagement(t *testing.T) {
	svc := &AIService{}

	// 1. Enqueue Auto POI
	svc.enqueuePlayback(&model.Narrative{Title: "Auto", Manual: false, Type: "poi"}, false)
	if len(svc.playbackQueue) != 1 {
		t.Error("expected 1 item in queue")
	}

	// 2. Enqueue Priority Manual POI
	svc.enqueuePlayback(&model.Narrative{Title: "Manual", Manual: true, Type: "poi"}, true)
	if len(svc.playbackQueue) != 2 || svc.playbackQueue[0].Title != "Manual" {
		t.Error("priority item should be at the front")
	}

	// 3. Peek Queue
	n := svc.peekPlaybackQueue()
	if n == nil || n.Title != "Manual" {
		t.Error("peek failed")
	}

	// 4. Pop Queue
	n2 := svc.popPlaybackQueue()
	if n2 == nil || n2.Title != "Manual" || len(svc.playbackQueue) != 1 {
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
	svc.playbackQueue = []*model.Narrative{{Type: "poi"}}
	svc.ResetSession(context.Background())
	if len(svc.playbackQueue) != 0 {
		t.Error("expected empty queue after reset")
	}

	// 7. promoteInQueue - Boost case
	svc.playbackQueue = []*model.Narrative{
		{POI: &model.POI{WikidataID: "Q1"}, Type: "poi"},
		{POI: &model.POI{WikidataID: "Q2"}, Type: "poi"},
	}
	if !svc.promoteInQueue("Q2", true) {
		t.Error("expected true (found Q2)")
	}
	if svc.playbackQueue[0].POI.WikidataID != "Q2" {
		t.Error("Q2 should be boosted to front")
	}

	// 8. handleManualQueueAndOverride - Generating case
	svc.generating = true
	if !svc.handleManualQueueAndOverride("Q3", "long", true, true) {
		t.Error("expected true (enqueued priority job)")
	}
}

func TestAIService_QueueLimits(t *testing.T) {
	svc := &AIService{}

	// 1. Allow Manual POI when empty
	if !checkQueueLimits(svc.playbackQueue, "poi", true) {
		t.Error("should allow manual POI when empty")
	}

	// 2. Deny Auto POI when auto already queued
	svc.playbackQueue = []*model.Narrative{{Type: "poi", Manual: false}}
	if checkQueueLimits(svc.playbackQueue, "poi", false) {
		t.Error("should deny second auto POI")
	}

	// 3. Allow Manual POI even if auto queued
	if !checkQueueLimits(svc.playbackQueue, "poi", true) {
		t.Error("should allow manual POI if only auto is queued")
	}

	// 4. Deny second Manual POI
	svc.playbackQueue = append(svc.playbackQueue, &model.Narrative{Type: "poi", Manual: true})
	if checkQueueLimits(svc.playbackQueue, "poi", true) {
		t.Error("should deny second manual POI")
	}

	// 5. Deny Screenshot if already queued
	svc.playbackQueue = []*model.Narrative{{Type: model.NarrativeTypeScreenshot}}
	if checkQueueLimits(svc.playbackQueue, "screenshot", true) {
		t.Error("should deny second screenshot")
	}

	// 6. Allow Screenshot if none queued
	svc.playbackQueue = nil
	if !checkQueueLimits(svc.playbackQueue, "screenshot", true) {
		t.Error("should allow screenshot when none queued")
	}

	// 7. Debrief and Essay limits
	svc.playbackQueue = []*model.Narrative{{Type: "debrief"}}
	if checkQueueLimits(svc.playbackQueue, "debrief", true) {
		t.Error("should deny second debrief")
	}
	svc.playbackQueue = []*model.Narrative{{Type: "essay"}}
	if checkQueueLimits(svc.playbackQueue, "essay", true) {
		t.Error("should deny second essay")
	}
}

func TestAIService_CanEnqueue(t *testing.T) {
	svc := &AIService{}
	if !svc.canEnqueuePlayback("poi", true) {
		t.Error("canEnqueue failed")
	}
}
