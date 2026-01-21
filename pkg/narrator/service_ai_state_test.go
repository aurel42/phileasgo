package narrator

import (
	"context"
	"testing"

	"phileasgo/pkg/model"
)

func TestAIService_StateChecks(t *testing.T) {
	svc := &AIService{
		active:     true,
		generating: true,
	}

	if !svc.IsActive() {
		t.Error("expected active")
	}
	if !svc.IsGenerating() {
		t.Error("expected generating")
	}

	svc.active = false
	svc.generating = false
	if svc.IsActive() {
		t.Error("expected inactive")
	}
}

func TestAIService_Replay(t *testing.T) {
	mockAudio := &MockAudio{IsPlayingVal: true, CanReplay: true}
	svc := &AIService{
		audio:   mockAudio,
		lastPOI: &model.POI{NameEn: "Test POI"},
		active:  false,
	}

	// 1. POI Replay
	if !svc.ReplayLast(context.Background()) {
		t.Error("expected replay success")
	}
	if !svc.IsActive() || svc.currentPOI == nil {
		t.Error("failed to restore POI state")
	}

	// 2. Essay Replay
	svc.lastPOI = nil
	svc.currentPOI = nil
	svc.lastEssayTopic = &EssayTopic{Name: "Flight"}
	svc.lastEssayTitle = "" // Test "Essay about Flight" fallback
	if !svc.ReplayLast(context.Background()) {
		t.Error("expected essay replay success")
	}
	if svc.CurrentTitle() != "Essay about Flight" {
		t.Errorf("expected Essay about Flight title, got %s", svc.CurrentTitle())
	}
}

func TestAIService_Cooldown(t *testing.T) {
	svc := &AIService{}

	// Skip cooldown
	svc.SkipCooldown()
	if !svc.ShouldSkipCooldown() {
		t.Error("expected skip cooldown active")
	}

	svc.ResetSkipCooldown()
	if svc.ShouldSkipCooldown() {
		t.Error("expected skip cooldown reset")
	}
}

func TestAIService_PlaybackDetails(t *testing.T) {
	mockAudio := &MockAudio{IsPlayingVal: true}
	svc := &AIService{
		audio: mockAudio,
		playbackQueue: []*model.Narrative{
			{POI: &model.POI{NameEn: "Queued POI"}},
		},
		currentPOI: &model.POI{NameEn: "Current POI"},
	}

	// 1. CurrentTitle
	if svc.CurrentTitle() != "Current POI" {
		t.Errorf("expected Current POI, got %s", svc.CurrentTitle())
	}

	// 2. IsPlaying
	if !svc.IsPlaying() {
		t.Error("expected playing")
	}

	// 3. Remaining (Delegates to audio)
	_ = svc.Remaining()

	// 3. GetPreparedPOI
	p := svc.GetPreparedPOI()
	if p == nil || p.NameEn != "Queued POI" {
		t.Errorf("expected Queued POI, got %v", p)
	}

	// 4. Generating fallback
	svc.playbackQueue = nil
	svc.generatingPOI = &model.POI{NameEn: "Gen POI"}
	p = svc.GetPreparedPOI()
	if p == nil || p.NameEn != "Gen POI" {
		t.Errorf("expected Gen POI, got %v", p)
	}
}
