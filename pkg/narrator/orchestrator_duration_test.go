package narrator

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/session"
	"testing"
	"time"
)

func TestOrchestrator_CurrentDuration(t *testing.T) {
	tests := []struct {
		name             string
		narrative        *model.Narrative
		expectedDuration time.Duration
	}{
		{
			name: "POI Narrative with duration",
			narrative: &model.Narrative{
				Type:      model.NarrativeTypePOI,
				Duration:  15 * time.Second,
				Title:     "Test POI",
				AudioPath: "test",
				Format:    "mp3",
			},
			expectedDuration: 15 * time.Second,
		},
		{
			name: "Debriefing Narrative with duration",
			narrative: &model.Narrative{
				Type:      model.NarrativeTypeDebriefing,
				Duration:  45 * time.Second,
				Title:     "Debriefing",
				AudioPath: "test",
				Format:    "mp3",
			},
			expectedDuration: 45 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGen := &MockAIService{}
			mockAudio := &MockAudio{PlaySync: false} // Ensure it's async
			pbQ := playback.NewManager()
			sess := session.NewManager(nil)
			o := NewOrchestrator(mockGen, mockAudio, pbQ, sess, nil, nil, nil)
			o.pacingDuration = 0 // Avoid sleep in finalize

			// 1. Initial duration should be 0
			if o.CurrentDuration() != 0 {
				t.Errorf("Initial duration expected 0, got %v", o.CurrentDuration())
			}

			// 2. Play narrative
			// We call it in a goroutine because PlayNarrative might block if we don't handle it
			// But o.PlayNarrative calls o.audio.Play which in our mock is async if PlaySync is false.
			err := o.PlayNarrative(context.Background(), tt.narrative)
			if err != nil {
				t.Fatalf("PlayNarrative failed: %v", err)
			}

			// 3. Check duration while "playing"
			if o.CurrentDuration() != tt.expectedDuration {
				t.Errorf("CurrentDuration() = %v, want %v", o.CurrentDuration(), tt.expectedDuration)
			}

			// 4. Wait for async finalize (MockAudio sleeps 10ms then calls onComplete)
			time.Sleep(50 * time.Millisecond)

			if o.CurrentDuration() != 0 {
				t.Errorf("Duration after finalize expected 0, got %v", o.CurrentDuration())
			}
		})
	}
}

func TestOrchestrator_SetPlaybackState_Defensive(t *testing.T) {
	o := &Orchestrator{}

	tests := []struct {
		name     string
		path     string
		format   string
		expected string
	}{
		{
			name:     "No extension",
			path:     "audio",
			format:   "mp3",
			expected: "audio.mp3",
		},
		{
			name:     "Existing extension",
			path:     "audio.mp3",
			format:   "mp3",
			expected: "audio.mp3",
		},
		{
			name:     "Different extension",
			path:     "audio.wav",
			format:   "mp3",
			expected: "audio.wav.mp3",
		},
		{
			name:     "Uppercase extension",
			path:     "AUDIO.MP3",
			format:   "mp3",
			expected: "AUDIO.MP3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &model.Narrative{
				AudioPath: tt.path,
				Format:    tt.format,
			}
			got := o.setPlaybackState(n)
			if got != tt.expected {
				t.Errorf("setPlaybackState() = %v, want %v", got, tt.expected)
			}
		})
	}
}
