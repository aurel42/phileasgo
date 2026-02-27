package narrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/request"
	"phileasgo/pkg/session"
)

func TestAIService_GenerateNarrative_ProfileAndWords(t *testing.T) {
	// Setup service with mocks
	cfg := config.NewProvider(&config.Config{
		Narrator: config.NarratorConfig{
			ActiveTargetLanguage:  "en",
			TargetLanguageLibrary: []string{"en"},
		},
	}, nil)
	mockTTS := &MockTTS{Format: "mp3"}

	tests := []struct {
		name         string
		req          GenerationRequest
		wantProfile  string
		wantMaxWords int
	}{
		{
			name: "POI Narration - Uses narration profile",
			req: GenerationRequest{
				Type:     model.NarrativeTypePOI,
				Prompt:   "Tell me about this place.",
				MaxWords: 100,
			},
			wantProfile:  "narration",
			wantMaxWords: 100,
		},
		{
			name: "Screenshot Analysis - Uses screenshot profile and image",
			req: GenerationRequest{
				Type:      model.NarrativeTypeScreenshot,
				Prompt:    "What is in this image?",
				ImagePath: "test_image.png",
				MaxWords:  50,
			},
			wantProfile:  "screenshot",
			wantMaxWords: 50,
		},
		{
			name: "Regional Essay - Uses essay profile",
			req: GenerationRequest{
				Type:     model.NarrativeTypeEssay,
				Prompt:   "History of the region.",
				MaxWords: 400,
			},
			wantProfile:  "essay",
			wantMaxWords: 400,
		},
		{
			name: "Flight Debrief - Uses debrief profile",
			req: GenerationRequest{
				Type:     model.NarrativeTypeDebriefing,
				Prompt:   "Summarize the flight.",
				MaxWords: 200,
			},
			wantProfile:  "debriefing",
			wantMaxWords: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedProfile string
			mockLLM := &MockLLM{
				Response: "TITLE: Mock Title\nMock narration content.",
				GenerateTextFunc: func(ctx context.Context, name, prompt string) (string, error) {
					capturedProfile = name
					return "TITLE: Mock Title\nMock narration content.", nil
				},
				GenerateImageTextFunc: func(ctx context.Context, name, prompt, imagePath string) (string, error) {
					capturedProfile = name
					return "TITLE: Mock Title\nMock narration content.", nil
				},
			}

			svc := &AIService{
				cfg:        cfg,
				llm:        mockLLM,
				tts:        mockTTS,
				st:         &MockStore{},
				sim:        &MockSim{},
				prompts:    &prompts.Manager{},
				sessionMgr: session.NewManager(nil),
				running:    true,
			}
			svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil, nil, nil)

			narrative, err := svc.GenerateNarrative(context.Background(), &tt.req)
			if err != nil {
				t.Fatalf("GenerateNarrative failed: %v", err)
			}

			if capturedProfile != tt.wantProfile {
				t.Errorf("GenerateNarrative used profile %q, want %q", capturedProfile, tt.wantProfile)
			}

			if narrative.RequestedWords != tt.wantMaxWords {
				t.Errorf("Narrative.RequestedWords = %d, want %d", narrative.RequestedWords, tt.wantMaxWords)
			}

			// Verify script doesn't contain asterisks (markdown stripping)
			if strings.Contains(narrative.Script, "*") {
				t.Errorf("Narrative.Script contains asterisks: %q", narrative.Script)
			}
		})
	}
}

