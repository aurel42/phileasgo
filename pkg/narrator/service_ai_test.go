package narrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/session"
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
			name:  "Wikipedia Fetch",
			poiID: "QWiki",
			poiFind: &model.POI{
				WikidataID: "QWiki",
				NameUser:   "Wiki POI",
				WPURL:      "https://en.wikipedia.org/wiki/Foo",
			},
			wikiContent:     "Some wiki text",
			expectNarration: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mocks
			cfg := config.NewProvider(&config.Config{
				Narrator: config.NarratorConfig{
					ActiveTargetLanguage:  "en",
					TargetLanguageLibrary: []string{"en"},
				},
			}, nil)
			mockLLM := &MockLLM{Response: "Generated Script", Err: tt.llmErr}
			mockTTS := &MockTTS{Format: "mp3", Err: tt.ttsErr}
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

			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockPOI, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

			// Setup callback to verify narration
			var narrated bool
			svc.SetOnPlayback(func(n *model.Narrative, priority bool) {
				narrated = true
				svc.session().AddNarration(n.Title, n.Title, n.Script) // Simulate history recording normally done by play
			})

			svc.Start()

			svc.PlayPOI(context.Background(), tt.poiID, true, false, &sim.Telemetry{}, "uniform")

			// Trigger Priority Processing (Simulate Main Loop)
			svc.ProcessGenerationQueue(context.Background())

			// Wait a bit
			time.Sleep(200 * time.Millisecond)

			if tt.expectNarration {
				if !narrated {
					t.Errorf("Expected narration for %s", tt.name)
				}
			} else {
				if narrated {
					t.Errorf("Expected no narration for %s", tt.name)
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
			mockGeo := &MockGeo{Country: "US"}
			mockWiki := &MockWikipedia{Content: tt.wikiContent}

			// Temp prompts
			tmpDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(tmpDir, "narrator"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "narrator", "script.tmpl"), []byte("Prompt: At your {{.ClockPos}} o'clock about Context: {{.RecentContext}}"), 0o644)
			_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)
			pm, _ := prompts.NewManager(tmpDir)

			svc := NewAIService(config.NewProvider(&config.Config{}, nil), mockLLM, mockTTS, pm, mockPOI, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil, nil, nil, session.NewManager(nil))
			svc.Start()

			svc.PlayPOI(context.Background(), tt.poi.WikidataID, true, false, &tt.telemetry, "uniform")
			svc.ProcessGenerationQueue(context.Background())
			time.Sleep(50 * time.Millisecond) // Wait for go routine

			if capturedPrompt == "" {
				t.Fatalf("GenerateText was not called, capturedPrompt is empty")
			}
			for _, expect := range tt.expectInPrompt {
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
	svc := NewAIService(config.NewProvider(&config.Config{}, nil), &MockLLM{}, &MockTTS{}, pm, &MockPOIProvider{}, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewProvider(&config.Config{
				Narrator: config.NarratorConfig{
					Units:                 tt.units,
					ActiveTargetLanguage:  "en",
					TargetLanguageLibrary: []string{"en"},
				},
			}, nil)
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
			mockGeo := &MockGeo{Country: "US"}
			mockStore := &MockStore{}
			mockWiki := &MockWikipedia{Content: "Wiki"}

			// Init Prompts
			tmpDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(tmpDir, "narrator"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "narrator", "script.tmpl"), []byte("Prompt: {{if eq .UnitSystem \"metric\"}}about {{.DistKm}} kilometers{{else}}{{.DistNm}} miles{{end}}"), 0o644)
			_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)
			_ = os.MkdirAll(filepath.Join(tmpDir, "units"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "units", "imperial.tmpl"), []byte("Imperial rules"), 0o644)
			_ = os.WriteFile(filepath.Join(tmpDir, "units", "metric.tmpl"), []byte("Metric rules"), 0o644)

			pm, _ := prompts.NewManager(tmpDir)

			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockPOI, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil, nil, nil, session.NewManager(nil))
			svc.Start()

			svc.PlayPOI(context.Background(), "Q8080", true, false, &tt.telemetry, "uniform")
			svc.ProcessGenerationQueue(context.Background())
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

