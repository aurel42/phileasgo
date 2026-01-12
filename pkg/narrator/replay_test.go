package narrator

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/model"
)

func TestAIService_ReplayLast(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(s *AIService, m *MockAudioService)
		wantActive     bool
		wantCurrentPOI bool
		wantEssay      bool
		wantReplay     bool
	}{
		{
			name: "Replay last POI success",
			setup: func(s *AIService, m *MockAudioService) {
				s.lastPOI = &model.POI{WikidataID: "Q1", NameEn: "Test POI"}
				m.ShouldReplay = true
				m.IsBusyVal = true
			},
			wantActive:     true,
			wantCurrentPOI: true,
			wantEssay:      false,
			wantReplay:     true,
		},
		{
			name: "Replay last Essay success",
			setup: func(s *AIService, m *MockAudioService) {
				s.lastEssayTopic = &EssayTopic{Name: "History"}
				s.lastEssayTitle = "Great History"
				m.ShouldReplay = true
				m.IsBusyVal = true
			},
			wantActive:     true,
			wantCurrentPOI: false,
			wantEssay:      true,
			wantReplay:     true,
		},
		{
			name: "Audio replay fails",
			setup: func(s *AIService, m *MockAudioService) {
				s.lastPOI = &model.POI{WikidataID: "Q1"}
				m.ShouldReplay = false
			},
			wantActive:     false,
			wantCurrentPOI: false,
			wantEssay:      false,
			wantReplay:     false,
		},
		{
			name: "Replay with no state (fallback)",
			setup: func(s *AIService, m *MockAudioService) {
				// No last POI or Essay
				m.ShouldReplay = true
			},
			wantActive:     false, // Should not activate UI
			wantCurrentPOI: false,
			wantEssay:      false,
			wantReplay:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Mocks
			mockAudio := &MockAudioService{}
			s := &AIService{
				audio: mockAudio,
			}

			// Run Setup
			tt.setup(s, mockAudio)

			// Execute
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			got := s.ReplayLast(ctx)

			// Verify Return
			if got != tt.wantReplay {
				t.Errorf("ReplayLast() = %v, want %v", got, tt.wantReplay)
			}

			// Verify State
			s.mu.RLock()
			if s.active != tt.wantActive {
				t.Errorf("active = %v, want %v", s.active, tt.wantActive)
			}

			if tt.wantCurrentPOI && (s.currentPOI == nil || s.currentPOI != s.lastPOI) {
				t.Errorf("currentPOI not restored correctly")
			}
			if !tt.wantCurrentPOI && s.currentPOI != nil {
				t.Errorf("currentPOI should be nil")
			}

			if tt.wantEssay && (s.currentTopic == nil || s.currentTopic != s.lastEssayTopic) {
				t.Errorf("currentTopic not restored correctly")
			}
			if !tt.wantEssay && s.currentTopic != nil {
				t.Errorf("currentTopic should be nil")
			}
			s.mu.RUnlock()
		})
	}
}

// MockAudioService for testing - minimal implementation
type MockAudioService struct {
	ShouldReplay bool
	IsBusyVal    bool
}

func (m *MockAudioService) Play(path string, skipChecks bool) error { return nil }
func (m *MockAudioService) Stop()                                   {}
func (m *MockAudioService) Shutdown()                               {}
func (m *MockAudioService) Pause()                                  {}
func (m *MockAudioService) Resume()                                 {}
func (m *MockAudioService) Skip()                                   {}
func (m *MockAudioService) IsPlaying() bool                         { return m.IsBusyVal }
func (m *MockAudioService) IsBusy() bool                            { return m.IsBusyVal }
func (m *MockAudioService) IsPaused() bool                          { return false }
func (m *MockAudioService) SetVolume(vol float64)                   {}
func (m *MockAudioService) Volume() float64                         { return 1.0 }
func (m *MockAudioService) SetUserPaused(paused bool)               {}
func (m *MockAudioService) IsUserPaused() bool                      { return false }
func (m *MockAudioService) ResetUserPause()                         {}
func (m *MockAudioService) LastNarrationFile() string               { return "" }
func (m *MockAudioService) ReplayLastNarration() bool               { return m.ShouldReplay }
func (m *MockAudioService) Position() time.Duration                 { return 0 }
func (m *MockAudioService) Duration() time.Duration {
	return time.Second * 10
}

func (m *MockAudioService) Remaining() time.Duration {
	return 0
}
