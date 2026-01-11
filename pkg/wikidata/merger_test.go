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
	// Setup Config with Groups and Sizes
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"mountain":  {Size: "XL"},
			"base":      {Size: "L"},
			"shop":      {Size: "S"},
			"city":      {Size: "XL"},
			"aerodrome": {Size: "L"},
		},
		CategoryGroups: map[string][]string{
			"Settlements": {"city"},
			"Aerodromes":  {"aerodrome"},
			"Natural":     {"mountain", "base"},
		},
		MergeDistance: map[string]float64{
			"S":  100,
			"L":  2000,
			"XL": 5000,
		},
	}
	// Manual Group Lookup Setup
	cfg.GroupLookup = map[string]string{
		"mountain":  "Natural",
		"base":      "Natural",
		"city":      "Settlements",
		"aerodrome": "Aerodromes",
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
			name: "Same Group - Close Mountain Gobbles Base",
			// Mountain (XL, 5km radius) vs Base (L, 2km radius). Both 'Natural'.
			// Distance is 1km (within 5km). Mountain has longer article.
			candidates: []*model.POI{
				{WikidataID: "Base", Category: "base", Lat: 0.01, Lon: 0, WPArticleLength: 200},  // ~1.1km
				{WikidataID: "Mtn", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 1000}, // Center
			},
			expected: []string{"Mtn"},
		},
		{
			name: "Different Groups - ISOLATION (City vs Aerodrome)",
			// City (Settlements, XL=5km) vs Aerodrome (Aerodromes, L=2km).
			// Dist 1.1km. Without isolation, City gobbles Aerodrome.
			// With isolation, both kept.
			candidates: []*model.POI{
				{WikidataID: "City", Category: "city", Lat: 0, Lon: 0, WPArticleLength: 1000},
				{WikidataID: "Airport", Category: "aerodrome", Lat: 0.01, Lon: 0, WPArticleLength: 500},
			},
			expected: []string{"City", "Airport"},
		},
		{
			name: "Different Groups - ISOLATION (Natural vs Settlements)",
			// Mountain (Natural, XL=5km) vs City (Settlements, XL=5km).
			// Same Location. Both kept.
			candidates: []*model.POI{
				{WikidataID: "Mtn", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 1000},
				{WikidataID: "City", Category: "city", Lat: 0, Lon: 0, WPArticleLength: 1000},
			},
			expected: []string{"City", "Mtn"},
		},
		{
			name: "Far Apart - Keep Both",
			// Distance 10km (> 5km).
			candidates: []*model.POI{
				{WikidataID: "Mtn", Category: "mountain", Lat: 0, Lon: 0, WPArticleLength: 1000},
				{WikidataID: "Shop", Category: "shop", Lat: 0.1, Lon: 0, WPArticleLength: 100}, // ~11km
			},
			expected: []string{"Mtn", "Shop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergePOIs(tt.candidates, cfg, logger)
			var gotQIDs []string
			for _, p := range result {
				gotQIDs = append(gotQIDs, p.WikidataID)
			}
			sort.Strings(tt.expected)
			sort.Strings(gotQIDs)

			if !reflect.DeepEqual(tt.expected, gotQIDs) {
				t.Errorf("Expected %v, got %v", tt.expected, gotQIDs)
			}
		})
	}
}

func TestMergeArticles(t *testing.T) {
	// Setup Config with Groups and Sizes
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"city":     {Size: "XL"},
			"monument": {Size: "M"},
			"lake":     {Size: "L"},
			"park":     {Size: "M"},
			"mountain": {Size: "XL"},
		},
		CategoryGroups: map[string][]string{
			"Settlements": {"city"},
			"Attractions": {"monument"},
			"Natural":     {"lake", "park", "mountain"},
		},
		MergeDistance: map[string]float64{
			"S":  100,
			"M":  500,
			"L":  2000,
			"XL": 5000,
		},
	}
	cfg.GroupLookup = make(map[string]string)
	for grp, cats := range cfg.CategoryGroups {
		for _, c := range cats {
			cfg.GroupLookup[c] = grp
		}
	}

	logger := slog.Default()

	tests := []struct {
		name       string
		candidates []Article
		expected   []string // Expected QIDs
	}{
		{
			name: "Same Group - Merge (Lake gobbles Park)",
			// Lake (Natural) vs Park (Natural). Dist 1.1km. Link win.
			candidates: []Article{
				{QID: "Park", Category: "park", Lat: 0.01, Lon: 0, Sitelinks: 100},
				{QID: "Lake", Category: "lake", Lat: 0, Lon: 0, Sitelinks: 500},
			},
			expected: []string{"Lake"},
		},
		{
			name: "Different Groups - ISOLATION (City vs Monument)",
			// Settlements vs Attractions. Same Location. Both kept.
			candidates: []Article{
				{QID: "City", Category: "city", Lat: 0, Lon: 0, Sitelinks: 1000},
				{QID: "Monu", Category: "monument", Lat: 0, Lon: 0, Sitelinks: 500},
			},
			expected: []string{"City", "Monu"},
		},
		{
			name: "Different Groups - ISOLATION (City vs Mountain)",
			// Settlements vs Natural. Dist 1km. Both kept.
			candidates: []Article{
				{QID: "City", Category: "city", Lat: 0, Lon: 0, Sitelinks: 1000},
				{QID: "Mtn", Category: "mountain", Lat: 0.01, Lon: 0, Sitelinks: 800},
			},
			expected: []string{"City", "Mtn"},
		},
		{
			name: "Same Group - Too Far (Keep Both)",
			candidates: []Article{
				{QID: "Lake1", Category: "lake", Lat: 0, Lon: 0, Sitelinks: 500},
				{QID: "Lake2", Category: "lake", Lat: 0.1, Lon: 0, Sitelinks: 400},
			},
			expected: []string{"Lake1", "Lake2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeArticles(tt.candidates, cfg, logger)
			var gotQIDs []string
			for _, a := range result {
				gotQIDs = append(gotQIDs, a.QID)
			}
			sort.Strings(tt.expected)
			sort.Strings(gotQIDs)

			if !reflect.DeepEqual(tt.expected, gotQIDs) {
				t.Errorf("Expected %v, got %v", tt.expected, gotQIDs)
			}
		})
	}
}
