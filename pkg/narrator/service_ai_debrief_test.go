package narrator

import (
	"context"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/sim"
)

func TestPlayDebrief(t *testing.T) {
	// Setup
	cfg := config.DefaultConfig()
	cfg.Narrator.Debrief.Enabled = true
	cfg.Narrator.NarrationLengthLongWords = 100

	// We need a real prompt manager or a mock?
	// Real one requires file system. Let's try to minimal setup or just rely on error flow if template missing.
	// Actually, service_ai_debrief.go calls prompts.Render("narrator/debrief.tmpl", ...)
	// If files are missing, it fails.
	// Let's rely on the fact that "make test" runs in project root, so prompts are available.
	pm, err := prompts.NewManager("../../configs/prompts")
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	mockLLM := &MockLLM{
		GenerateTextFunc: func(ctx context.Context, intent, prompt string) (string, error) {
			if intent == "summary" {
				return "This is a sufficiently long summary that should pass the length check. It talks about interesting places and provides a good overview of the journey so far.", nil
			}
			return "Ladies and gentlemen, welcome to our destination.", nil
		},
	}

	// MockAudio in mocks_dev_test.go has Play(filepath string, startPaused bool) error
	// But AIService expects AudioService interface: Play(ctx context.Context, text string) error
	// Wait, checking mocks_dev_test.go content again...
	// type MockAudio struct { ... }
	// func (m *MockAudio) Play(filepath string, startPaused bool) error { ... }
	//
	// Checking pkg/narrator/service_ai.go constructor:
	// func NewAIService(..., audio AudioService, ...)
	// interface AudioService is in service.go? No, it's defined where it's used or imported.
	// In service_ai.go: type AudioService interface { Play(ctx context.Context, text string) error ... } ?
	// NO, the snippet for mocks_dev_test.go showed MockAudio implementing Play(filepath string, startPaused bool)
	// BUT AIService calls s.audio.Play(ctx, text) ??
	// Let's check service_ai.go imports and interface definition.
	// If AIService uses a different interface than what MockAudio implements, we have a problem.
	// Actually, AIService uses `audio.Play(ctx, text)` in `PlayNarrative` usually.
	// Let's check `service_ai_common.go` line 25 `synthesizeAudio`.
	//
	// If the existing MockAudio in mocks_dev_test.go is for the "AudioPlayer" (low level),
	// and AIService expects a high level "TTS+Player" or just "Player"?
	//
	// Let's look at `service_ai.go` again to see what `NewAIService` expects.
	// It expects `AudioService`.
	//
	// In `mocks_dev_test.go`, `MockAudio` seems to Mock the `AudioPlayer` interface used by `narrator/service.go`?
	//
	// I will define a LOCAL mock specific for AIService requirements if the global one doesn't match.
	// Global `MockAudio` has `Play(filepath string...)`.
	// AIService likely needs `Play(ctx, text)` OR `Play(filepath)` depending on how it's wired.
	//
	// In `service_ai_debrief.go`: `s.PlayNarrative(..., narrative)` -> `PlayNarrative` calls `synthesizeAudio` -> `s.tts.Synthesize` -> then `s.audio.Play(path)`.
	//
	// So `AIService` needs an `AudioService` that has `Play(filepath, paused)`.
	// The `MockAudio` in `mocks_dev_test.go` DOES implement that.
	//
	// So I can use `MockAudio`.

	mockAudio := &MockAudio{}

	// We need a MockTTS as well for PlayNarrative -> synthesizeAudio
	mockTTS := &MockTTS{}

	svc := NewAIService(
		cfg,
		mockLLM,
		mockTTS,
		pm,
		mockAudio,
		nil,          // poiMgr
		nil,          // beaconSvc
		nil,          // geoSvc
		&MockSim{},   // simClient
		&MockStore{}, // store
		nil,          // wikipedia
		nil,          // langRes
		nil,          // essayH
		nil,          // interests
		nil,          // avoid
		nil,          // tracker
	)

	// Inject a valid summary
	svc.updateTripSummary(context.Background(), "Previous POI", "This was a great trip seeing London and Paris.")

	// Test 1: Happy Path
	tel := &sim.Telemetry{}
	if !svc.PlayDebrief(context.Background(), tel) {
		t.Error("PlayDebrief returned false, expected true")
	}

	// Wait for async generation to finish (PlayDebrief launches goroutine)
	// We can't easily sync without modifying service.
	// But PlayDebrief returns true immediately if it starts.
	// To test specific logic inside, we rely on coverage of the synchronous parts.
}

func TestPlayDebrief_Disabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Debrief.Enabled = false
	// We can pass nil for everything if we expect it to return false early
	svc := NewAIService(cfg, nil, nil, nil, nil, nil, nil, nil, &MockSim{}, &MockStore{}, nil, nil, nil, nil, nil, nil)

	if svc.PlayDebrief(context.Background(), &sim.Telemetry{}) {
		t.Error("PlayDebrief returned true when disabled")
	}
}

func TestPlayDebrief_Busy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Debrief.Enabled = true
	svc := NewAIService(cfg, nil, nil, nil, nil, nil, nil, nil, &MockSim{}, &MockStore{}, nil, nil, nil, nil, nil, nil)

	// Set active
	svc.active = true

	if svc.PlayDebrief(context.Background(), &sim.Telemetry{}) {
		t.Error("PlayDebrief returned true when busy")
	}
}

func TestPlayDebrief_ShortSummary(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Debrief.Enabled = true
	svc := NewAIService(cfg, nil, nil, nil, nil, nil, nil, nil, &MockSim{}, &MockStore{}, nil, nil, nil, nil, nil, nil)

	// Short summary
	// updateTripSummary writes helper prompt, let's just cheat and set string directly if possible?
	// tripSummary field is private. We must use updateTripSummary but we need to mock LLM then because updateTripSummary calls LLM!
	// Wait, updateTripSummary calls LLM to generate summary.

	// Better way: use reflection or just rely on the fact that if LLM fails, summary remains empty?
	// Actually, service_ai.go:127 initializes `tripSummary` to "".
	// If we want to test "Short Summary", empty string is short enough.

	// So we don't call updateTripSummary at all.

	if svc.PlayDebrief(context.Background(), &sim.Telemetry{}) {
		t.Error("PlayDebrief returned true when summary is too short")
	}
}
