package narrator

import (
	"context"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/session"
)

// TestScreenshotCoordinatesPersistence verifies that Lat/Lon are correctly propagated
// from GenerationRequest to Narrative.
func TestScreenshotCoordinatesPersistence(t *testing.T) {
	// Setup Prompts (using temp dir to avoid loading errors/nil panic)
	pm, _ := prompts.NewManager(t.TempDir())

	// Setup minimalist service (mocks only where needed)
	svc := NewAIService(&config.Config{},
		&MockLLM{Response: "Script"},
		&MockTTS{Format: "mp3"},
		pm,
		&MockAudio{},
		&MockPOIProvider{},
		&MockBeacon{},
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager())

	// User Aircraft Location
	userLat := 45.0
	userLon := 90.0

	req := GenerationRequest{
		Type:   model.NarrativeTypeScreenshot,
		Title:  "Visual Analysis",
		Prompt: "Describe this image",
		// Simulate data populate by queue
		Lat:      userLat,
		Lon:      userLon,
		MaxWords: 50,
	}

	// We need prompts for GenerateNarrative to not crash if it renders?
	// Actually GenerateNarrative calls generating functions.
	// But we can test `constructNarrative` indirectly via GenerateNarrative (which calls generateInitialScript -> prompts).
	// TO AVOID COMPLEX MOCKS regarding prompts:
	// We can't access `constructNarrative` (private).
	// We MUST use GenerateNarrative.
	// So we need a MockPromptManager or just handle the error?
	// GenerateNarrative calls `handleGenerationState` then `generateInitialScript`.
	// `generateInitialScript` calls `llm.GenerateImageText` for screenshots.
	// Our MockLLM supports that.
	// So prompts are NOT used for script generation in screenshot mode?
	// `generateInitialScript`:
	// if req.ImagePath != "" { ... s.llm.GenerateImageText ... return }
	// So prompts are bypassed for screenshots! Perfect.

	req.ImagePath = "test.jpg"

	ctx := context.Background()
	narrative, err := svc.GenerateNarrative(ctx, &req)
	if err != nil {
		t.Fatalf("GenerateNarrative failed: %v", err)
	}

	if narrative.Lat != userLat {
		t.Errorf("Expected Lat %f, got %f", userLat, narrative.Lat)
	}
	if narrative.Lon != userLon {
		t.Errorf("Expected Lon %f, got %f", userLon, narrative.Lon)
	}
}

// TestBorderBeaconExemption verifies that playing a Border narrative
// does NOT clear the existing beacons.
func TestBorderBeaconExemption(t *testing.T) {
	pm, _ := prompts.NewManager(t.TempDir()) // Fix: Valid PM

	mockBeacon := &MockBeacon{}
	svc := NewAIService(&config.Config{},
		&MockLLM{},
		&MockTTS{},
		pm,
		&MockAudio{},
		&MockPOIProvider{},
		mockBeacon, // The key mock
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager())

	// 1. Simulate active beacon
	// We can't easily "simulate" it in internal state without calling SetTarget,
	// but we just check if Clear() is called.
	// MockBeacon tracks calls.

	n := &model.Narrative{
		Type:      model.NarrativeTypeBorder,
		Title:     "Border Crossing",
		Script:    "Welcome to Italy",
		AudioPath: "test_audio",
		Format:    "mp3",
	}

	err := svc.PlayNarrative(context.Background(), n)
	if err != nil {
		// PlayNarrative calls s.audio.Play. MockAudio default returns nil.
		t.Fatalf("PlayNarrative failed: %v", err)
	}

	if mockBeacon.Cleared {
		t.Error("Beacon service should NOT be cleared for Border narratives")
	}

	// 2. Verify behavior for other types (e.g. Screenshot)
	mockBeacon.Cleared = false // Reset

	// Re-create service to reset "active" state immediately
	svc = NewAIService(&config.Config{},
		&MockLLM{},
		&MockTTS{},
		pm,
		&MockAudio{},
		&MockPOIProvider{},
		mockBeacon,
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager())

	nScreen := &model.Narrative{
		Type:      model.NarrativeTypeScreenshot,
		Title:     "Screenshot",
		Script:    "Wow",
		AudioPath: "test_audio",
		Format:    "mp3",
	}

	err = svc.PlayNarrative(context.Background(), nScreen)
	if err != nil {
		t.Fatalf("PlayNarrative failed: %v", err)
	}

	if !mockBeacon.Cleared {
		t.Error("Beacon service SHOULD be cleared for Screenshot narratives")
	}
}
