package narrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func TestAIService_PlayPOI(t *testing.T) {
	// Setup Prompts (using real manager with temp dir)
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Script for {{.NameUser}}"), 0o644)
	userCommonDir := filepath.Join(tempDir, "common")
	_ = os.MkdirAll(userCommonDir, 0o755)

	pm, err := prompts.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create prompt manager: %v", err)
	}

	tests := []struct {
		name            string
		poiID           string
		poiFind         *model.POI
		poiFindErr      error
		llmErr          error
		ttsErr          error
		audioErr        error
		wikiContent     string
		wikiErr         error
		expectNarration bool
	}{
		{
			name:  "Happy Path",
			poiID: "Q1",
			poiFind: &model.POI{
				WikidataID: "Q1",
				NameUser:   "Test POI",
				Lat:        10.0,
				Lon:        20.0,
			},
			expectNarration: true,
		},
		{
			name:            "POI Not Found",
			poiID:           "QMISSING",
			poiFind:         nil,
			poiFindErr:      errors.New("not found"),
			expectNarration: false,
		},
		{
			name:            "LLM Failure",
			poiID:           "Q2",
			poiFind:         &model.POI{WikidataID: "Q2", NameUser: "P2"},
			llmErr:          errors.New("llm fail"),
			expectNarration: false,
		},
		{
			name:            "TTS Failure",
			poiID:           "Q3",
			poiFind:         &model.POI{WikidataID: "Q3", NameUser: "P3"},
			ttsErr:          errors.New("tts fail"),
			expectNarration: false,
		},
		{
			name:            "Audio Failure",
			poiID:           "Q4",
			poiFind:         &model.POI{WikidataID: "Q4", NameUser: "P4"},
			audioErr:        errors.New("audio fail"),
			expectNarration: false, // It fails at end, but count increases only at end of function? No, count at end.
		},
		{
			name:  "Wikipedia Fetch",
			poiID: "QWiki",
			poiFind: &model.POI{
				WikidataID: "QWiki",
				NameUser:   "Wiki POI",
				WPURL:      "https://en.wikipedia.org/wiki/Foo",
			},
			wikiContent:     "Some wiki text",
			expectNarration: true, // Should still narrate even if wiki fails or succeeds
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
			mockLLM := &MockLLM{Response: "Generated Script", Err: tt.llmErr}
			mockTTS := &MockTTS{Format: "mp3", Err: tt.ttsErr}
			mockAudio := &MockAudio{PlayErr: tt.audioErr}
			mockPOI := &MockPOIProvider{
				GetPOIFunc: func(ctx context.Context, qid string) (*model.POI, error) {
					if qid == tt.poiID {
						return tt.poiFind, tt.poiFindErr
					}
					return nil, errors.New("unexpected poi")
				},
			}
			mockGeo := &MockGeo{Country: "US"}
			mockSim := &MockSim{Telemetry: sim.Telemetry{
				Latitude: 10.0, Longitude: 20.0, Heading: 0,
			}}
			mockStore := &MockStore{}
			mockWiki := &MockWikipedia{Content: tt.wikiContent, Err: tt.wikiErr}
			mockBeacon := &MockBeacon{}

			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil)
			svc.Start()

			// Call PlayPOI (synchronous wrapper logic needed or wait?)
			// PlayPOI launches goroutine. We need to wait.
			// To test deterministic behavior, call narratePOI directly?
			// narratePOI is private.
			// We can expose it for test or use WaitGroup if we modify service, OR simply sleep/poll (flaky).
			// OR: We check side effects after a small delay.

			// Actually, PlayPOI launches generic goroutine.
			// Use WaitGroup?
			// Better: Inspect logs? No.
			// Hack: In `mocks_dev_test`, we can add a channel to signal completion in MockAudio if successful.

			// For this test, I'll rely on a small sleep for simplicity as it's a unit test with mocks (fast).
			svc.PlayPOI(context.Background(), tt.poiID, true, &sim.Telemetry{}, "uniform")

			// Wait a bit
			time.Sleep(200 * time.Millisecond)

			if tt.expectNarration {
				if svc.NarratedCount() != 1 && tt.audioErr != nil {
					// Audio error prevents count increase in current implementation?
					// Let's check logic:
					// err = s.audio.Play... if err != nil { return }
					// s.narratedCount++ is AFTER audio play.
					// So if audio fails, count is 0.
				} else if svc.NarratedCount() != 1 && tt.audioErr == nil {
					t.Errorf("Expected 1 narrated POI, got %d", svc.NarratedCount())
				}
			} else {
				if svc.NarratedCount() != 0 {
					t.Errorf("Expected 0 narrated POIs, got %d", svc.NarratedCount())
				}
			}
		})
	}
}

