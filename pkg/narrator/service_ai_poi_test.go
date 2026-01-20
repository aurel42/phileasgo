package narrator

import (
	"context"
	"os"
	"phileasgo/pkg/llm/prompts"
	"strings"
	"testing"
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
