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
	cfg := config.NewProvider(&config.Config{}, nil) // Define cfg here
	svc := NewAIService(cfg, &MockLLM{}, &MockTTS{}, pm, &MockPOIProvider{}, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager(nil), nil, nil)

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
