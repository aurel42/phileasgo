package audio

import (
	"testing"
)

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.Volume() != 1.0 {
		t.Errorf("Expected default volume 1.0, got %f", m.Volume())
	}
}

func TestManager_Volume(t *testing.T) {
	m := New()

	// Test setting valid volume
	m.SetVolume(0.5)
	if m.Volume() != 0.5 {
		t.Errorf("Expected volume 0.5, got %f", m.Volume())
	}

	// Test clamping to 0
	m.SetVolume(-0.5)
	if m.Volume() != 0 {
		t.Errorf("Expected volume 0, got %f", m.Volume())
	}

	// Test clamping to 1
	m.SetVolume(1.5)
	if m.Volume() != 1 {
		t.Errorf("Expected volume 1, got %f", m.Volume())
	}
}

func TestManager_UserPause(t *testing.T) {
	m := New()

	// Default should be false
	if m.IsUserPaused() {
		t.Error("Expected user pause to be false by default")
	}

	// Set user paused
	m.SetUserPaused(true)
	if !m.IsUserPaused() {
		t.Error("Expected user pause to be true")
	}

	// Reset user pause
	m.ResetUserPause()
	if m.IsUserPaused() {
		t.Error("Expected user pause to be false after reset")
	}
}

func TestManager_PlaybackState(t *testing.T) {
	m := New()

	// Initially not playing or paused
	if m.IsPlaying() {
		t.Error("Expected IsPlaying to be false initially")
	}
	if m.IsPaused() {
		t.Error("Expected IsPaused to be false initially")
	}
}

func TestManager_LastNarrationFile(t *testing.T) {
	m := New()

	// Initially empty
	if m.LastNarrationFile() != "" {
		t.Error("Expected LastNarrationFile to be empty initially")
	}

	// ReplayLastNarration should return false when no file
	if m.ReplayLastNarration() {
		t.Error("Expected ReplayLastNarration to return false when no file")
	}
}

func TestGetVoiceByID(t *testing.T) {
	// Test valid voice
	v := GetVoiceByID("charon")
	if v.ID != "charon" {
		t.Errorf("Expected voice ID 'charon', got '%s'", v.ID)
	}
	if v.Gender != "Male" {
		t.Errorf("Expected gender 'Male', got '%s'", v.Gender)
	}

	// Test invalid voice returns first
	v = GetVoiceByID("nonexistent")
	if v.ID != "aoede" {
		t.Errorf("Expected fallback voice ID 'aoede', got '%s'", v.ID)
	}
}
