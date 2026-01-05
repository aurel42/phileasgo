package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCategories(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "categories.yaml")

	// Create a sample YAML file matching the structure we use
	yamlContent := `
categories:
  Aerodrome:
    weight: 1.7
    icon: "airfield"
    size: "L"
    qids:
      "Q62447": "aerodrome"
      "Q12345": "test_obj"
ignored_categories:
  "Q56061": "Administrative Territorial Entity"
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0o644)
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}

	// Load it
	cfg, err := LoadCategories(configPath)
	if err != nil {
		t.Fatalf("LoadCategories failed: %v", err)
	}

	// Verify Aerodrome exists
	cat, ok := cfg.Categories["aerodrome"] // Keys are normalized to lowercase
	if !ok {
		t.Fatalf("Category 'aerodrome' not found in loaded config (keys should be lowered)")
	}

	// Verify QIDs are loaded (This is what failed before!)
	if len(cat.QIDs) == 0 {
		t.Errorf("Critical: QIDs map is empty! Struct tags match failed.")
	}

	if _, ok := cat.QIDs["Q62447"]; !ok {
		t.Errorf("Expected Q62447 in QIDs, not found")
	}
}