func TestAIService_GeneratePlay(t *testing.T) {
	// Verify decoupled methods
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	svc := NewAIService(config.NewProvider(&config.Config{}, nil),
		&MockLLM{Response: "GenScript"},
		&MockTTS{Format: "mp3"},
		pm,
		&MockPOIProvider{GetPOIFunc: func(_ context.Context, _ string) (*model.POI, error) {
			return &model.POI{WikidataID: "QGen"}, nil
		}},
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

	ctx := context.Background()

	// 1. Generate
	req := GenerationRequest{
		Type:   model.NarrativeTypePOI,
		Title:  "QGen",
		SafeID: "QGen",
		Prompt: "Test Prompt",
		POI:    &model.POI{WikidataID: "QGen"},
	}
	narrative, err := svc.GenerateNarrative(ctx, &req)
	if err != nil {
		t.Fatalf("GenerateNarrative failed: %v", err)
	}
	if narrative == nil {
		t.Fatal("Narrative is nil")
	}
	if narrative.Script != "GenScript" {
		t.Errorf("Expected script 'GenScript', got '%s'", narrative.Script)
	}

	// 2. Register callback and trigger
	var called bool
	svc.SetOnPlayback(func(n *model.Narrative, priority bool) {
		called = true
	})
	svc.enqueuePlayback(narrative, true)
	if !called {
		t.Error("Expected onPlayback to be called")
	}
}

func TestAIService_SummarizeAndLogEvent(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "event_summary.tmpl"), []byte("Summary: {{.LastScript}}"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)

	pm, _ := prompts.NewManager(tempDir)

	cfg := config.NewProvider(&config.Config{
		Narrator: config.NarratorConfig{},
	}, nil)
	mockLLM := &MockLLM{Response: "Clean Summary"}
	svc := &AIService{
		cfg:        cfg,
		llm:        mockLLM,
		prompts:    pm,
		sessionMgr: session.NewManager(nil),
		sim:        &MockSim{},
	}

	n := &model.Narrative{
		Type:   model.NarrativeTypePOI,
		Title:  "Test POI",
		Script: "Script Content",
		POI: &model.POI{
			WikidataID: "Q123",
			Icon:       "castle.png",
			Lat:        48.85,
			Lon:        2.35,
		},
	}

	svc.summarizeAndLogEvent(context.Background(), n)

	events := svc.session().GetState().Events
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Clean Summary" {
		t.Errorf("Expected summary 'Clean Summary', got '%s'", events[0].Summary)
	}
}

func TestAIService_LatencyTracking(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	pm, _ := prompts.NewManager(tempDir)

	mockLLM := &MockLLM{Response: "Script"}
	mockLLM.GenerateTextFunc = func(ctx context.Context, name, prompt string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "Script", nil
	}

	svc := NewAIService(config.NewProvider(&config.Config{}, nil),
		mockLLM,
		&MockTTS{Format: "mp3"},
		pm,
		&MockPOIProvider{GetPOIFunc: func(_ context.Context, _ string) (*model.POI, error) {
			return &model.POI{WikidataID: "QLatency"}, nil
		}},
		&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

	// 2. GenerateNarrative (should take ~50ms)
	req := GenerationRequest{
		Type:   model.NarrativeTypePOI,
		Title:  "QLatency",
		SafeID: "QLatency",
		Prompt: "Test Prompt",
		POI:    &model.POI{WikidataID: "QLatency"},
	}
	_, err := svc.GenerateNarrative(context.Background(), &req)
	if err != nil {
		t.Fatalf("GenerateNarrative failed: %v", err)
	}

	// 3. Check Stats
	stats := svc.Stats()
	val, ok := stats["latency_avg_ms"]
	if !ok {
		t.Fatal("latency_avg_ms missing after generation")
	}

	latencyMs, ok := val.(int64)
	if !ok {
		t.Fatalf("latency_avg_ms is not int64, got %T", val)
	}

	if latencyMs < 40 {
		t.Errorf("Expected latency >= 40ms, got %dms", latencyMs)
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
		stagedPOIID      string
		requestPOIID     string
		expectedNarrated int
	}{
		{
			name:             "Happy Path: Consumes Staged",
			stagedPOIID:      "QStaged",
			requestPOIID:     "QStaged",
			expectedNarrated: 2, // 1 from stage + 1 from explicit request
		},
		{
			name:             "No Staged Data -> Generates Fresh",
			stagedPOIID:      "",
			requestPOIID:     "QFresh",
			expectedNarrated: 1,
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

			svc := NewAIService(config.NewProvider(&config.Config{}, nil),
				mockLLM,
				&MockTTS{Format: "mp3"},
				pm,
				mockPOI,
				&MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

			var narratedCount int
			svc.SetOnPlayback(func(n *model.Narrative, priority bool) {
				narratedCount++
				svc.session().AddNarration(n.Title, n.Title, n.Script)
			})

			ctx := context.Background()

			if tt.stagedPOIID != "" {
				err := svc.PrepareNextNarrative(ctx, tt.stagedPOIID, "uniform", &sim.Telemetry{})
				if err != nil {
					t.Fatalf("PrepareNextNarrative failed: %v", err)
				}
			}

			svc.PlayPOI(ctx, tt.requestPOIID, true, false, &sim.Telemetry{}, "uniform")
			svc.ProcessGenerationQueue(ctx)
			time.Sleep(100 * time.Millisecond)

			if narratedCount != tt.expectedNarrated {
				t.Errorf("Expected narrated count %d, got %d", tt.expectedNarrated, narratedCount)
			}
		})
	}
}

func TestAIService_ScriptValidation(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempDir, "narrator"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "narrator", "script.tmpl"), []byte("Msg"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempDir, "common"), 0o755)
	_ = os.MkdirAll(filepath.Join(tempDir, "context"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "context", "rescue_script.tmpl"), []byte("RESCUED {{.Script}}"), 0o644)
	pm, _ := prompts.NewManager(tempDir)

	longScript := strings.Repeat("word ", 1000)
	mockLLM := &MockLLM{Response: longScript}
	mockPOI := &MockPOIProvider{
		GetPOIFunc: func(_ context.Context, qid string) (*model.POI, error) {
			return &model.POI{WikidataID: qid}, nil
		},
	}

	cfg := config.NewProvider(&config.Config{
		Narrator: config.NarratorConfig{
			NarrationLengthLongWords: 200,
		},
	}, nil)

	svc := NewAIService(cfg, mockLLM, &MockTTS{Format: "mp3"}, pm, mockPOI, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWikipedia{}, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

	req := GenerationRequest{
		Type:     model.NarrativeTypePOI,
		Title:    "QLong",
		SafeID:   "QLong",
		Prompt:   "Test Prompt",
		POI:      &model.POI{WikidataID: "QLong"},
		MaxWords: 200,
	}
	narrative, err := svc.GenerateNarrative(context.Background(), &req)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !strings.Contains(narrative.Script, "word word") {
		t.Error("Expected original long script to be preserved")
	}
}

