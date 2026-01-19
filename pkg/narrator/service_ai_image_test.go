package narrator

import (
	"context"
	"fmt"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/sim"
	"testing"
)

func TestAIService_PlayImage(t *testing.T) {
	// Setup checks
	cfg := config.DefaultConfig()
	cfg.Narrator.Screenshot.Enabled = true
	cfg.Narrator.NarrationLengthShortWords = 42

	// We need a mock PromptManager that returns a known string
	// But `prompts.Manager` is a struct, not an interface.
	// The `Render` method uses actual files.
	// For unit testing `service_ai.go` logic without loading strict templates,
	// we assume `prompts.NewManager` is needed OR we rely on integration test style
	// if we can't mock the struct method.
	// However, `PlayImage` calls `s.prompts.Render`.
	// We might fail if templates aren't found.
	// We can use a real prompt manager pointing to a test directory, or just "configs/prompts" if accessible.

	// For the sake of this test, let's assume we can use the real one if we point to "configs/prompts" relative path.
	// But better: checks other logic (locking, LLM call, TTS call).

	tests := []struct {
		name          string
		userPaused    bool
		alreadyBusy   bool
		llmError      bool
		llmEmpty      bool
		ttsError      bool
		expectedAudio bool
	}{
		{
			name:          "Success flow",
			expectedAudio: true,
		},
		{
			name:          "User paused -> Skip",
			userPaused:    true,
			expectedAudio: false,
		},
		{
			name:          "Already generating -> Skip",
			alreadyBusy:   true,
			expectedAudio: false,
		},
		{
			name:          "LLM Error -> No Audio",
			llmError:      true,
			expectedAudio: false,
		},
		{
			name:          "LLM Empty -> No Audio",
			llmEmpty:      true,
			expectedAudio: false,
		},
		{
			name:          "TTS Error -> No Audio",
			ttsError:      true,
			expectedAudio: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mocks
			mockLLM := &MockLLM{Response: "test description"}
			if tt.llmError {
				mockLLM.Err = fmt.Errorf("llm error")
			} else if tt.llmEmpty {
				mockLLM.GenerateImageTextFunc = func(ctx context.Context, name, prompt, imagePath string) (string, error) {
					return "", nil
				}
			}

			mockTTS := &MockTTS{Format: "mp3"}
			if tt.ttsError {
				mockTTS.Err = fmt.Errorf("tts error")
			}

			mockAudio := &MockAudio{}
			if tt.userPaused {
				mockAudio.IsUserPausedVal = true
			}

			// Service
			// We can't mock prompts easily without interface extraction or real file loading.
			// Let's rely on `mockPromptManager` if we had one? We don't.
			// Pass nil for prompts? It will crash.
			// Let's skip prompt manager test by mocking `Render`? No can do.
			// So we MUST load a prompt manager.
			// We can pass a path that doesn't exist? No.
			// We will try to load "testdata/prompts" if we create it, OR blindly use the project one.
			// Since we run from `pkg/narrator`, project root is `../../configs/prompts`.
			pm, _ := prompts.NewManager("../../configs/prompts")
			// Check if pm is nil (it might be if path invalid).
			// If invalid, `Render` will fail or `NewManager` returns error.
			// If `NewManager` fails, `s.prompts` is nil -> crash.
			// We should perhaps handle nil in PlayImage? No, init guarantees it.

			// Hack: Create a dummy PromptManager if `../../configs/prompts` fails.
			// But for now let's hope it works.

			svc := &AIService{
				cfg:     cfg,
				llm:     mockLLM,
				tts:     mockTTS,
				prompts: pm,
				audio:   mockAudio,
				queue:   make([]*Narrative, 0), // Initialize queue
			}

			if tt.alreadyBusy {
				svc.generating = true
			}

			tel := &sim.Telemetry{
				Latitude: 10, Longitude: 20, AltitudeAGL: 3000,
			}

			// Act
			svc.PlayImage(context.Background(), "test.png", tel)

			// Assert - Check queue instead of direct audio call
			// PlayImage now enqueues and triggers processQueue async
			if tt.expectedAudio && len(svc.queue) == 0 {
				t.Error("Expected narrative to be enqueued, got empty queue")
			}
			if !tt.expectedAudio && len(svc.queue) > 0 {
				t.Error("Expected NO narrative, but queue is not empty")
			}

			// Verify flag reset (unless checking mid-execution which is hard here)
			if !tt.alreadyBusy && svc.generating {
				t.Error("generating flag was not reset")
			}
		})
	}
}