func TestAIService_GenerateNarrative_RescueAvoidance(t *testing.T) {
	// Setup minimalist service
	cfg := config.NewProvider(&config.Config{}, nil)
	mockTTS := &MockTTS{Format: "mp3"}

	// Setup Prompts
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "context")
	_ = os.MkdirAll(promptsDir, 0o755)
	_ = os.WriteFile(filepath.Join(promptsDir, "rescue_script.tmpl"), []byte("Rescue this: {{.Script}}"), 0o644)
	_ = os.WriteFile(filepath.Join(promptsDir, "second_pass.tmpl"), []byte("Refined: {{.Script}}"), 0o644)
	pm, _ := prompts.NewManager(tmpDir)

	t.Run("No Rescue when within buffer", func(t *testing.T) {
		// Target 50, response 60 (well within +30% buffer)
		longResponse := "This is a response that has exactly twelve words in it now." // 12 words
		mockLLM := &MockLLM{
			Response: "TITLE: Title\n" + longResponse,
		}
		svc := &AIService{
			cfg:        cfg,
			llm:        mockLLM,
			tts:        mockTTS,
			prompts:    pm,
			st:         &MockStore{},
			sim:        &MockSim{},
			sessionMgr: session.NewManager(nil),
			running:    true,
		}
		svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil, nil, nil)

		req := &GenerationRequest{
			Type:     model.NarrativeTypePOI,
			MaxWords: 10, // 10 + 30% = 13. 12 words < 13 words.
			Prompt:   "Prompt",
		}

		_, err := svc.GenerateNarrative(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		if mockLLM.GenerateTextCalls != 1 {
			t.Errorf("Expected 1 LLM call, got %d (rescue triggered unnecessarily?)", mockLLM.GenerateTextCalls)
		}
	})

	t.Run("Rescue triggered when significantly over", func(t *testing.T) {
		// Target 10, response 20 (+100% over)
		veryLongResponse := "This is a much longer response that definitely exceeds the ten word limit plus the thirty percent buffer that we have." // 22 words
		mockLLM := &MockLLM{
			Response: "TITLE: Title\n" + veryLongResponse,
			GenerateTextFunc: func(ctx context.Context, name, prompt string) (string, error) {
				if name == "script_rescue" {
					return "TITLE: Title\nShortened response.", nil
				}
				return "TITLE: Title\n" + veryLongResponse, nil
			},
		}
		svc := &AIService{
			cfg:        cfg,
			llm:        mockLLM,
			tts:        mockTTS,
			prompts:    pm,
			st:         &MockStore{},
			sim:        &MockSim{},
			sessionMgr: session.NewManager(nil),
			running:    true,
		}
		svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil, nil, nil)

		req := &GenerationRequest{
			Type:     model.NarrativeTypePOI,
			MaxWords: 10,
			Prompt:   "Prompt",
		}

		narrative, err := svc.GenerateNarrative(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		if mockLLM.GenerateTextCalls < 2 {
			t.Errorf("Expected at least 2 LLM calls (initial + rescue), got %d", mockLLM.GenerateTextCalls)
		}

		if narrative.Script != "Shortened response." {
			t.Errorf("Expected rescued script \"Shortened response.\", got %q", narrative.Script)
		}
		if narrative.Title != "Title" {
			t.Errorf("Expected rescued title \"Title\", got %q", narrative.Title)
		}
	})

	t.Run("TwoPass Excludes Rescue", func(t *testing.T) {
		// Even if script is long, rescue should be skipped if TwoPass is enabled
		veryLongResponse := "This is a much longer response that definitely exceeds the ten word limit plus the thirty percent buffer that we have." // 22 words

		mockLLM := &MockLLM{
			GenerateTextFunc: func(ctx context.Context, name, prompt string) (string, error) {
				// 1. Initial Generation (narration)
				if name == "narration" {
					return "TITLE: Title\n" + veryLongResponse, nil
				}

				// 2. Second Pass or Rescue?
				// Rescue template has "Rescue this:"
				if strings.Contains(prompt, "Rescue this:") {
					if name != "script_rescue" {
						return "", fmt.Errorf("expected script_rescue profile for rescue")
					}
					return "", context.DeadlineExceeded // Should NOT happen
				}

				// Second Pass (must also use script_rescue)
				if name != "script_rescue" {
					return "", fmt.Errorf("expected script_rescue profile for second pass, got %s", name)
				}
				return "Refined script result", nil
			},
		}

		svc := &AIService{
			cfg:        cfg,
			llm:        mockLLM,
			tts:        mockTTS,
			prompts:    pm,
			st:         &MockStore{},
			sim:        &MockSim{},
			sessionMgr: session.NewManager(nil),
			running:    true,
		}
		svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil, nil, nil)

		req := &GenerationRequest{
			Type:     model.NarrativeTypePOI,
			MaxWords: 10,
			Prompt:   "Prompt",
			TwoPass:  true, // ENABLED
		}

		narrative, err := svc.GenerateNarrative(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		// Expect 2-pass result, NOT rescue result (and definitely not an error from the rescue trap)
		if narrative.Script != "Refined script result" {
			t.Errorf("Expected 2-pass script 'Refined script result', got %q", narrative.Script)
		}
	})
}

func TestAIService_GenerateNarrative_GarbageRetry(t *testing.T) {
	cfg := config.NewProvider(&config.Config{}, nil)
	mockTTS := &MockTTS{Format: "mp3"}
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "context")
	_ = os.MkdirAll(promptsDir, 0o755)
	_ = os.WriteFile(filepath.Join(promptsDir, "second_pass.tmpl"), []byte("Refined: {{.Script}}"), 0o644)
	pm, _ := prompts.NewManager(tmpDir)

	garbageResponse := strings.Repeat("garbage ", 500) // 500 words, exceeds target (10) * 2 and 400 min threshold

	var providersCalled []string
	mockLLM := &MockLLM{
		GenerateTextFunc: func(ctx context.Context, name, prompt string) (string, error) {
			// Record "current" provider (simulated by label or absence of skip)
			pLabel, _ := ctx.Value(request.CtxProviderLabel).(string)
			providersCalled = append(providersCalled, pLabel)

			if name == "narration" {
				return garbageResponse, nil
			}

			if name != "script_rescue" {
				return "", fmt.Errorf("expected script_rescue profile, got %s", name)
			}

			// For second_pass (using script_rescue profile)
			if excluded, ok := ctx.Value(request.CtxExcludedProviders).([]string); ok && len(excluded) > 0 {
				// This is the retry call
				return "Clean refined script", nil
			}

			// First attempt at second pass returns EVEN MOAR garbage
			return strings.Repeat("garbage ", 1000), nil
		},
	}

	svc := &AIService{
		cfg:        cfg,
		llm:        mockLLM,
		tts:        mockTTS,
		prompts:    pm,
		st:         &MockStore{},
		sim:        &MockSim{},
		sessionMgr: session.NewManager(nil),
		running:    true,
	}
	svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil, nil, nil)

	// Inject provider label into context to simulate failover reporting current provider
	ctx := context.WithValue(context.Background(), request.CtxProviderLabel, "p1")

	req := &GenerationRequest{
		Type:     model.NarrativeTypePOI,
		MaxWords: 10,
		Prompt:   "Prompt",
		TwoPass:  true,
	}

	narrative, err := svc.GenerateNarrative(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	if narrative.Script != "Clean refined script" {
		t.Errorf("Expected cleaned script on retry, got %q", narrative.Script)
	}

	// We expect 3 calls:
	// 1. Initial narration (p1) -> garbage
	// 2. Second pass attempt 1 (p1) -> garbage
	// 3. Second pass attempt 2 (p2 - simulated by ctx exclusion) -> clean
	if mockLLM.GenerateTextCalls != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", mockLLM.GenerateTextCalls)
	}
}
