package narrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/prompt"
)

func TestEssayHandler_SelectTopic(t *testing.T) {
	// 1. Create temporary essays.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "essays.yaml")
	configContent := `
topics:
  - id: "t1"
    name: "Topic 1"
    description: "Desc 1"
    max_words: 100
  - id: "t2"
    name: "Topic 2"
    description: "Desc 2"
    max_words: 200
  - id: "t3"
    name: "Topic 3"
    description: "Desc 3"
    max_words: 300
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write mock config: %v", err)
	}

	// 2. Setup Prompt Manager (needed for NewEssayHandler but unused in SelectTopic)
	pm, _ := prompts.NewManager(tmpDir)

	// 3. Init Handler
	eh, err := NewEssayHandler(configPath, pm)
	if err != nil {
		t.Fatalf("Failed to create essay handler: %v", err)
	}

	// 4. Test Selection & Repetition
	// We have 3 topics.
	// Initial state: availablePool is empty.

	// Pick 1
	t1, err := eh.SelectTopic()
	if err != nil {
		t.Fatalf("SelectTopic 1 failed: %v", err)
	}
	// Pool should have been refilled (3) and one removed -> 2
	if len(eh.availablePool) != 2 {
		t.Errorf("Expected pool size 2 after first pick, got %d", len(eh.availablePool))
	}

	// Pick 2
	t2, err := eh.SelectTopic()
	if err != nil {
		t.Fatalf("SelectTopic 2 failed: %v", err)
	}
	if t2.ID == t1.ID {
		t.Errorf("Expected different topic, got %s twice", t1.ID)
	}
	if len(eh.availablePool) != 1 {
		t.Errorf("Expected pool size 1 after second pick, got %d", len(eh.availablePool))
	}

	// Pick 3
	t3, err := eh.SelectTopic()
	if err != nil {
		t.Fatalf("SelectTopic 3 failed: %v", err)
	}
	if t3.ID == t1.ID || t3.ID == t2.ID {
		t.Errorf("Expected unique topic, got %s (seen: %s, %s)", t3.ID, t1.ID, t2.ID)
	}
	if len(eh.availablePool) != 0 {
		t.Errorf("Expected pool size 0 after third pick, got %d", len(eh.availablePool))
	}

	// Pick 4 -> Pool size was 0. Should refill to 3 and pick 1 -> 2 remaining.
	t4, err := eh.SelectTopic()
	if err != nil {
		t.Fatalf("SelectTopic 4 failed: %v", err)
	}
	if t4 == nil {
		t.Error("Got nil topic on exhaust")
	}
	if len(eh.availablePool) != 2 {
		t.Errorf("Expected pool size 2 after refill and pick, got %d", len(eh.availablePool))
	}
}

func TestEssayHandler_BuildPrompt(t *testing.T) {
	// 1. Create temp dir with config and template
	tmpDir := t.TempDir()

	// Config
	configPath := filepath.Join(tmpDir, "essays.yaml")
	_ = os.WriteFile(configPath, []byte("topics: []"), 0o644)

	// Template
	tmplDir := filepath.Join(tmpDir, "narrator")
	_ = os.MkdirAll(tmplDir, 0o755)
	tmplPath := filepath.Join(tmplDir, "essay.tmpl")
	tmplContent := `Topic: {{.TopicName}}
Context: {{.TargetCountry}}
Words: {{.MaxWords}}`
	_ = os.WriteFile(tmplPath, []byte(tmplContent), 0o644)

	// dummy common template to allow loading
	_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)

	pm, err := prompts.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to init prompt manager: %v", err)
	}

	eh, err := NewEssayHandler(configPath, pm)
	if err != nil {
		t.Fatalf("NewEssayHandler failed: %v", err)
	}

	topic := &EssayTopic{
		ID:       "t1",
		Name:     "Geography",
		MaxWords: 50,
	}

	pd := prompt.Data{
		"TargetCountry": "France",
		"Lat":           48.85,
		"Lon":           2.35,
	}

	// Render
	res, err := eh.BuildPrompt(context.Background(), topic, &pd)
	if err != nil {
		t.Fatalf("BuildPrompt failed: %v", err)
	}

	expected := []string{"Topic: Geography", "Context: France", "Words: 50"}
	for _, exp := range expected {
		if !strings.Contains(res, exp) {
			t.Errorf("Prompt missing %q. Got:\n%s", exp, res)
		}
	}
}

// TestEssayHandler_BuildPrompt_CoordinatesPassthrough tests that Lat/Lon from
// NarrationPromptData are correctly passed through to the template without being
// shadowed by zero values. This test would have caught the coordinate regression
// where anonymous struct fields shadowed the embedded struct fields.
func TestEssayHandler_BuildPrompt_CoordinatesPassthrough(t *testing.T) {
	tmpDir := t.TempDir()

	// Config
	configPath := filepath.Join(tmpDir, "essays.yaml")
	_ = os.WriteFile(configPath, []byte("topics: []"), 0o644)

	// Template that renders coordinates
	tmplDir := filepath.Join(tmpDir, "narrator")
	_ = os.MkdirAll(tmplDir, 0o755)
	tmplPath := filepath.Join(tmplDir, "essay.tmpl")
	tmplContent := `Coordinates: {{.Lat}}, {{.Lon}}`
	_ = os.WriteFile(tmplPath, []byte(tmplContent), 0o644)

	_ = os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755)

	pm, err := prompts.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to init prompt manager: %v", err)
	}

	eh, err := NewEssayHandler(configPath, pm)
	if err != nil {
		t.Fatalf("NewEssayHandler failed: %v", err)
	}

	topic := &EssayTopic{ID: "t1", Name: "Test", MaxWords: 50}
	pd := prompt.Data{
		"Lat": 48.8566,
		"Lon": 2.3522,
	}

	res, err := eh.BuildPrompt(context.Background(), topic, &pd)
	if err != nil {
		t.Fatalf("BuildPrompt failed: %v", err)
	}

	// Critical assertion: coordinates should NOT be zero
	if strings.Contains(res, "0, 0") {
		t.Errorf("Coordinates were rendered as 0, 0 - indicates regression where Lat/Lon fields shadow embedded struct. Got: %s", res)
	}

	// Verify actual coordinates are present
	if !strings.Contains(res, "48.8566") {
		t.Errorf("Lat not found in rendered prompt. Got: %s", res)
	}
	if !strings.Contains(res, "2.3522") {
		t.Errorf("Lon not found in rendered prompt. Got: %s", res)
	}
}

func TestNewEssayHandler_NotFound(t *testing.T) {
	pm, _ := prompts.NewManager(t.TempDir())
	_, err := NewEssayHandler("non-existent-file.yaml", pm)
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestSelectTopic_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")
	_ = os.WriteFile(configPath, []byte("topics: []"), 0o644)
	pm, _ := prompts.NewManager(tmpDir)

	eh, _ := NewEssayHandler(configPath, pm)
	_, err := eh.SelectTopic()
	if err == nil {
		t.Error("Expected error for empty topics, got nil")
	}
}
