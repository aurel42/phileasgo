package narrator

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"testing"
)

func TestAIService_AnnouncementFallback(t *testing.T) {
	pm, _ := prompts.NewManager(t.TempDir())
	cfg := config.NewProvider(&config.Config{
		Narrator: config.NarratorConfig{
			TargetLanguage: "en",
		},
	}, nil)

	tests := []struct {
		name            string
		narrativeType   model.NarrativeType
		definedProfiles []string
		expectedProfile string
	}{
		{
			name:            "Letsgo uses specific profile if available",
			narrativeType:   model.NarrativeTypeLetsgo,
			definedProfiles: []string{"letsgo", "announcements"},
			expectedProfile: "letsgo",
		},
		{
			name:            "Letsgo falls back to announcements",
			narrativeType:   model.NarrativeTypeLetsgo,
			definedProfiles: []string{"announcements"},
			expectedProfile: "announcements",
		},
		{
			name:            "Briefing falls back to announcements",
			narrativeType:   model.NarrativeTypeBriefing,
			definedProfiles: []string{"announcements"},
			expectedProfile: "announcements",
		},
		{
			name:            "Debrief is NOT an announcement (no fallback)",
			narrativeType:   model.NarrativeTypeDebriefing,
			definedProfiles: []string{"announcements"},
			expectedProfile: "debriefing",
		},
		{
			name:            "Screenshot is NOT an announcement (no fallback)",
			narrativeType:   model.NarrativeTypeScreenshot,
			definedProfiles: []string{"announcements"},
			expectedProfile: "screenshot",
		},
		{
			name:            "Border is NOT an announcement (no fallback)",
			narrativeType:   model.NarrativeTypeBorder,
			definedProfiles: []string{"announcements"},
			expectedProfile: "border",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedProfile string
			mockLLM := &MockLLM{
				HasProfileFunc: func(name string) bool {
					for _, p := range tt.definedProfiles {
						if p == name {
							return true
						}
					}
					return false
				},
				GenerateTextFunc: func(ctx context.Context, profile, prompt string) (string, error) {
					capturedProfile = profile
					return "Script", nil
				},
			}

			svc := &AIService{
				cfg:     cfg,
				llm:     mockLLM,
				tts:     &MockTTS{Format: "mp3"},
				prompts: pm,
				st:      &MockStore{},
				sim:     &MockSim{},
				running: true,
			}

			req := &GenerationRequest{
				Type:   tt.narrativeType,
				Prompt: "Test",
			}

			_, _ = svc.GenerateNarrative(context.Background(), req)

			if capturedProfile != tt.expectedProfile {
				t.Errorf("expected profile '%s', got '%s'", tt.expectedProfile, capturedProfile)
			}
		})
	}
}
