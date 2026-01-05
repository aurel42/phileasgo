package narrator

import (
	"os"
	"path/filepath"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
)

func TestAIService_fetchUnitsInstruction(t *testing.T) {
	// Setup Prompts in Temp Dir
	tmpDir := t.TempDir()
	unitsDir := filepath.Join(tmpDir, "units")
	if err := os.MkdirAll(unitsDir, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(unitsDir, "test.tmpl"), []byte("USE TEST UNITS"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	// Create hybrid template (default fallback)
	if err := os.WriteFile(filepath.Join(unitsDir, "hybrid.tmpl"), []byte("USE HYBRID UNITS"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	pm, err := prompts.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	tests := []struct {
		name     string
		cfgUnits string
		want     string
	}{
		{
			name:     "Explicit Test",
			cfgUnits: "test",
			want:     "USE TEST UNITS",
		},
		{
			name:     "Default (Empty)",
			cfgUnits: "",
			want:     "USE HYBRID UNITS",
		},
		{
			name:     "Case Insensitive",
			cfgUnits: "TeSt",
			want:     "USE TEST UNITS",
		},
		{
			name:     "Missing Template",
			cfgUnits: "missing",
			want:     "", // Should fail gracefully (return empty string or error logged)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &AIService{
				cfg: &config.Config{
					Narrator: config.NarratorConfig{
						Units: tt.cfgUnits,
					},
				},
				prompts: pm,
			}

			got := svc.fetchUnitsInstruction()
			if got != tt.want {
				t.Errorf("fetchUnitsInstruction() = %q, want %q", got, tt.want)
			}
		})
	}
}