func TestAIService_ContextAndNav_V2(t *testing.T) {
	tests := []struct {
		name           string
		poi            *model.POI
		telemetry      sim.Telemetry
		recentPOIs     []*model.POI
		wikiContent    string
		expectInPrompt []string
	}{
		{
			name: "With Nav Instruction",
			poi: &model.POI{
				WikidataID: "QNav", Lat: 10.05, Lon: 20.0, // North of user (>4.5km)
			},
			telemetry: sim.Telemetry{
				Latitude: 10.0, Longitude: 20.0, Heading: 0, // Heading North
			},
			// 0.05 deg ~ 5.5km -> "At your 12 o'clock, about 3 miles away" (Airborne > 4.5km)
			expectInPrompt: []string{"At your 12 o'clock", "about"},
		},
		{
			name: "With Recent POIs",
			poi:  &model.POI{WikidataID: "QCurrent", Lat: 0, Lon: 0},
			recentPOIs: []*model.POI{
				{NameEn: "Old POI", Category: "History"},
			},
			expectInPrompt: []string{"Old POI (History)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &MockLLM{Response: "Script"}
			mockPOI := &MockPOIProvider{
				GetPOIFunc: func(ctx context.Context, qid string) (*model.POI, error) {
					return tt.poi, nil
				},
			}
			mockSim := &MockSim{Telemetry: tt.telemetry}
			mockStore := &MockStore{RecentPOIs: tt.recentPOIs}

			var capturedPrompt string
			mockLLM.GenerateTextFunc = func(ctx context.Context, name, prompt string) (string, error) {
				capturedPrompt = prompt
				return "Script", nil
			}

			// Need basic required mocks
			mockTTS := &MockTTS{Format: "mp3"}
			mockAudio := &MockAudio{}
			mockGeo := &MockGeo{Country: "US"}
			mockWiki := &MockWikipedia{Content: tt.wikiContent}
			mockBeacon := &MockBeacon{}
			// Temp prompts
			tmpDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(tmpDir, "narrator"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "narrator", "script.tmpl"), []byte("Prompt: {{.NavInstruction}} Context: {{.RecentPoisContext}}"), 0o644)
			_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)
			pm, _ := prompts.NewManager(tmpDir)

			svc := NewAIService(&config.Config{}, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil)
			svc.Start()

			svc.PlayPOI(context.Background(), tt.poi.WikidataID, true, &tt.telemetry, "uniform")
			time.Sleep(20 * time.Millisecond) // Wait for go routine

			for _, expect := range tt.expectInPrompt {
				if capturedPrompt == "" {
					t.Fatalf("GenerateText was not called, capturedPrompt is empty")
				}
				if !strings.Contains(capturedPrompt, expect) {
					t.Errorf("Expected prompt to contain '%s', but got: '%s'", expect, capturedPrompt)
				}
			}
		})
	}
}

