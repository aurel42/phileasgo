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
			mockWiki := &MockWiki{Content: tt.wikiContent, Err: tt.wikiErr}
			mockBeacon := &MockBeacon{}

			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil)
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
			svc.PlayPOI(context.Background(), tt.poiID, true, &sim.Telemetry{})

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

func TestAIService_ContextAndNav(t *testing.T) {
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
				WikidataID: "QNav", Lat: 10.01, Lon: 20.0, // North of user
			},
			telemetry: sim.Telemetry{
				Latitude: 10.0, Longitude: 20.0, Heading: 0, // Heading North
			},
			expectInPrompt: []string{"Straight ahead", "less than a mile"}, // 1.1km ~ 0.68 miles
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
			mockWiki := &MockWiki{Content: tt.wikiContent}
			mockBeacon := &MockBeacon{}
			// Temp prompts
			tmpDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(tmpDir, "narrator"), 0o755)
			_ = os.WriteFile(filepath.Join(tmpDir, "narrator", "script.tmpl"), []byte("Prompt: {{.NavInstruction}} Context: {{.RecentPoisContext}}"), 0o644)
			_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)
			pm, _ := prompts.NewManager(tmpDir)

			svc := NewAIService(&config.Config{}, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil)
			svc.Start()

			svc.PlayPOI(context.Background(), tt.poi.WikidataID, true, &tt.telemetry)
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
	svc := NewAIService(&config.Config{}, &MockLLM{}, &MockTTS{}, pm, &MockAudio{}, &MockPOIProvider{}, &MockBeacon{}, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWiki{}, nil, nil, nil)

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
			poi:            &model.POI{Lat: 10.02, Lon: 20.0}, // ~2.2km = ~1.2nm
			telemetry:      sim.Telemetry{Latitude: 10.0, Longitude: 20.0, Heading: 0},
			expectInPrompt: []string{"miles"},
		},
		{
			name:           "Metric (>1 km)",
			units:          "metric",
			poi:            &model.POI{Lat: 10.02, Lon: 20.0}, // ~2.2km
			telemetry:      sim.Telemetry{Latitude: 10.0, Longitude: 20.0, Heading: 0},
			expectInPrompt: []string{"kilometers", "about 2 kilometers"},
		},
		{
			name:           "Hybrid -> Metric",
			units:          "hybrid",
			poi:            &model.POI{Lat: 10.02, Lon: 20.0},
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
			mockWiki := &MockWiki{Content: "Wiki"}
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

			svc := NewAIService(cfg, mockLLM, mockTTS, pm, mockAudio, mockPOI, mockBeacon, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil)
			svc.Start()

			svc.PlayPOI(context.Background(), "QTest", true, &tt.telemetry)
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

	// Scenario: LLM fails, Beacon should be cleared
	svc := NewAIService(&config.Config{}, &MockLLM{Err: errors.New("fail")}, &MockTTS{}, pm, &MockAudio{}, &MockPOIProvider{GetPOIFunc: func(_ context.Context, _ string) (*model.POI, error) {
		return &model.POI{}, nil
	}}, mockBeacon, &MockGeo{}, &MockSim{}, &MockStore{}, &MockWiki{}, nil, nil, nil)

	svc.Start()
	svc.PlayPOI(context.Background(), "Q1", true, &sim.Telemetry{})
	time.Sleep(50 * time.Millisecond) // Wait for go routine

	if !mockBeacon.Cleared {
		t.Error("Expected Beacon to be cleared after LLM failure, but it was not")
	}
}
