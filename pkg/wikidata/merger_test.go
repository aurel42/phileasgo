package wikidata

import (
	"log/slog"
	"reflect"
	"sort"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
)

func TestMergePOIs(t *testing.T) {
	// Setup Config with Merge Distances
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"mountain": {Size: "XL"},
			"base":     {Size: "L"},
			"shop":     {Size: "S"},
		},
		MergeDistance: map[string]float64{
			"S":  100,
			"L":  2000,
			"XL": 5000,
		},
	}

	logger := slog.Default()

	tests := []struct {
		name       string
		candidates []*model.POI
		expected   []string // Expected QIDs
	}{
		{
			name:       "No Candidates",
			candidates: nil,
			expected:   nil,
		},
		{
			name: "Single Candidate",
			candidates: []*model.POI{
				{WikidataID: "Q1", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 100},
			},
			expected: []string{"Q1"},
		},
		{
			name: "Close Mountain Gobbles Base",
			// Mountain (XL, 5km radius) is close to Base (L, 2km radius).
			// Mountain has longer article (1000 vs 200).
			// Distance is 1km (within 5km).
			// Expect: Mountain kept.
			candidates: []*model.POI{
				{WikidataID: "Base", Category: "base", Lat: 0.01, Lon: 0, WPArticleLength: 200},  // ~1.1km away
				{WikidataID: "Mtn", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 1000}, // Center
			},
			expected: []string{"Mtn"},
		},
		{
			name: "Base Gobbles Mountain (if Base is Huge)",
			// If the Base has a massive article, it should win even if Mountain is "XL" sized?
			// The logic sorts by Length first.
			// Base (Len 2000) vs Mountain (Len 100).
			// Base wins sort. Base radius (L=2km) vs Mtn Radius (XL=5km). Max=5km.
			// Distance 1km.
			// Expect: Base kept.
			candidates: []*model.POI{
				{WikidataID: "Base", Category: "base", Lat: 0.01, Lon: 0, WPArticleLength: 2000},
				{WikidataID: "Mtn", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 100},
			},
			expected: []string{"Base"},
		},
		{
			name: "Far Apart - Keep Both",
			// Distance 10km (> 5km).
			candidates: []*model.POI{
				{WikidataID: "Mtn", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 1000},
				{WikidataID: "Shop", Category: "shop", Lat: 0.1, Lon: 0, WPArticleLength: 100}, // ~11km
			},
			expected: []string{"Mtn", "Shop"}, // Order depends on sort (Mtn first)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergePOIs(tt.candidates, cfg, logger)
			var gotQIDs []string
			for _, p := range result {
				gotQIDs = append(gotQIDs, p.WikidataID)
			}
			// Sort for deterministic comparison if needed, but ElementsMatch handles generic "bag"
			// Here we just sort both constraints
			sort.Strings(tt.expected)
			sort.Strings(gotQIDs)

			if !reflect.DeepEqual(tt.expected, gotQIDs) {
				t.Errorf("Expected %v, got %v", tt.expected, gotQIDs)
			}
		})
	}
}