func TestAIService_Lifecycle(t *testing.T) {
	// Simple coverage for Start/Stop/Stats
	pm, _ := prompts.NewManager(t.TempDir())
	svc := NewAIService(&config.Config{}, &MockLLM{}, &MockTTS{}, pm, &MockAudio{}, &MockPOIProvider{}, &MockBeacon{}, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

	if svc.running {
		t.Error("should not be running initially")
	}
	svc.Start()
	if !svc.running {
		t.Error("should be running after start")
	}

	stats := svc.Stats()
	if stats == nil {
		t.Error("stats should not be nil")
	}

	svc.Stop()
	if svc.running {
		t.Error("should not be running after stop")
	}
}

func TestAIService_NavUnits(t *testing.T) {
	tests := []struct {
		name           string
		units          string
		poi            *model.POI
		telemetry      sim.Telemetry
		expectInPrompt []string
	}{
		{
			name:           "Imperial Default (>1 mile)",
			units:          "imperial",
			poi:            &model.POI{Lat: 10.05, Lon: 20.0}, // ~5.5km
			telemetry:      sim.Telemetry{Latitude: 10.0, Longitude: 20.0, Heading: 0},
			expectInPrompt: []string{"miles"},
		},
		{
			name:           "Metric (>1 km)",
			units:          "metric",
			poi:            &model.POI{Lat: 10.05, Lon: 20.0}, // ~5.5km
			telemetry:      sim.Telemetry{Latitude: 10.0, Longitude: 20.0, Heading: 0},
			expectInPrompt: []string{"kilometers", "about 6 kilometers"},
		},
		{
			name:           "Hybrid -> Metric",
			units:          "hybrid",
			poi:            &model.POI{Lat: 10.05, Lon: 20.0},
			telemetry:      sim.Telemetry{Latitude: 10.0, Longitude: 20.0, Heading: 0},
			expectInPrompt: []string{"kilometers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Narrator: config.NarratorConfig{
					Units:          tt.units,
					TargetLanguage: "en",
				},
			}
			mockLLM := &MockLLM{Response: "Script"}
			mockPOI := &MockPOIProvider{
				GetPOIFunc: func(ctx context.Context, qid string) (*model.POI, error) {
					return tt.poi, nil
				},
			}
			mockSim := &MockSim{Telemetry: tt.telemetry}

			var capturedPrompt string
			mockLLM.GenerateTextFunc = func(ctx context.Context, name, prompt string) (string, error) {
				capturedPrompt = prompt
				return "Script", nil
			}

			// Required mocks
			mockTTS := &MockTTS{Format: "mp3"}
			mockAudio := &MockAudio{}
			mockGeo := &MockGeo{Country: "US"}
			mockStore := &MockStore{}
			mockWiki := &MockWikipedia{Content: "Wiki"}
			mockBeacon := &MockBeacon{}

			// Init Prompts
			tmpDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(tmpDir, "narrator"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "narrator", "script.tmpl"), []byte("Prompt: {{.NavInstruction}}"), 0o644)
			_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)
			// Create dummy units templates to avoid load error
			_ = os.MkdirAll(filepath.Join(tmpDir, "units"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "units", "imperial.tmpl"), []byte("Imperial rules"), 0o644)
			_ = os.WriteFile(filepath.Join(tmpDir, "units", "metric.tmpl"), []byte("Metric rules"), 0o644)
			_ = os.WriteFile(filepath.Join(tmpDir, "units", "hybrid.tmpl"), []byte("Hybrid rules"), 0o644)

			pm, _ := prompts.NewManager(tmpDir)

			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil)
			svc.Start()

			svc.PlayPOI(context.Background(), "QTest", true, &tt.telemetry, "uniform")
			time.Sleep(50 * time.Millisecond) // Wait for goroutine

			if capturedPrompt == "" {
				t.Fatal("Prompt not captured")
			}
			for _, expect := range tt.expectInPrompt {
				if !strings.Contains(capturedPrompt, expect) {
					t.Errorf("Expected prompt to contain '%s', got '%s'", expect, capturedPrompt)
				}
			}
		})
	}
}

