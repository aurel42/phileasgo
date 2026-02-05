package narrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
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
			svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil)

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
		svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil)

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
		svc.promptAssembler = prompt.NewAssembler(svc.cfg, svc.st, svc.prompts, svc.geoSvc, svc.wikipedia, svc.poiMgr, svc.llm, svc.categoriesCfg, nil, nil)

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
}
