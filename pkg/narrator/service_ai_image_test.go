package narrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/narrator/playback"
)

func TestAIService_PlayImage(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "narrator_test")
	defer os.RemoveAll(tempDir)
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "screenshot.tmpl"), []byte("Screenshot of {{.City}}"), 0o644)

	mockLLM := &MockLLM{Response: "Beautiful view!"}
	mockTTS := &MockTTS{}
	mockAudio := &MockAudio{}
	mockSim := &MockSim{}
	mockGeo := &MockGeo{City: "Paris"}
	mockStore := &MockStore{}

	pm, _ := prompts.NewManager(tempDir)

	svc := &AIService{
		cfg: &config.Config{
			Narrator: config.NarratorConfig{
				SummaryMaxWords:           500,
				NarrationLengthShortWords: 50,
				NarrationLengthLongWords:  200,
			},
		},
		llm:       mockLLM,
		tts:       mockTTS,
		audio:     mockAudio,
		sim:       mockSim,
		geoSvc:    mockGeo,
		st:        mockStore,
		prompts:   pm,
		playbackQ: playback.NewManager(),
		genQ:      generation.NewManager(),
	}

	// 1. Play Image
	imagePath := filepath.Join(tempDir, "test.jpg")
	_ = os.WriteFile(imagePath, []byte("fake image data"), 0o644)

	svc.PlayImage(context.Background(), imagePath, nil)

	// Wait for async gen
	time.Sleep(500 * time.Millisecond)

	if mockLLM.GenerateTextCalls == 0 && mockLLM.GenerateImageTextCalls == 0 {
		t.Errorf("expected LLM calls, got none")
	}

	if mockAudio.PlayCalls != 1 {
		t.Errorf("expected 1 audio play call, got %d", mockAudio.PlayCalls)
	}
}

func TestAIService_PlayImage_RenderError(t *testing.T) {
	tempDir := t.TempDir()
	// No templates provided, Render will fail

	mockLLM := &MockLLM{}
	mockTTS := &MockTTS{}
	mockAudio := &MockAudio{}
	mockSim := &MockSim{}
	mockGeo := &MockGeo{City: "Paris"}
	mockStore := &MockStore{}

	pm, _ := prompts.NewManager(tempDir)

	svc := &AIService{
		cfg:       &config.Config{},
		llm:       mockLLM,
		tts:       mockTTS,
		audio:     mockAudio,
		sim:       mockSim,
		geoSvc:    mockGeo,
		st:        mockStore,
		prompts:   pm,
		playbackQ: playback.NewManager(),
		genQ:      generation.NewManager(),
	}

	imagePath := filepath.Join(tempDir, "test.jpg")
	_ = os.WriteFile(imagePath, []byte("fake image data"), 0o644)

	// Action
	svc.PlayImage(context.Background(), imagePath, nil)

	// Verify no async gen started due to render error
	time.Sleep(100 * time.Millisecond)
	if mockLLM.GenerateImageTextCalls != 0 {
		t.Errorf("expected no LLM calls on render error")
	}
}