func TestAIService_BeaconCleanup(t *testing.T) {
	// Setup Prompts
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	mockBeacon := &MockBeacon{}

	// Scenario: Audio playback fails, Beacon should be cleared (it was set at start of play)
	svc := NewAIService(&config.Config{}, &MockLLM{Response: "Script"}, &MockTTS{Format: "mp3"}, pm, &MockAudio{PlayErr: errors.New("fail")}, &MockPOIProvider{GetPOIFunc: func(_ context.Context, _ string) (*model.POI, error) {
		return &model.POI{WikidataID: "Q1"}, nil
	}}, mockBeacon, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

	svc.Start()
	svc.PlayPOI(context.Background(), "Q1", true, &sim.Telemetry{}, "uniform")
	time.Sleep(50 * time.Millisecond) // Wait for go routine

	if !mockBeacon.Cleared {
		t.Error("Expected Beacon to be cleared after Audio failure, but it was not")
	}
}

func TestAIService_GeneratePlay(t *testing.T) {
	// Verify decoupled methods
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	svc := NewAIService(&config.Config{},
		&MockLLM{Response: "GenScript"},
		&MockTTS{Format: "mp3"},
		pm,
		&MockAudio{},
		&MockPOIProvider{GetPOIFunc: func(_ context.Context, _ string) (*model.POI, error) {
			return &model.POI{WikidataID: "QGen"}, nil
		}},
		&MockBeacon{},
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

	ctx := context.Background()

	// 1. Generate
	narrative, err := svc.GenerateNarrative(ctx, "QGen", "uniform", &sim.Telemetry{})
	if err != nil {
		t.Fatalf("GenerateNarrative failed: %v", err)
	}
	if narrative == nil {
		t.Fatal("Narrative is nil")
	}
	if narrative.Script != "GenScript" {
		t.Errorf("Expected script 'GenScript', got '%s'", narrative.Script)
	}
	if narrative.POI.WikidataID != "QGen" {
		t.Errorf("Expected POI QGen, got %s", narrative.POI.WikidataID)
	}

	// 2. Play
	err = svc.PlayNarrative(ctx, narrative)
	if err != nil {
		t.Fatalf("PlayNarrative failed: %v", err)
	}
	// Wait for playback "busy" loop (mock audio not busy by default so it returns immediately)
	if svc.NarratedCount() != 1 {
		t.Errorf("Expected narrated count 1, got %d", svc.NarratedCount())
	}
}
func TestAIService_UpdateTripSummary(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "summary_update.tmpl"), []byte("Summary: {{.CurrentSummary}} New: {{.LastScript}} Limit: {{.MaxWords}}"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)

	pm, _ := prompts.NewManager(tempDir)

	tests := []struct {
		name           string
		currentSummary string
		lastTitle      string
		lastScript     string
		maxWords       int
		llmResponse    string
		expectSummary  string
	}{
		{
			name:           "First Summary",
			currentSummary: "",
			lastTitle:      "Initial Stop",
			lastScript:     "Hello world",
			maxWords:       100,
			llmResponse:    "New Summary",
			expectSummary:  "New Summary",
		},
		{
			name:           "Summary Update",
			currentSummary: "Old info",
			lastTitle:      "Second Stop",
			lastScript:     "More info",
			maxWords:       200,
			llmResponse:    "Consolidated info",
			expectSummary:  "Consolidated info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Narrator: config.NarratorConfig{
					SummaryMaxWords: tt.maxWords,
				},
			}
			mockLLM := &MockLLM{Response: tt.llmResponse}
			svc := &AIService{
				cfg:         cfg,
				llm:         mockLLM,
				prompts:     pm,
				tripSummary: tt.currentSummary,
			}

			svc.updateTripSummary(context.Background(), tt.lastTitle, tt.lastScript)

			if svc.getTripSummary() != tt.expectSummary {
				t.Errorf("Expected summary '%s', got '%s'", tt.expectSummary, svc.getTripSummary())
			}
		})
	}
}

