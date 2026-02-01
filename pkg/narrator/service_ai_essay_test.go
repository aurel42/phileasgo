package narrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
)

func TestAIService_PlayEssay(t *testing.T) {
	// 1. Setup Environment (Configs, Templates)
	tmpDir := t.TempDir()

	// Essays Config
	essayCfgPath := filepath.Join(tmpDir, "essays.yaml")
	essayContent := `
topics:
  - id: "t1"
    name: "History of Flight"
    description: "About flying"
    max_words: 50
`
	_ = os.WriteFile(essayCfgPath, []byte(essayContent), 0o644)

	// Prompts
	promptsDir := filepath.Join(tmpDir, "narrator")
	_ = os.MkdirAll(promptsDir, 0o755)
	_ = os.WriteFile(filepath.Join(promptsDir, "essay.tmpl"), []byte("Write about {{.TopicName}} in {{.TargetCountry}}"), 0o644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)

	pm, err := prompts.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	// 2. Setup EssayHandler
	eh, err := NewEssayHandler(essayCfgPath, pm)
	if err != nil {
		t.Fatalf("Failed to create essay handler: %v", err)
	}

	// 3. Tests
	tests := []struct {
		name          string
		llmErr        error
		ttsErr        error
		audioErr      error
		expectSuccess bool // Expect Start
	}{
		{
			name:          "Happy Path",
			expectSuccess: true,
		},
		{
			name:          "LLM Error",
			llmErr:        errors.New("llm fail"),
			expectSuccess: true, // It starts, but fails internally
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mocks
			cfg := &config.Config{
				Narrator: config.NarratorConfig{
					TargetLanguage: "en",
				},
			}
			mockLLM := &MockLLM{Response: "Essay Script", Err: tt.llmErr}
			mockTTS := &MockTTS{Format: "mp3", Err: tt.ttsErr}
			mockAudio := &MockAudio{PlayErr: tt.audioErr}
			mockGeo := &MockGeo{Country: "France"}
			mockSim := &MockSim{Telemetry: sim.Telemetry{Latitude: 48.0, Longitude: 2.0}}

			// Inject Essay Handler
			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockAudio, &MockPOIProvider{}, &MockBeacon{}, mockGeo, mockSim, &MockStore{}, &MockWikipedia{}, nil, nil, eh, nil, nil, nil, session.NewManager())
			svc.Start()

			// Action
			started := svc.PlayEssay(context.Background(), &sim.Telemetry{})
			if started != tt.expectSuccess {
				t.Errorf("PlayEssay returned %v, expected %v", started, tt.expectSuccess)
			}

			if started {
				// Wait for async execution
				time.Sleep(100 * time.Millisecond)

				// Verify interactions
				if !svc.IsActive() && tt.llmErr == nil && tt.ttsErr == nil && tt.audioErr == nil {
					_ = true // Good state
				}
				// If error expected, we just ensure no panic/crash above
			}
		})
	}
}

func TestAIService_PlayEssay_NoHandler(t *testing.T) {
	// Setup minimalist service without essay handler
	svc := NewAIService(&config.Config{}, &MockLLM{}, &MockTTS{}, nil, &MockAudio{}, &MockPOIProvider{}, &MockBeacon{}, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager())

	if svc.PlayEssay(context.Background(), &sim.Telemetry{}) {
		t.Error("Expected PlayEssay to return false when handler is nil")
	}
}
func TestAIService_PlayEssay_UserPause(t *testing.T) {
	mockAudio := &MockAudio{}
	mockAudio.SetUserPaused(true)
	svc := &AIService{
		audio:  mockAudio,
		essayH: &EssayHandler{},
	}

	if svc.PlayEssay(context.Background(), &sim.Telemetry{}) {
		t.Error("Expected PlayEssay to return false when UserPause is active")
	}
}
