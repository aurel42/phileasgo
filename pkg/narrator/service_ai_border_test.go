package narrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/sim"
)

func TestAIService_PlayBorder(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "border.tmpl"), []byte("From {{.From}} to {{.To}} Summary: {{.TripSummary}} TTS: {{.TTSInstructions}}"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	_ = os.MkdirAll(filepath.Join(tempDir, "tts"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "tts", "edge-tts.tmpl"), []byte("TTS Info"), 0o644)
	pm, _ := prompts.NewManager(tempDir)

	cfg := &config.Config{}
	mockLLM := &MockLLM{Response: "Border Script"}
	mockTTS := &MockTTS{Format: "mp3"}
	mockAudio := &MockAudio{}
	mockPOI := &MockPOIProvider{
		GetPOIFunc: func(ctx context.Context, qid string) (*model.POI, error) {
			return &model.POI{WikidataID: qid, NameEn: "Test"}, nil
		},
	}
	mockGeo := &MockGeo{Country: "US", City: "MockCity"}
	mockSim := &MockSim{}
	mockStore := &MockStore{}
	mockWiki := &MockWikipedia{}
	mockBeacon := &MockBeacon{}

	var capturedPrompt string
	mockLLM.GenerateTextFunc = func(ctx context.Context, name, prompt string) (string, error) {
		capturedPrompt = prompt
		return "Border Script", nil
	}

	svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil, nil, nil)
	svc.mu.Lock()
	svc.tripSummary = "Recent history"
	svc.mu.Unlock()
	svc.Start()

	// 1. Success Case
	tel := &sim.Telemetry{Latitude: 10, Longitude: 20}
	ok := svc.PlayBorder(context.Background(), "France", "Germany", tel)
	if !ok {
		t.Fatal("PlayBorder returned false, expected true")
	}

	// Trigger Generation
	svc.ProcessGenerationQueue(context.Background())
	time.Sleep(100 * time.Millisecond)

	if svc.NarratedCount() != 1 {
		t.Errorf("Expected 1 narrated border, got %d", svc.NarratedCount())
	}

	if !strings.Contains(capturedPrompt, "Summary: Recent history") {
		t.Errorf("Expected prompt to contain TripSummary, got: %s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "TTS: TTS Info") {
		t.Errorf("Expected prompt to contain TTSInstructions, got: %s", capturedPrompt)
	}

	// 2. Queue Constraint (Busy)
	// Simulate busy by filling playback queue
	svc.mu.Lock()
	svc.playbackQ.Clear()
	svc.playbackQ.Enqueue(&model.Narrative{Type: model.NarrativeTypeBorder, Manual: true}, false)
	svc.playbackQ.Enqueue(&model.Narrative{Type: model.NarrativeTypeBorder, Manual: true}, false)
	svc.playbackQ.Enqueue(&model.Narrative{Type: model.NarrativeTypeBorder, Manual: true}, false)
	svc.playbackQ.Enqueue(&model.Narrative{Type: model.NarrativeTypeBorder, Manual: true}, false)
	svc.playbackQ.Enqueue(&model.Narrative{Type: model.NarrativeTypeBorder, Manual: true}, false)
	svc.mu.Unlock()

	ok = svc.PlayBorder(context.Background(), "A", "B", tel)
	if ok {
		t.Error("PlayBorder should have failed due to queue limits")
	}

	// 3. Pending Generation
	svc.mu.Lock()
	svc.playbackQ.Clear()
	svc.genQ.Clear()
	svc.genQ.Enqueue(&generation.Job{Type: model.NarrativeTypePOI})
	svc.mu.Unlock()

	ok = svc.PlayBorder(context.Background(), "A", "B", tel)
	if ok {
		t.Error("PlayBorder should have failed due to pending generation")
	}
}
