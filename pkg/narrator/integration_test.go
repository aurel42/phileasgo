package narrator_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/prompt"
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

	data := prompt.Data{
		"TourGuideName":        "Ava",
		"Persona":              "Intelligent",
		"Accent":               "Neutral",
		"Language":             "en",
		"Language_code":        "en",
		"Language_name":        "English",
		"Language_region_code": "en-US",
		"FemalePersona":        "Intelligent",
		"FemaleAccent":         "Neutral",
		"PassengerMale":        "Andrew",
		"MalePersona":          "Curious",
		"MaleAccent":           "Neutral",
		"FlightStage":          "Cruise",
		"POINameNative":        "Paris",
		"POINameUser":          "Paris",
		"NameNative":           "Paris",
		"NameUser":             "Paris",
		"Name":                 "Paris",
		"Category":             "City",
		"WikipediaText":        "Text",
		"ArticleURL":           "http://example.com",
		"TargetLanguage":       "en",
		"Country":              "France",
		"Region":               "Ile-de-France",
		"AltitudeAGL":          5000.0,
		"GroundSpeed":          120.0,
		"ActiveStyle":          "Informative",
		"ActiveSecretWord":     "Phileas",
		"Avoid":                []string{"Politics"},
		"IsOnGround":           false,
		"City":                 "Paris",
		"Heading":              180.0,
		"Lat":                  10.0,
		"Lon":                  20.0,
		"TopicName":            "Local History",
		"TopicDescription":     "Description",
		"DistKm":               10.0,
		"DistNm":               5.4,
		"Bearing":              180.0,
		"RelBearing":           0.0,
		"CardinalDir":          "South",
		"UnitsInstruction":     "Use metric units.",
		"Movement":             "Flying North",
		"NavInstruction":       "10km ahead",
		"ClockPos":             12,
		"RelativeDir":          "ahead",
		"UnitSystem":           "metric",
		"IsStub":               false,
		"PregroundContext":     "Notes",
		"TTSInstructions":      "Speak clearly.",
		"LastSentence":         "Hello.",
		"TripSummary":          "Summary.",
		"TargetRegion":         "Ile-de-France",
		"TargetCountry":        "France",
		"MaxWords":             150,
		"RecentContext":        "None",
		"Script":               "Narrative text.",
		"Interests":            []string{"Aviation"},
		"Images":               []prompt.ImageResult{{Title: "Img", URL: "url"}},
		"CategoryList":         "Airport",
		"DomStrat":             "Uniform",
		"From":                 "France",
		"To":                   "Germany",
		"NarrativeType":        "script",
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
