package narrator

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"strings"
	"testing"

	"phileasgo/pkg/generation"
)

func TestAIService_RescueScript(t *testing.T) {
	mockLLM := &MockLLM{
		Response: "this is a shorter script",
	}

	tmpDir := t.TempDir()
	pm, _ := prompts.NewManager(tmpDir)

	svc := &AIService{
		llm:     mockLLM,
		prompts: pm,
		cfg: config.NewProvider(&config.Config{
			Narrator: config.NarratorConfig{
				TargetLanguage: "en-US",
			},
		}, nil),
		sessionMgr: session.NewManager(nil),
	}

	original := strings.Repeat("long ", 500)
	_, err := svc.rescueScript(context.Background(), original, 50)

	if err == nil {
		t.Error("expected error due to missing template")
	}
}

func TestAIService_PlayPOI_Constraints(t *testing.T) {
	mockPOI := &model.POI{WikidataID: "Q123", NameEn: "Test POI"}
	mockPOIProv := &MockPOIProvider{
		GetPOIFunc: func(ctx context.Context, qid string) (*model.POI, error) {
			return mockPOI, nil
		},
	}
	mockSim := &MockSim{}

	svc := &AIService{
		poiMgr:     mockPOIProv,
		sim:        mockSim,
		st:         &MockStore{},
		genQ:       generation.NewManager(),
		sessionMgr: session.NewManager(nil),
	}

	// 1. Manual PlayPOI - should enqueue generation
	svc.PlayPOI(context.Background(), "Q123", true, false, &sim.Telemetry{}, "")
	if svc.genQ.Count() != 1 {
		t.Errorf("Expected 1 manual generation job, got %d", svc.genQ.Count())
	}

	// 2. Automated PlayPOI - should skip because we haven't released the previous generation slot
	// Wait, claimGeneration is about 'generating' bool.
	svc.mu.Lock()
	svc.generating = true
	svc.mu.Unlock()

	svc.PlayPOI(context.Background(), "Q456", false, false, &sim.Telemetry{}, "")
	// Should skip because generating=true

	svc.PlayPOI(context.Background(), "Q789", true, false, &sim.Telemetry{}, "")
	if svc.genQ.Count() != 2 {
		t.Errorf("Expected manual PlayPOI to ignore busy generating state, but queue count is: %d", svc.genQ.Count())
	}
}
