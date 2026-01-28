package audio

import (
	"fmt"
	"testing"

	"phileasgo/pkg/config"
)

func TestNew(t *testing.T) {
	m := New(&config.NarratorConfig{})
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.Volume() != 1.0 {
		t.Errorf("Expected default volume 1.0, got %f", m.Volume())
	}
}

func TestManager_StateAccessors(t *testing.T) {
	m := New(&config.NarratorConfig{})

	tests := []struct {
		name   string
		action func(*Manager)
		check  func(*Manager) error
	}{
		{
			name:   "Default State",
			action: func(m *Manager) {},
			check: func(m *Manager) error {
				if m.Volume() != 1.0 {
					return errFmt("expected volume 1.0, got %f", m.Volume())
				}
				if m.IsUserPaused() {
					return errFmt("expected user pause false")
				}
				if m.IsPlaying() {
					return errFmt("expected IsPlaying false")
				}
				if m.Remaining() != 0 {
					return errFmt("expected Remaining 0")
				}
				return nil
			},
		},
		{
			name: "Volume Control",
			action: func(m *Manager) {
				m.SetVolume(0.5)
			},
			check: func(m *Manager) error {
				if m.Volume() != 0.5 {
					return errFmt("expected volume 0.5, got %f", m.Volume())
				}
				return nil
			},
		},
		{
			name: "Volume Clamping Low",
			action: func(m *Manager) {
				m.SetVolume(-0.5)
			},
			check: func(m *Manager) error {
				if m.Volume() != 0 {
					return errFmt("expected volume 0, got %f", m.Volume())
				}
				return nil
			},
		},
		{
			name: "Volume Clamping High",
			action: func(m *Manager) {
				m.SetVolume(1.5)
			},
			check: func(m *Manager) error {
				if m.Volume() != 1.0 {
					return errFmt("expected volume 1.0, got %f", m.Volume())
				}
				return nil
			},
		},
		{
			name: "User Pause Toggle",
			action: func(m *Manager) {
				m.SetUserPaused(true)
			},
			check: func(m *Manager) error {
				if !m.IsUserPaused() {
					return errFmt("expected user pause true")
				}
				return nil
			},
		},
		{
			name: "User Pause Reset",
			action: func(m *Manager) {
				m.SetUserPaused(true)
				m.ResetUserPause()
			},
			check: func(m *Manager) error {
				if m.IsUserPaused() {
					return errFmt("expected user pause false")
				}
				return nil
			},
		},
		{
			name:   "Last Narration Empty",
			action: func(m *Manager) {},
			check: func(m *Manager) error {
				if m.LastNarrationFile() != "" {
					return errFmt("expected empty last file")
				}
				if m.ReplayLastNarration(nil) {
					return errFmt("expected ReplayLastNarration false")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset manager for each test ideally, or share?
			// Since actions mutate, we should reset.
			// Re-create manager or reset fields.
			m.mu.Lock()
			m.volume = 1.0
			m.userPaused = false
			m.lastNarrationFile = ""
			m.mu.Unlock()

			tt.action(m)
			if err := tt.check(m); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestGetVoiceByID(t *testing.T) {
	tests := []struct {
		name         string
		inputID      string
		expectID     string
		expectGender string
	}{
		{
			name:         "Valid Voice",
			inputID:      "charon",
			expectID:     "charon",
			expectGender: "Male",
		},
		{
			name:         "Invalid Voice Fallback",
			inputID:      "nonexistent",
			expectID:     "aoede",
			expectGender: "Female", // Aoede is female
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := GetVoiceByID(tt.inputID)
			if v.ID != tt.expectID {
				t.Errorf("Expected ID %s, got %s", tt.expectID, v.ID)
			}
			if tt.expectGender != "" && v.Gender != tt.expectGender {
				t.Errorf("Expected gender %s, got %s", tt.expectGender, v.Gender)
			}
		})
	}
}

// Helper for concise error returning
type strErr string

func (e strErr) Error() string { return string(e) }
func errFmt(format string, a ...interface{}) error {
	return strErr(fmt.Sprintf(format, a...))
}
