package geo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeatureService_GetFeaturesAtPoint(t *testing.T) {
	// 1. Setup temporary GeoJSON for testing
	// We'll use a small GeoJSON with one feature (The Alps)
	tmpDir, err := os.MkdirTemp("", "geo-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	geoJsonPath := filepath.Join(tmpDir, "test.geojson")
	geoJsonContent := `{
	  "type": "FeatureCollection",
	  "features": [
		{
		  "type": "Feature",
		  "properties": {
			"name": "The Alps",
			"qid": "Q1286",
			"category": "mountain_range"
		  },
		  "geometry": {
			"type": "Polygon",
			"coordinates": [
			  [
				[5.0, 45.0],
				[15.0, 45.0],
				[15.0, 48.0],
				[5.0, 48.0],
				[5.0, 45.0]
			  ]
			]
		  }
		}
	  ]
	}`
	if err := os.WriteFile(geoJsonPath, []byte(geoJsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Initialize Service
	svc, err := NewFeatureService(geoJsonPath)
	if err != nil {
		t.Fatalf("Failed to create FeatureService: %v", err)
	}

	// 3. Test Point inside (Innsbruck-ish: 47.2, 11.4)
	results := svc.GetFeaturesAtPoint(47.2, 11.4)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	} else {
		if results[0].Name != "The Alps" {
			t.Errorf("Expected 'The Alps', got '%s'", results[0].Name)
		}
		if results[0].QID != "Q1286" {
			t.Errorf("Expected 'Q1286', got '%s'", results[0].QID)
		}
	}

	// 4. Test Point outside (London: 51.5, -0.1)
	results = svc.GetFeaturesAtPoint(51.5, -0.1)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for point outside, got %d", len(results))
	}
}
