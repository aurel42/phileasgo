package narrator

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/session"
	"testing"
)

// TestBeaconNotClearedOnPlayback verifies that playing any narrative
// does NOT actively clear beacons (SetTarget handles clearing when new target is set).
func TestBeaconNotClearedOnPlayback(t *testing.T) {
	mockBeacon := &MockBeacon{}
	mockGen := &MockAIService{}
	pbQ := playback.NewManager()
	sess := session.NewManager(nil)
	o := NewOrchestrator(mockGen, &MockAudio{}, pbQ, sess, mockBeacon, nil, nil, nil)

	// Use a mock audio that doesn't block and runs synchronously
	o.audio = &MockAudio{CanReplay: true, PlaySync: true}

	// Test Border narrative
	n := &model.Narrative{
		Type:      model.NarrativeTypeBorder,
		Title:     "Border Crossing",
		Script:    "Welcome to Italy",
		AudioPath: "test_audio",
		Format:    "mp3",
	}

	err := o.PlayNarrative(context.Background(), n)
	if err != nil {
		t.Fatalf("PlayNarrative failed: %v", err)
	}

	if mockBeacon.Cleared {
		t.Error("Beacon service should NOT be cleared for Border narratives")
	}

	// Test Screenshot narrative - also should NOT clear (SetTarget handles it)
	mockBeacon.Cleared = false

	nScreen := &model.Narrative{
		Type:      model.NarrativeTypeScreenshot,
		Title:     "Screenshot",
		Script:    "Wow",
		AudioPath: "test_audio",
		Format:    "mp3",
	}

	err = o.PlayNarrative(context.Background(), nScreen)
	if err != nil {
		t.Fatalf("PlayNarrative failed: %v", err)
	}

	if mockBeacon.Cleared {
		t.Error("Beacon service should NOT be cleared - SetTarget handles clearing when new target is set")
	}
}

func TestOrchestrator_Pause(t *testing.T) {
	mockAudio := &MockAudio{}
	o := NewOrchestrator(&MockAIService{}, mockAudio, playback.NewManager(), nil, nil, nil, nil, nil)

	mockAudio.SetUserPaused(true)
	if !o.IsPaused() {
		t.Error("Expected Orchestrator to be paused")
	}

	o.q.Enqueue(&model.Narrative{Title: "Paused", AudioPath: "test", Format: "mp3"}, false)
	o.ProcessPlaybackQueue(context.Background())

	if o.IsPlaying() {
		t.Error("Expected no playback while paused")
	}
}
