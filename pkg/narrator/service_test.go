package narrator

import (
	"context"
	"testing"

	"phileasgo/pkg/sim"
)

func TestStubService_Lifecycle(t *testing.T) {
	s := NewStubService()

	// Initial state
	if s.IsActive() {
		t.Error("New stub service should not be active")
	}
	if s.IsGenerating() {
		t.Error("New stub service should not be generating")
	}
	if s.IsPlaying() {
		t.Error("New stub service should not be playing")
	}
	if s.IsPaused() {
		t.Error("New stub service should not be paused")
	}

	// Start
	s.Start()
	// Stub doesn't change active state on Start, just logs
	// Stop
	s.Stop()
}

func TestStubService_PlayPOI(t *testing.T) {
	s := NewStubService()
	ctx := context.Background()

	// Play
	s.PlayPOI(ctx, "test-poi", true, nil, "uniform")

	if s.NarratedCount() != 1 {
		t.Errorf("NarratedCount() = %d, want 1", s.NarratedCount())
	}
}

func TestStubService_PlayEssay(t *testing.T) {
	s := NewStubService()
	ctx := context.Background()
	tel := &sim.Telemetry{}

	if !s.PlayEssay(ctx, tel) {
		t.Error("PlayEssay should return true for stub")
	}
}

func TestStubService_Cooldown(t *testing.T) {
	s := NewStubService()

	if s.ShouldSkipCooldown() {
		t.Error("ShouldSkipCooldown should be false initially")
	}

	s.SkipCooldown()
	if !s.ShouldSkipCooldown() {
		t.Error("ShouldSkipCooldown should be true after SkipCooldown")
	}

	s.ResetSkipCooldown()
	if s.ShouldSkipCooldown() {
		t.Error("ShouldSkipCooldown should be false after ResetSkipCooldown")
	}
}

func TestStubService_Stats(t *testing.T) {
	s := NewStubService()
	stats := s.Stats()

	expectedKeys := []string{
		"gemini_text_success",
		"gemini_text_fail",
		"gemini_tts_success",
		"gemini_tts_fail",
		"gemini_text_active",
		"gemini_tts_active",
	}

	for _, k := range expectedKeys {
		if _, ok := stats[k]; !ok {
			t.Errorf("Stats missing key: %s", k)
		}
	}
}

func TestStubService_Getters(t *testing.T) {
	s := NewStubService()
	ctx := context.Background()

	if s.CurrentPOI() != nil {
		t.Error("CurrentPOI should be nil")
	}
	if s.CurrentTitle() != "" {
		t.Error("CurrentTitle should be empty")
	}
	if s.ReplayLast(ctx) {
		t.Error("ReplayLast should return false")
	}
}
