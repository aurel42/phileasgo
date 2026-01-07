package narrator_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/narrator"
)

// TestNarrator_Integration verifies that the actual production templates
// can be rendered with the actual struct used in AIService.
func TestNarrator_Integration(t *testing.T) {
	// Locate project root relative to this test file
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	promptsDir := filepath.Join(projectRoot, "configs", "prompts")

	// Initialize Manager with REAL templates
	pm, err := prompts.NewManager(promptsDir)
	if err != nil {
		t.Fatalf("Failed to load production templates from %s: %v", promptsDir, err)
	}

	data := narrator.NarrationPromptData{
		TourGuideName:        "Ava",
		Persona:              "Intelligent, fascinating",
		Accent:               "Neutral",
		Language:             "en",
		Language_code:        "en",
		Language_name:        "English",
		Language_region_code: "en-US",
		FemalePersona:        "Intelligent, fascinating",
		FemaleAccent:         "English",
		PassengerMale:        "Andrew",
		MalePersona:          "Curious",
		MaleAccent:           "American",
		FlightStage:          "Cruise",
		NameNative:           "Paris",
		POINameNative:        "Paris",
		NameUser:             "Paris",
		POINameUser:          "Paris",
		Category:             "City",
		WikipediaText:        "Paris is the capital of France.",
		NavInstruction:       "10km ahead",
		TargetLanguage:       "en",
		TargetCountry:        "France",
		Country:              "France",
		TargetRegion:         "Ile-de-France",
		Region:               "Ile-de-France",
		MaxWords:             150,
		RecentPoisContext:    "None",
		RecentContext:        "None",
	}

	content, err := pm.Render("narrator/script.tmpl", data)
	if err != nil {
		t.Fatalf("Failed to render production template 'narrator/script.tmpl': %v", err)
	}

	if content == "" {
		t.Error("Rendered content is empty")
	}

	t.Logf("Successfully rendered template. Preview:\n%.100s...", content)
}
