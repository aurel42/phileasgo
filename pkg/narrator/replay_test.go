package narrator

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/session"
)

func TestOrchestrator_ReplayLast(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(o *Orchestrator, m *MockAudioService)
		wantActive     bool
		wantCurrentPOI bool
		wantReplay     bool
	}{
		{
			name: "Replay last POI success",
			setup: func(o *Orchestrator, m *MockAudioService) {
				o.lastPOI = &model.POI{WikidataID: "Q1", NameEn: "Test POI"}
				m.ShouldReplay = true
				m.IsBusyVal = true
			},
			wantActive:     true,
			wantCurrentPOI: true,
			wantReplay:     true,
		},
		{
			name: "Audio replay fails",
			setup: func(o *Orchestrator, m *MockAudioService) {
				o.lastPOI = &model.POI{WikidataID: "Q1"}
				m.ShouldReplay = false
			},
			wantActive:     false,
			wantCurrentPOI: false,
			wantReplay:     false,
		},
		{
			name: "Replay with no state (fallback)",
			setup: func(o *Orchestrator, m *MockAudioService) {
				// No last POI
				m.ShouldReplay = true
			},
			wantActive:     false, // Should not activate UI if no POI/Image
			wantCurrentPOI: false,
			wantReplay:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Mocks
			mockAudio := &MockAudioService{}
			pbQ := playback.NewManager()
			sess := session.NewManager(nil)

			// Orchestrator needs a Generator, we can use nil or a dummy if it's not called
			o := NewOrchestrator(nil, mockAudio, pbQ, sess, nil, nil, nil)

			// Run Setup
			tt.setup(o, mockAudio)

			// Execute
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			got := o.ReplayLast(ctx)

			// Verify Return
			if got != tt.wantReplay {
				t.Errorf("ReplayLast() = %v, want %v", got, tt.wantReplay)
			}

			// Verify State
			o.mu.Lock()
			if o.active != tt.wantActive {
				t.Errorf("active = %v, want %v", o.active, tt.wantActive)
			}

			if tt.wantCurrentPOI && (o.currentPOI == nil || o.currentPOI != o.lastPOI) {
				t.Errorf("currentPOI not restored correctly")
			}
			if !tt.wantCurrentPOI && o.currentPOI != nil {
				t.Errorf("currentPOI should be nil")
			}
			o.mu.Unlock()
		})
	}
}

// MockAudioService for testing - minimal implementation
type MockAudioService struct {
	ShouldReplay bool
	IsBusyVal    bool
}

func (m *MockAudioService) Play(path string, skipChecks bool, onComplete func()) error {
	if onComplete != nil {
		go onComplete()
	}
	return nil
}
func (m *MockAudioService) Stop()                     {}
func (m *MockAudioService) Shutdown()                 {}
func (m *MockAudioService) Pause()                    {}
func (m *MockAudioService) Resume()                   {}
func (m *MockAudioService) Skip()                     {}
func (m *MockAudioService) IsPlaying() bool           { return m.IsBusyVal }
func (m *MockAudioService) IsBusy() bool              { return m.IsBusyVal }
func (m *MockAudioService) IsPaused() bool            { return false }
func (m *MockAudioService) SetVolume(vol float64)     {}
func (m *MockAudioService) Volume() float64           { return 1.0 }
func (m *MockAudioService) SetUserPaused(paused bool) {}
func (m *MockAudioService) IsUserPaused() bool        { return false }
func (m *MockAudioService) ResetUserPause()           {}
func (m *MockAudioService) LastNarrationFile() string { return "" }
func (m *MockAudioService) ReplayLastNarration(onComplete func()) bool {
	if m.ShouldReplay && onComplete != nil {
		go onComplete()
	}
	return m.ShouldReplay
}
func (m *MockAudioService) Position() time.Duration { return 0 }
func (m *MockAudioService) Duration() time.Duration {
	return time.Second * 10
}

func (m *MockAudioService) Remaining() time.Duration {
	return 0
}

func (m *MockAudioService) AverageLatency() time.Duration {
	return 0
}
func (m *MockAudioService) Process() {}
