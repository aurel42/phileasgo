package narrator

import (
	"context"
	"os"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"strings"
	"testing"

	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/narrator/playback"
)

func TestAIService_RescueScript(t *testing.T) {
	mockLLM := &MockLLM{
		Response: "this is a shorter script",
	}

	tmpDir, _ := os.MkdirTemp("", "prompts-test")
	defer os.RemoveAll(tmpDir)

	pm, _ := prompts.NewManager(tmpDir)

	svc := &AIService{
		llm:     mockLLM,
		prompts: pm,
		cfg: &config.Config{
			Narrator: config.NarratorConfig{
				TargetLanguage: "en-US",
			},
		},
		sessionMgr: session.NewManager(),
	}

	// Pre-create the template in the manager's root
	// Since Render looks at m.root, we can manually parse it
	pm.Render("context/rescue_script.tmpl", nil) // This will fail but ensure root exists

	original := strings.Repeat("long ", 500)
	// We expect Render to fail because template doesn't exist,
	// but we want to see how rescueScript handles it.
	// Actually rescueScript signature is (string, error)
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
		playbackQ:  playback.NewManager(),
		genQ:       generation.NewManager(),
		sessionMgr: session.NewManager(),
	}

	// 1. Manual PlayPOI - should enqueue generation
	svc.PlayPOI(context.Background(), "Q123", true, false, &sim.Telemetry{}, "")
	if svc.genQ.Count() != 1 {
		t.Errorf("Expected 1 manual generation job, got %d", svc.genQ.Count())
	}

	// 2. Automated PlayPOI - skip because busy (pending generation)
	svc.PlayPOI(context.Background(), "Q456", false, false, &sim.Telemetry{}, "")
	// Should log "Skipping auto-play (priority queue not empty)" and return

	// 3. UserPause - should skip automated but proceed with manual
	mockAudio := &MockAudio{}
	mockAudio.SetUserPaused(true)
	svc.audio = mockAudio
	svc.PlayPOI(context.Background(), "Q789", false, false, &sim.Telemetry{}, "")
	if svc.genQ.Count() != 1 {
		t.Errorf("Expected automated PlayPOI to be skipped when paused, but queue count changed: %d", svc.genQ.Count())
	}

	svc.PlayPOI(context.Background(), "Q789", true, false, &sim.Telemetry{}, "")
	if svc.genQ.Count() != 2 {
		t.Errorf("Expected manual PlayPOI to ignore pause, but queue count is: %d", svc.genQ.Count())
	}
}
