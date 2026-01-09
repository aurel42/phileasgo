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

func TestMergeArticles(t *testing.T) {
	// Setup Config with Groups and Sizes
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"city":     {Size: "XL"},
			"monument": {Size: "M"},
			"lake":     {Size: "L"},
			"park":     {Size: "M"},
		},
		CategoryGroups: map[string][]string{
			"Settlements": {"city"},
			"Attractions": {"monument"},
			"Natural":     {"lake", "park"},
		},
		MergeDistance: map[string]float64{
			"M":  500,
			"L":  2000,
			"XL": 5000,
		},
	}
	// Rebuild lookup usually happens on load, but we can manually set it or just mock GetGroup behavior via manually populated map if Config allows.
	// Config.GetGroup relies on GroupLookup. We must populate it.
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
			// Lake (Natural) vs Park (Natural).
			// Lake (500 links, L=2km) vs Park (100 links, M=500m).
			// Dist 1km.
			// Lake wins by Sitelinks. Lake Radius covers Park.
			candidates: []Article{
				{QID: "Park", Category: "park", Lat: 0.01, Lon: 0, Sitelinks: 100}, // ~1.1km
				{QID: "Lake", Category: "lake", Lat: 0, Lon: 0, Sitelinks: 500},
			},
			expected: []string{"Lake"},
		},
		{
			name: "Different Groups - ISOLATION (City vs Monument)",
			// City (Settlements) vs Monument (Attractions).
			// City (1000 links, XL=5km) vs Monument (500 links, M=500m).
			// Same Location.
			// Normal merge: City wins.
			// Isolation: SHOULD KEEP BOTH.
			candidates: []Article{
				{QID: "City", Category: "city", Lat: 0, Lon: 0, Sitelinks: 1000},
				{QID: "Monu", Category: "monument", Lat: 0, Lon: 0, Sitelinks: 500},
			},
			expected: []string{"City", "Monu"},
		},
		{
			name: "Same Group - Too Far (Keep Both)",
			candidates: []Article{
				{QID: "Lake1", Category: "lake", Lat: 0, Lon: 0, Sitelinks: 500},
				{QID: "Lake2", Category: "lake", Lat: 0.1, Lon: 0, Sitelinks: 400}, // ~11km
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
