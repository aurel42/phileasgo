package rivers

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"phileasgo/pkg/geo"

	"github.com/paulmach/orb"
)

// quietLogger returns a logger that discards all output.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestNewSentinelWithMissingFile ensures Sentinel doesn't crash on missing file.
func TestNewSentinelWithMissingFile(t *testing.T) {
	s := NewSentinel(quietLogger(), "/nonexistent/path.geojson")

	if s == nil {
		t.Fatal("expected non-nil Sentinel even with missing file")
	}
	if len(s.rivers) != 0 {
		t.Errorf("expected 0 rivers with missing file, got %d", len(s.rivers))
	}
}

// TestLoadDataWithValidGeoJSON tests loading rivers from a valid GeoJSON file.
func TestLoadDataWithValidGeoJSON(t *testing.T) {
	// Create temp file with valid GeoJSON
	geojson := `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"name_en": "Test River", "name": "Testfluss"},
				"geometry": {
					"type": "MultiLineString",
					"coordinates": [[[8.0, 47.0], [8.0, 48.0], [8.0, 49.0]]]
				}
			},
			{
				"type": "Feature",
				"properties": {"name": "Secondary River"},
				"geometry": {
					"type": "LineString",
					"coordinates": [[10.0, 48.0], [11.0, 48.0]]
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "rivers.geojson")
	if err := os.WriteFile(tmpFile, []byte(geojson), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	s := NewSentinel(quietLogger(), tmpFile)
	if s == nil {
		t.Fatal("expected non-nil Sentinel")
	}
	if len(s.rivers) != 2 {
		t.Fatalf("expected 2 rivers, got %d", len(s.rivers))
	}

	// Check first river (should use name_en)
	if s.rivers[0].Name != "Test River" {
		t.Errorf("expected name 'Test River', got %q", s.rivers[0].Name)
	}

	// Check second river (should use name fallback)
	if s.rivers[1].Name != "Secondary River" {
		t.Errorf("expected name 'Secondary River', got %q", s.rivers[1].Name)
	}
}

// TestLoadDataEdgeCases tests various edge cases in loadData.
func TestLoadDataEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		geojson        string
		expectedRivers int
	}{
		{
			name:           "empty feature collection",
			geojson:        `{"type": "FeatureCollection", "features": []}`,
			expectedRivers: 0,
		},
		{
			name: "feature without name",
			geojson: `{"type": "FeatureCollection", "features": [{
				"type": "Feature",
				"properties": {},
				"geometry": {"type": "LineString", "coordinates": [[0,0], [1,1]]}
			}]}`,
			expectedRivers: 0, // skipped due to no name
		},
		{
			name: "unsupported geometry type",
			geojson: `{"type": "FeatureCollection", "features": [{
				"type": "Feature",
				"properties": {"name": "Test"},
				"geometry": {"type": "Point", "coordinates": [0, 0]}
			}]}`,
			expectedRivers: 0, // skipped due to Point type
		},
		{
			name: "empty geometry",
			geojson: `{"type": "FeatureCollection", "features": [{
				"type": "Feature",
				"properties": {"name": "Test"},
				"geometry": {"type": "MultiLineString", "coordinates": []}
			}]}`,
			expectedRivers: 0, // skipped due to empty MLS
		},
		{
			name: "empty linestring in MLS",
			geojson: `{"type": "FeatureCollection", "features": [{
				"type": "Feature",
				"properties": {"name": "Test"},
				"geometry": {"type": "MultiLineString", "coordinates": [[]]}
			}]}`,
			expectedRivers: 0, // skipped due to empty first line
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "rivers.geojson")
			if err := os.WriteFile(tmpFile, []byte(tc.geojson), 0o644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			s := NewSentinel(quietLogger(), tmpFile)
			if len(s.rivers) != tc.expectedRivers {
				t.Errorf("expected %d rivers, got %d", tc.expectedRivers, len(s.rivers))
			}
		})
	}
}

// TestLoadDataWithInvalidJSON tests handling of malformed JSON.
func TestLoadDataWithInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.geojson")
	if err := os.WriteFile(tmpFile, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	s := NewSentinel(quietLogger(), tmpFile)
	if s == nil {
		t.Fatal("expected non-nil Sentinel even with invalid JSON")
	}
	if len(s.rivers) != 0 {
		t.Errorf("expected 0 rivers with invalid JSON, got %d", len(s.rivers))
	}
}

// TestRiverStruct verifies River struct fields.
func TestRiverStruct(t *testing.T) {
	r := River{
		Name:   "Test River",
		Geom:   orb.MultiLineString{{{0, 0}, {1, 1}}},
		Mouth:  geo.Point{Lat: 1, Lon: 1},
		Source: geo.Point{Lat: 0, Lon: 0},
		BBox:   orb.Bound{Min: orb.Point{0, 0}, Max: orb.Point{1, 1}},
	}

	if r.Name != "Test River" {
		t.Errorf("expected name 'Test River', got %q", r.Name)
	}
	if r.Mouth.Lat != 1 || r.Mouth.Lon != 1 {
		t.Errorf("unexpected Mouth: %+v", r.Mouth)
	}
	if r.Source.Lat != 0 || r.Source.Lon != 0 {
		t.Errorf("unexpected Source: %+v", r.Source)
	}
}

// TestCandidateStruct verifies Candidate struct fields.
func TestCandidateStruct(t *testing.T) {
	c := Candidate{
		Name:         "Test River",
		ClosestPoint: geo.Point{Lat: 0.5, Lon: 0.5},
		Distance:     1000,
		IsAhead:      true,
		Mouth:        geo.Point{Lat: 1, Lon: 1},
		Source:       geo.Point{Lat: 0, Lon: 0},
	}

	if c.Name != "Test River" {
		t.Errorf("expected name 'Test River', got %q", c.Name)
	}
	if !c.IsAhead {
		t.Error("expected IsAhead to be true")
	}
	if c.Distance != 1000 {
		t.Errorf("expected distance 1000, got %f", c.Distance)
	}
}

// TestSentinelUpdate tests the Update method with synthetic river data.
// DetectionRadius in sentinel.go is 25km, so we position aircraft within that range.
func TestSentinelUpdate(t *testing.T) {
	// Create a sentinel with manually injected rivers
	s := &Sentinel{
		logger: quietLogger(),
		rivers: []River{
			{
				Name: "Rhine",
				Geom: orb.MultiLineString{
					// River segment running North-South at lon=8
					// orb.Point is [lon, lat]
					{{8.0, 47.0}, {8.0, 48.0}, {8.0, 49.0}, {8.0, 50.0}},
				},
				Mouth:  geo.Point{Lat: 50.0, Lon: 8.0},
				Source: geo.Point{Lat: 47.0, Lon: 8.0},
				BBox:   orb.Bound{Min: orb.Point{7.5, 46.5}, Max: orb.Point{8.5, 50.5}},
			},
			{
				Name: "Danube",
				Geom: orb.MultiLineString{
					// River segment running East-West at lat=48
					{{10.0, 48.0}, {11.0, 48.0}, {12.0, 48.0}},
				},
				Mouth:  geo.Point{Lat: 48.0, Lon: 12.0},
				Source: geo.Point{Lat: 48.0, Lon: 10.0},
				BBox:   orb.Bound{Min: orb.Point{9.5, 47.5}, Max: orb.Point{12.5, 48.5}},
			},
		},
	}

	tests := []struct {
		name           string
		lat, lon       float64
		heading        float64
		expectNil      bool
		expectRiver    string
		expectDistLess float64 // expect distance less than this (meters)
	}{
		{
			name:        "far from any river",
			lat:         40.0,
			lon:         0.0,
			heading:     0,
			expectNil:   true,
			expectRiver: "",
		},
		{
			name:           "near Rhine heading north - within 25km",
			lat:            47.9, // ~11km south of river at lat 48
			lon:            8.0,  // exactly on river longitude
			heading:        0,    // heading north toward closest point on river
			expectNil:      false,
			expectRiver:    "Rhine",
			expectDistLess: 15000, // ~11km
		},
		{
			name:           "near Danube heading north - within 25km",
			lat:            47.9, // ~11km south of river at lat 48
			lon:            11.0, // on river
			heading:        0,    // heading north toward the river
			expectNil:      false,
			expectRiver:    "Danube",
			expectDistLess: 15000,
		},
		{
			name:        "south of Rhine heading away (south)",
			lat:         47.9, // south of river at lat 48
			lon:         8.0,  // on the river longitude
			heading:     180,  // heading south, river is NORTH behind us
			expectNil:   true, // closest point is behind, not ahead
			expectRiver: "",
		},
		{
			name:           "just east of Rhine heading west",
			lat:            48.0,
			lon:            8.05, // ~3.5km east of river
			heading:        270,  // heading west toward the river
			expectNil:      false,
			expectRiver:    "Rhine",
			expectDistLess: 5000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := s.Update(tc.lat, tc.lon, tc.heading)

			// Check if we expect nil candidate
			c, _ := result.(*Candidate)

			if tc.expectNil {
				if c != nil {
					t.Errorf("expected nil candidate, got: %+v", c)
				}
				return
			}

			// We expect a non-nil candidate
			if c == nil {
				t.Fatalf("expected non-nil candidate for %q, got nil", tc.name)
			}

			if tc.expectRiver != "" && c.Name != tc.expectRiver {
				t.Errorf("expected river %q, got %q", tc.expectRiver, c.Name)
			}
			if tc.expectDistLess > 0 && c.Distance >= tc.expectDistLess {
				t.Errorf("expected distance < %f, got %f", tc.expectDistLess, c.Distance)
			}
			if !c.IsAhead {
				t.Error("expected IsAhead to be true")
			}
		})
	}
}

// TestSentinelUpdateEmptyRivers ensures Update handles empty rivers gracefully.
func TestSentinelUpdateEmptyRivers(t *testing.T) {
	s := &Sentinel{
		logger: quietLogger(),
		rivers: []River{},
	}

	result := s.Update(48.0, 8.0, 0)
	c, _ := result.(*Candidate)
	if c != nil {
		t.Errorf("expected nil candidate with no rivers, got: %+v", c)
	}
}

// TestSentinelConcurrency verifies thread-safety under concurrent access.
func TestSentinelConcurrency(t *testing.T) {
	s := &Sentinel{
		logger: quietLogger(),
		rivers: []River{
			{
				Name:   "Test",
				Geom:   orb.MultiLineString{{{0, 0}, {1, 1}}},
				Mouth:  geo.Point{Lat: 1, Lon: 1},
				Source: geo.Point{Lat: 0, Lon: 0},
				BBox:   orb.Bound{Min: orb.Point{-1, -1}, Max: orb.Point{2, 2}},
			},
		},
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.Update(0.5, 0.5, float64(j%360))
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