func TestAllProductionTemplatesExecuteSuccessfully(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	promptsDir := filepath.Join(projectRoot, "configs", "prompts")

	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		t.Skip("configs/prompts not found, skipping production template test")
	}

	pm, err := prompts.NewManager(promptsDir)
	if err != nil {
		t.Fatalf("Failed to load production templates: %v", err)
	}

	cfg := config.NewProvider(config.DefaultConfig(), nil)
	svc := NewAIService(cfg, &MockLLM{}, &MockTTS{}, pm, &MockPOIProvider{}, &MockGeo{}, &MockSim{}, &MockStore{}, nil, nil, nil, nil, nil, nil, nil, session.NewManager(nil))

	data := svc.promptAssembler.NewPromptData(svc.getSessionState())
	data["FlightStage"] = "Cruise"
	data["NameNative"] = "Test POI"
	data["POINameNative"] = "Test POI"
	data["NameUser"] = "Test POI"
	data["POINameUser"] = "Test POI"
	data["Category"] = "City"
	data["WikipediaText"] = "Test wiki content."
	data["NavInstruction"] = "10km ahead"
	data["Lat"] = 10.0
	data["Lon"] = 20.0
	data["AltitudeAGL"] = 5000.0
	data["Heading"] = 180.0
	data["GroundSpeed"] = 120.0
	data["RecentContext"] = "None"
	data["UnitsInstruction"] = "Use metric units."
	data["FlightStatusSentence"] = "Flying over the sea."
	data["From"] = "France"
	data["To"] = "Italy"
	data["Title"] = "Sample Title"
	data["Script"] = "Sample Script Content"
	data["CurrentSummary"] = "Summary"
	data["LastTitle"] = "Last"
	data["LastScript"] = "Last"
	data["Images"] = []prompt.ImageResult{{Title: "Img", URL: "url"}}
	data["CategoryList"] = "Airport"
	data["TopicName"] = "Local History"

	err = filepath.Walk(promptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		rel, _ := filepath.Rel(promptsDir, path)
		name := filepath.ToSlash(rel)
		if strings.HasPrefix(name, "common/") {
			return nil
		}
		t.Run(name, func(t *testing.T) {
			_, err := pm.Render(name, data)
			if err != nil {
				t.Errorf("Failed to render %s: %v", name, err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}
}
