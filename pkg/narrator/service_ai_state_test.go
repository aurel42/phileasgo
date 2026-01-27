package narrator

import (
	"context"
	"testing"

	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/narrator/playback"
)

func TestAIService_StateChecks(t *testing.T) {
	svc := &AIService{
		active:     true,
		generating: true,
		playbackQ:  playback.NewManager(),
		genQ:       generation.NewManager(),
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
		audio:     mockAudio,
		lastPOI:   &model.POI{NameEn: "Test POI"},
		active:    false,
		playbackQ: playback.NewManager(),
		genQ:      generation.NewManager(),
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
	svc := &AIService{
		playbackQ: playback.NewManager(),
		genQ:      generation.NewManager(),
	}

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
		audio:      mockAudio,
		playbackQ:  playback.NewManager(),
		genQ:       generation.NewManager(),
		currentPOI: &model.POI{NameEn: "Current POI"},
	}
	svc.playbackQ.Enqueue(&model.Narrative{POI: &model.POI{NameEn: "Queued POI"}}, false)

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
	svc.playbackQ.Clear()
	svc.generatingPOI = &model.POI{NameEn: "Gen POI"}
	p = svc.GetPreparedPOI()
	if p == nil || p.NameEn != "Gen POI" {
		t.Errorf("expected Gen POI, got %v", p)
	}
}
