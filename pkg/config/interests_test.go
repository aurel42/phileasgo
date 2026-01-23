package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInterests(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "interests.yaml")

	yamlContent := `
interests:
  - "castles"
  - "history"
avoid:
  - "ruins"
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0o644)
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	cfg, err := LoadInterests(configPath)
	if err != nil {
		t.Fatalf("LoadInterests failed: %v", err)
	}

	if len(cfg.Interests) != 2 {
		t.Errorf("Expected 2 interests, got %d", len(cfg.Interests))
	}
	if cfg.Interests[0] != "castles" {
		t.Errorf("Expected first interest 'castles', got '%s'", cfg.Interests[0])
	}
	if len(cfg.Avoid) != 1 {
		t.Errorf("Expected 1 avoid, got %d", len(cfg.Avoid))
	}
	if cfg.Avoid[0] != "ruins" {
		t.Errorf("Expected avoid 'ruins', got '%s'", cfg.Avoid[0])
	}

	// Test missing file
	_, err = LoadInterests(filepath.Join(tempDir, "nonexistent.yaml"))
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}

	// Test invalid YAML
	invalidPath := filepath.Join(tempDir, "invalid.yaml")
	_ = os.WriteFile(invalidPath, []byte("interests: ["), 0o644)
	_, err = LoadInterests(invalidPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}
