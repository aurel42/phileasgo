package wikidata

import (
	"log/slog"
	"phileasgo/pkg/config"
	"phileasgo/pkg/rescue"
	"testing"
)

type testConfigProvider struct {
	config.Provider
	cfg *config.Config
}

func (m *testConfigProvider) AppConfig() *config.Config { return m.cfg }

func TestPostProcessArticlesRescue(t *testing.T) {
	mockCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"tower": {SitelinksMin: 5},
		},
	}
	stub := &StubClassifier{cfg: mockCfg}

	appCfg := &config.Config{}
	appCfg.Wikidata.Rescue.PromoteByDimension.MinHeight = 30.0
	appCfg.Wikidata.Rescue.PromoteByDimension.MinLength = 500.0
	appCfg.Wikidata.Rescue.PromoteByDimension.MinArea = 10000.0

	pl := &Pipeline{
		classifier: stub,
		logger:     slog.Default(),
		cfgProv:    &testConfigProvider{cfg: appCfg},
		grid:       NewGrid(),
	}

	h500 := 500.0
	h10 := 10.0
	tests := []struct {
		name        string
		rawArticles []Article
		medians     rescue.MedianStats
		wantCount   int // processed count
		wantRescued int // rescued count
		wantCat     string
	}{
		{
			name: "Categorized but low sitelinks - Rescued",
			rawArticles: []Article{
				{QID: "Q1", Category: "tower", Sitelinks: 1, Height: &h500},
			},
			medians:     rescue.MedianStats{MedianHeight: 10},
			wantCount:   1,
			wantRescued: 1,
			wantCat:     "height",
		},
		{
			name: "Categorized but low sitelinks - Not rescued (too small)",
			rawArticles: []Article{
				{QID: "Q1", Category: "tower", Sitelinks: 1, Height: &h10},
			},
			medians:     rescue.MedianStats{MedianHeight: 10},
			wantCount:   0,
			wantRescued: 0,
		},
		{
			name: "Categorized and meets sitelinks - Not rescued (but processed)",
			rawArticles: []Article{
				{QID: "Q2", Category: "tower", Sitelinks: 10, Height: &h500},
			},
			medians:     rescue.MedianStats{MedianHeight: 10},
			wantCount:   1,
			wantRescued: 0,
			wantCat:     "tower",
		},
		{
			name: "Unclassified - Rescued",
			rawArticles: []Article{
				{QID: "Q3", Category: "", Height: &h500},
			},
			medians:     rescue.MedianStats{MedianHeight: 10},
			wantCount:   1,
			wantRescued: 1,
			wantCat:     "height",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processed, rescued, err := pl.postProcessArticles(tt.rawArticles, 0, 0, tt.medians)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(processed) != tt.wantCount {
				t.Errorf("%s: got %d processed, want %d", tt.name, len(processed), tt.wantCount)
			}
			if rescued != tt.wantRescued {
				t.Errorf("%s: got %d rescued, want %d", tt.name, rescued, tt.wantRescued)
			}
			if len(processed) > 0 && tt.wantCat != "" && processed[0].Category != tt.wantCat {
				t.Errorf("%s: got category %s, want %s", tt.name, processed[0].Category, tt.wantCat)
			}
		})
	}
}
