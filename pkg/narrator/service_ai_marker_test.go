package narrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func TestAIService_NextPOIMarker(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	mockBeacon := &MockBeacon{}
	mockPOI := &MockPOIProvider{
		GetPOIFunc: func(_ context.Context, qid string) (*model.POI, error) {
			if qid == "QNext" {
				return &model.POI{WikidataID: "QNext", Lat: 20.0, Lon: 30.0}, nil
			}
			return &model.POI{WikidataID: qid, Lat: 10.0, Lon: 10.0}, nil
		},
	}

	mockAudio := &MockAudio{}

	svc := NewAIService(&config.Config{},
		&MockLLM{Response: "Script"},
		&MockTTS{Format: "mp3"},
		pm,
		mockAudio,
		mockPOI,
		mockBeacon,
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

	ctx := context.Background()
	svc.Start()

	// 1. Test Immediate Marker on PlayPOI
	// This will start audio playback (MockAudio.IsPlayingVal = true)
	// And launch monitorPlayback in background
	svc.PlayPOI(ctx, "Q1", true, &sim.Telemetry{}, "uniform")

	if !mockBeacon.TargetSet {
		t.Error("Expected Beacon to be set immediately upon PlayPOI call")
	}
	if mockBeacon.LastTgtLat != 10.0 {
		t.Errorf("Expected lat 10.0, got %f", mockBeacon.LastTgtLat)
	}

	// 2. Test Marker Switch on Playback End (Pipeline)
	// Reset Beacon tracking
	mockBeacon.TargetSet = false
	mockBeacon.LastTgtLat = 0

	// Prepare Next (Staging)
	_ = svc.PrepareNextNarrative(ctx, "QNext", "uniform", &sim.Telemetry{})

	// Verify staged
	svc.mu.Lock()
	if svc.stagedNarrative == nil {
		t.Fatal("Failed to stage narrative")
	}
	svc.mu.Unlock()

	// Simulate Audio Finish
	mockAudio.IsPlayingVal = false

	// Wait for monitorPlayback to detect finish (ticker 100ms)
	time.Sleep(300 * time.Millisecond)

	// Check if beacon switched to QNext (Lat 20.0)
	if !mockBeacon.TargetSet {
		t.Error("Expected Beacon to be set after playback ended with staged narrative")
	}
	if mockBeacon.LastTgtLat != 20.0 {
		t.Errorf("Expected Beacon to switch to QNext (Lat 20.0), got %f", mockBeacon.LastTgtLat)
	}
}
