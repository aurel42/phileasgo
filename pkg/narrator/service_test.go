package narrator

import (
	"context"
	"testing"
)

func TestNewStubService(t *testing.T) {
	s := NewStubService()
	if s == nil {
		t.Fatal("NewStubService returned nil")
	}
}

func TestStubService_StartStop(t *testing.T) {
	s := NewStubService()

	s.Start()
	if !s.running {
		t.Error("Expected running to be true after Start")
	}

	s.Stop()
	if s.running {
		t.Error("Expected running to be false after Stop")
	}
}

func TestStubService_IsActive(t *testing.T) {
	s := NewStubService()

	// Initially not active
	if s.IsActive() {
		t.Error("Expected IsActive to be false initially")
	}
}

func TestStubService_NarratedCount(t *testing.T) {
	s := NewStubService()

	// Initially zero
	if s.NarratedCount() != 0 {
		t.Errorf("Expected NarratedCount 0, got %d", s.NarratedCount())
	}

	// Play manual
	s.PlayPOI(context.Background(), "Q1", true, nil)
	if s.NarratedCount() != 1 {
		t.Errorf("expected 1 narrated POI, got %d", s.NarratedCount())
	}

	// Play another
	s.PlayPOI(context.Background(), "Q2", false, nil)
	if s.NarratedCount() != 2 {
		t.Errorf("expected 2 narrated POI, got %d", s.NarratedCount())
	}

	// Replay same (stub counts unique? No, just map keys)
	s.PlayPOI(context.Background(), "Q1", true, nil)
	if s.NarratedCount() != 2 {
		t.Errorf("expected 2 narrated POI, got %d", s.NarratedCount())
	}
}

func TestStubService_Stats(t *testing.T) {
	s := NewStubService()

	stats := s.Stats()
	if stats == nil {
		t.Fatal("Stats returned nil")
	}

	// Check expected keys exist
	expectedKeys := []string{"gemini_text_success", "gemini_tts_success"}
	for _, key := range expectedKeys {
		if _, ok := stats[key]; !ok {
			t.Errorf("Expected key '%s' in stats", key)
		}
	}
}

func TestStubService_SkipCooldown(t *testing.T) {
	s := NewStubService()

	// Should not panic
	s.SkipCooldown()
}