func TestAIService_LatencyTracking(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	// Mock LLM with delay to simulate generation time
	mockLLM := &MockLLM{Response: "Script"}
	mockLLM.GenerateTextFunc = func(ctx context.Context, name, prompt string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "Script", nil
	}

	svc := NewAIService(&config.Config{},
		mockLLM,
		&MockTTS{Format: "mp3"},
		pm,
		&MockAudio{},
		&MockPOIProvider{GetPOIFunc: func(_ context.Context, _ string) (*model.POI, error) {
			return &model.POI{WikidataID: "QLatency"}, nil
		}},
		&MockBeacon{},
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

	// 1. Initial latencies should be empty
	stats := svc.Stats()
	if _, ok := stats["latency_avg_ms"]; ok {
		t.Error("latency_avg_ms should be missing initially")
	}

	// 2. GenerateNarrative (should take ~50ms)
	_, err := svc.GenerateNarrative(context.Background(), "QLatency", "uniform", &sim.Telemetry{})
	if err != nil {
		t.Fatalf("GenerateNarrative failed: %v", err)
	}

	// 3. Check Stats
	stats = svc.Stats()
	val, ok := stats["latency_avg_ms"]
	if !ok {
		t.Fatal("latency_avg_ms missing after generation")
	}

	latencyMs, ok := val.(int64)
	if !ok {
		t.Fatalf("latency_avg_ms is not int64, got %T", val)
	}

	if latencyMs < 40 {
		t.Errorf("Expected latency >= 40ms (from simulated delay), got %dms", latencyMs)
	}
}

func TestAIService_PipelineFlow(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	tests := []struct {
		name             string
		stagedPOIID      string // If set, we prepare this first
		requestPOIID     string // What we request to Play
		expectedNarrated int
		expectedStaged   bool // Should staged be nil after Play? (Always true if consumed or cleared)
	}{
		{
			name:             "Happy Path: Consumes Staged Constraint",
			stagedPOIID:      "QStaged",
			requestPOIID:     "QStaged",
			expectedNarrated: 1,
			expectedStaged:   false, // Should be consumed
		},
		{
			name:             "Mismatch: Staged A, Play B -> Ignores Stage, Generates B",
			stagedPOIID:      "QStaged",
			requestPOIID:     "QOther",
			expectedNarrated: 1,     // Only QOther played (Staged is discarded)
			expectedStaged:   false, // Cleared on mismatch? No, PlayPOI clears it if mismatch? Implementation says: if mismatch, we ignore staged. Does it clear? Let's check logic: "if s.stagedNarrative != nil ... if match { use } else { s.stagedNarrative = nil }" -> Yes, it clears.
		},
		{
			name:             "No Staged Data -> Generates Fresh",
			stagedPOIID:      "",
			requestPOIID:     "QFresh",
			expectedNarrated: 1,
			expectedStaged:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &MockLLM{Response: "Script"}
			mockPOI := &MockPOIProvider{
				GetPOIFunc: func(_ context.Context, qid string) (*model.POI, error) {
					return &model.POI{WikidataID: qid, NameEn: "POI"}, nil
				},
			}

			svc := NewAIService(&config.Config{},
				mockLLM,
				&MockTTS{Format: "mp3"},
				pm,
				&MockAudio{},
				mockPOI,
				&MockBeacon{},
				&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

			ctx := context.Background()

			// 1. Stage if needed
			if tt.stagedPOIID != "" {
				err := svc.PrepareNextNarrative(ctx, tt.stagedPOIID, "uniform", &sim.Telemetry{})
				if err != nil {
					t.Fatalf("PrepareNextNarrative failed: %v", err)
				}
				// Verify staged
				func() {
					svc.mu.Lock()
					defer svc.mu.Unlock()
					if svc.stagedNarrative == nil {
						t.Fatal("stagedNarrative is nil after Prepare")
					}
				}()
			}

			// 2. Play
			svc.PlayPOI(ctx, tt.requestPOIID, false, &sim.Telemetry{}, "uniform")

			// 3. Verify
			func() {
				svc.mu.Lock()
				defer svc.mu.Unlock()
				if !tt.expectedStaged && svc.stagedNarrative != nil {
					t.Error("stagedNarrative should be nil after PlayPOI")
				}
			}()

			if svc.NarratedCount() != tt.expectedNarrated {
				t.Errorf("Expected narrated count %d, got %d", tt.expectedNarrated, svc.NarratedCount())
			}
		})
	}
}
func TestAIService_ScriptValidation(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	// Create a script that is definitely too long
	// Config Default MaxWords is usually around 400. Limit is +200 = 600.
	// We generate 1000 words.
	longScript := strings.Repeat("word ", 1000)

	mockLLM := &MockLLM{Response: longScript}
	mockPOI := &MockPOIProvider{
		GetPOIFunc: func(_ context.Context, qid string) (*model.POI, error) {
			return &model.POI{WikidataID: qid}, nil
		},
	}

	cfg := &config.Config{
		Narrator: config.NarratorConfig{
			NarrationLengthMax: 200, // Explicitly set low max
		},
	}

	svc := NewAIService(cfg,
		mockLLM,
		&MockTTS{Format: "mp3"},
		pm,
		&MockAudio{},
		mockPOI,
		&MockBeacon{},
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil)

	_, err := svc.GenerateNarrative(context.Background(), "QLong", "uniform", &sim.Telemetry{})
	if err == nil {
		t.Fatal("Expected error for excessively long script, got nil")
	}

	if !strings.Contains(err.Error(), "generated script too long") {
		t.Errorf("Expected error message to contain 'generated script too long', got: %v", err)
	}
}
