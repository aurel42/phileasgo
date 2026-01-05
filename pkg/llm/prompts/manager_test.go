package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManager_Render(t *testing.T) {
	// Setup temporary directory structure
	tmpDir := t.TempDir()

	commonDir := filepath.Join(tmpDir, "common")
	narratorDir := filepath.Join(tmpDir, "narrator")

	// Write common/macros.tmpl
	macrosContent := `{{define "hello"}}Hello {{.Name}}{{end}}`
	if err := writeFile(filepath.Join(commonDir, "macros.tmpl"), macrosContent); err != nil {
		t.Fatal(err)
	}

	// Write narrator/script.tmpl that uses macro
	scriptContent := `{{template "hello" .}}! How are you?`
	if err := writeFile(filepath.Join(narratorDir, "script.tmpl"), scriptContent); err != nil {
		t.Fatal(err)
	}

	// Initialize Manager
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Render
	data := struct{ Name string }{Name: "World"}
	out, err := m.Render("narrator/script.tmpl", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Hello World! How are you?"
	if out != expected {
		t.Errorf("Expected %q, got %q", expected, out)
	}
}

func TestManager_Category(t *testing.T) {
	tmpDir := t.TempDir()

	categoryDir := filepath.Join(tmpDir, "category")

	// Write category/aerodrome.tmpl
	aerodromeContent := `Airport: {{.Name}}`
	if err := writeFile(filepath.Join(categoryDir, "aerodrome.tmpl"), aerodromeContent); err != nil {
		t.Fatal(err)
	}

	// Write main template using category macro
	mainContent := `Category: {{.Cat}}
{{category .Cat .}}`
	if err := writeFile(filepath.Join(tmpDir, "main.tmpl"), mainContent); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	tests := []struct {
		name     string
		cat      string
		expected string
	}{
		{
			name:     "Known Category",
			cat:      "aerodrome",
			expected: "Category: aerodrome\nAirport: Test",
		},
		{
			name:     "Case Insensitive",
			cat:      "AERODROME",
			expected: "Category: AERODROME\nAirport: Test",
		},
		{
			name:     "Unknown Category",
			cat:      "temple",
			expected: "Category: temple\n",
		},
		{
			name:     "Empty Category",
			cat:      "",
			expected: "Category: \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := struct {
				Cat  string
				Name string
			}{Cat: tt.cat, Name: "Test"}
			out, err := m.Render("main.tmpl", data)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}
			if out != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, out)
			}
		})
	}
}

func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestInterestsFunc(t *testing.T) {
	// Test basic functionality with 4 interests (should return 2 after excluding 2)
	interests := []string{"History", "Aviation", "Engineering", "Pop Culture"}
	result := interestsFunc(interests)

	// Count how many of the original interests are present
	presentCount := 0
	for _, item := range interests {
		if strings.Contains(result, item) {
			presentCount++
		}
	}

	// Should have exactly 2 interests remaining (4 - 2 excluded)
	expectedCount := 2
	if presentCount != expectedCount {
		t.Errorf("Expected %d interests in result, got %d (result: %q)", expectedCount, presentCount, result)
	}

	// Verify comma-separated format (1 comma for 2 items)
	if !strings.Contains(result, ", ") {
		t.Errorf("Expected comma-separated output, got %q", result)
	}

	// Verify count of items (by counting commas): 2 items = 1 comma
	commas := strings.Count(result, ", ")
	if commas != expectedCount-1 {
		t.Errorf("Expected %d commas, got %d", expectedCount-1, commas)
	}
}

func TestInterestsFunc_Empty(t *testing.T) {
	result := interestsFunc([]string{})
	if result != "" {
		t.Errorf("Expected empty string for empty input, got %q", result)
	}
}

func TestInterestsFunc_Shuffles(t *testing.T) {
	// Run multiple times and verify that at least once the order or set differs
	interests := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}

	// With 10 interests and 2 excluded, we should get 8 items each time
	// The result should vary in both order and which items are included
	seenResults := make(map[string]bool)
	for i := 0; i < 20; i++ {
		result := interestsFunc(interests)
		seenResults[result] = true
	}

	// We should see multiple different results due to shuffling and random exclusion
	if len(seenResults) < 2 {
		t.Error("interestsFunc should produce varying results due to shuffle and exclusion")
	}
}

func TestMaybeFunc(t *testing.T) {
	// Test 0% probability - should never include
	for i := 0; i < 10; i++ {
		if maybeFunc(0, "content") != "" {
			t.Error("0% probability should never include content")
		}
	}

	// Test 100% probability - should always include
	for i := 0; i < 10; i++ {
		if maybeFunc(100, "content") != "content" {
			t.Error("100% probability should always include content")
		}
	}

	// Test 50% probability - should vary
	included := 0
	for i := 0; i < 100; i++ {
		if maybeFunc(50, "content") == "content" {
			included++
		}
	}
	// Should be roughly 50%, allow wide margin (20-80)
	if included < 20 || included > 80 {
		t.Errorf("50%% probability should include ~50 times, got %d", included)
	}
}

func TestPickFunc(t *testing.T) {
	// Test single option
	result := pickFunc("only option")
	if result != "only option" {
		t.Errorf("Single option should return that option, got %q", result)
	}

	// Test multiple options - should vary
	seenResults := make(map[string]bool)
	for i := 0; i < 50; i++ {
		result := pickFunc("A|||B|||C")
		seenResults[result] = true
	}

	// Should have seen all options
	if len(seenResults) < 2 {
		t.Error("pickFunc should produce varying results")
	}

	// Verify options are trimmed
	result = pickFunc("  spaced  |||  option  ")
	if result != "spaced" && result != "option" {
		t.Errorf("Options should be trimmed, got %q", result)
	}
}

// TestProductionTemplates verifies that the actual production templates parse correctly.
// This catches issues like using template syntax inside function arguments.
func TestProductionTemplates(t *testing.T) {
	// Skip if configs/prompts doesn't exist (e.g., in CI without full repo)
	promptsDir := "../../../configs/prompts"
	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		t.Skip("configs/prompts not found, skipping production template test")
	}

	_, err := NewManager(promptsDir)
	if err != nil {
		t.Fatalf("Failed to load production templates: %v", err)
	}
}
