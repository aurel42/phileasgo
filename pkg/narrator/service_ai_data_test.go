package narrator

import (
	"context"
	"errors"
	"os"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func TestBuildPromptData(t *testing.T) {
	// Create Mocks
	mockGeo := &MockGeo{Country: "US", City: "TestCity"}
	// Ground to ensure silence < 4.5km
	mockSim := &MockSim{Telemetry: sim.Telemetry{
		Latitude: 10, Longitude: 20, IsOnGround: true,
	}}
	mockWiki := &MockWikipedia{Content: "<div class=\"mw-parser-output\"><p>WikiText</p></div>"}
	mockStore := &MockStore{}
	pm, _ := prompts.NewManager(t.TempDir()) // empty

	cfg := &config.Config{
		Narrator: config.NarratorConfig{
			TargetLanguage: "en-US",
			Units:          "metric",
		},
		TTS: config.TTSConfig{
			Engine: "edge",
		},
	}

	// Create service
	svc := NewAIService(cfg, nil, nil, pm, nil, nil, nil, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil, nil, nil)

	mockPOI := &MockPOIProvider{
		CountScoredAboveFunc: func(threshold float64, limit int) int {
			return 0
		},
	}
	svc.poiMgr = mockPOI

	t.Run("Last Sentence Injection", func(t *testing.T) {
		// Set internal state directly
		svc.lastScriptEnd = "It was a dark and stormy night"

		poi := &model.POI{Lat: 10, Lon: 20}
		pd := svc.buildPromptData(context.Background(), poi, nil, "uniform")

		if pd.LastSentence != "It was a dark and stormy night" {
			t.Errorf("Expected last sentence injected, got %q", pd.LastSentence)
		}
	})

	t.Run("Happy Path", func(t *testing.T) {
		poi := &model.POI{
			WikidataID: "Q1",
			Lat:        10.0,
			Lon:        20.0,
			WPURL:      "https://en.wikipedia.org/wiki/Foo",
		}

		pd := svc.buildPromptData(context.Background(), poi, nil, "uniform")

		// Assertions
		if pd.TourGuideName != "Ava" {
			t.Errorf("Expected Ava, got %s", pd.TourGuideName)
		}
		if pd.Language_code != "en" {
			t.Errorf("Expected en, got %s", pd.Language_code)
		}
		if pd.TargetRegion != "Near TestCity" {
			t.Errorf("Expected Near TestCity, got %s", pd.TargetRegion)
		}
		// Wiki text should be fetched (and cleaned)
		if pd.WikipediaText != "WikiText" {
			t.Errorf("Expected WikiText, got %s", pd.WikipediaText)
		}
		// Nav instruction: Distance 0 -> Ground < 4.5km -> Empty
		if pd.NavInstruction != "" {
			t.Errorf("Expected empty nav for 0 dist, got %s", pd.NavInstruction)
		}
	})

	t.Run("Language Parsing", func(t *testing.T) {
		svc.cfg.Narrator.TargetLanguage = "de-DE"
		poi := &model.POI{Lat: 10, Lon: 20}
		pd := svc.buildPromptData(context.Background(), poi, nil, "uniform")
		if pd.Language_code != "de" { // "de" is fallback if no resolver
			t.Errorf("Expected de, got %s", pd.Language_code)
		}
	})
}

func TestFetchWikipediaText(t *testing.T) {
	mockWiki := &MockWikipedia{}
	mockStore := &MockStore{
		Articles: make(map[string]*model.Article),
	}
	svc := &AIService{
		wikipedia: mockWiki,
		st:        mockStore,
	}

	tests := []struct {
		name       string
		poi        *model.POI
		storeArt   *model.Article
		wikiResult string
		wikiErr    error
		want       string
	}{
		{
			name:     "In Store",
			poi:      &model.POI{WikidataID: "Q1"},
			storeArt: &model.Article{Text: "Cached"},
			want:     "Cached",
		},
		{
			name:       "Fetch Success",
			poi:        &model.POI{WikidataID: "Q2", WPURL: "https://en.wikipedia.org/wiki/Bar"},
			storeArt:   nil,
			wikiResult: "<div class=\"mw-parser-output\"><p>Fresh</p></div>",
			want:       "Fresh",
		},
		{
			name: "Bad URL",
			poi:  &model.POI{WikidataID: "Q3", WPURL: "https://wikidata.org/wiki/Q3"},
			want: "",
		},
		{
			name:    "Fetch Error",
			poi:     &model.POI{WikidataID: "Q4", WPURL: "https://en.wikipedia.org/wiki/Baz"},
			wikiErr: errors.New("fail"),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Store State
			if tt.storeArt != nil {
				mockStore.Articles[tt.poi.WikidataID] = tt.storeArt
			} else {
				delete(mockStore.Articles, tt.poi.WikidataID)
			}

			mockWiki.Content = tt.wikiResult
			mockWiki.Err = tt.wikiErr

			got := svc.fetchWikipediaText(context.Background(), tt.poi)
			if got.Prose != tt.want {
				t.Errorf("fetchWikipediaText() = %q, want %q", got.Prose, tt.want)
			}
		})
	}
}

func TestFetchRecentContext(t *testing.T) {
	mockStore := &MockStore{}
	svc := &AIService{st: mockStore}

	tests := []struct {
		name       string
		userLat    float64
		userLon    float64
		recentPOIs []*model.POI
		want       string
	}{
		{
			name:    "None nearby",
			userLat: 0, userLon: 0,
			recentPOIs: []*model.POI{{Lat: 10, Lon: 10, NameEn: "Far"}}, // ~1000km away
			want:       "None",
		},
		{
			name:    "One nearby",
			userLat: 0, userLon: 0,
			recentPOIs: []*model.POI{{Lat: 0.1, Lon: 0.1, NameEn: "Near", Category: "Park"}}, // ~15km
			want:       "Near (Park)",
		},
		{
			name:    "Empty",
			userLat: 0, userLon: 0,
			recentPOIs: []*model.POI{},
			want:       "None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore.RecentPOIs = tt.recentPOIs
			got := svc.fetchRecentContext(context.Background(), tt.userLat, tt.userLon)
			if got != tt.want {
				t.Errorf("fetchRecentContext() = %q, want %q", got, tt.want)
			}
		})
	}

}

func TestSampleNarrationLength_Logic(t *testing.T) {
	// Simple deterministic test using mocked poiMgr
	mockPOI := &MockPOIProvider{}
	svc := &AIService{
		cfg:    &config.Config{Narrator: config.NarratorConfig{}},
		poiMgr: mockPOI,
		st:     &MockStore{},
	}

	tests := []struct {
		name        string
		rivalsCount int
		expectStrat string // "min_skew", "max_skew", etc
	}{
		{
			name:        "Many Rivals",
			rivalsCount: 5,
			expectStrat: "min_skew",
		},
		{
			name:        "Lone Wolf",
			rivalsCount: 0,
			expectStrat: "max_skew",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPOI.CountScoredAboveFunc = func(threshold float64, limit int) int {
				return tt.rivalsCount
			}
			_, strat := svc.sampleNarrationLength(&model.POI{Score: 10}, "", 100)
			if strat != tt.expectStrat {
				t.Errorf("Expected strategy %s, got %s", tt.expectStrat, strat)
			}
		})
	}
}

func TestTripSummary(t *testing.T) {
	cfg := &config.Config{
		Narrator: config.NarratorConfig{},
	}
	svc := &AIService{cfg: cfg}
	svc.tripSummary = "Initial trip summary."

	// Check prompt data
	pd := NarrationPromptData{
		TripSummary: svc.getTripSummary(),
	}
	if pd.TripSummary != "Initial trip summary." {
		t.Errorf("Expected summary 'Initial trip summary.', got %s", pd.TripSummary)
	}
}

func TestFetchPregroundContext(t *testing.T) {
	tests := []struct {
		name          string
		categoriesCfg *config.CategoriesConfig
		llmHasProfile bool
		llmResponse   string
		llmErr        error
		poiCategory   string
		want          string
	}{
		{
			name:          "No categories config",
			categoriesCfg: nil,
			poiCategory:   "airfield",
			want:          "",
		},
		{
			name: "Category pregrounding disabled",
			categoriesCfg: &config.CategoriesConfig{
				Categories: map[string]config.Category{
					"city": {Preground: false},
				},
			},
			poiCategory: "city",
			want:        "",
		},
		{
			name: "LLM missing pregrounding profile",
			categoriesCfg: &config.CategoriesConfig{
				Categories: map[string]config.Category{
					"airfield": {Preground: true},
				},
			},
			llmHasProfile: false,
			poiCategory:   "airfield",
			want:          "",
		},
		{
			name: "Success",
			categoriesCfg: &config.CategoriesConfig{
				Categories: map[string]config.Category{
					"airfield": {Preground: true},
				},
			},
			llmHasProfile: true,
			llmResponse:   "Local context from Sonar",
			poiCategory:   "airfield",
			want:          "Local context from Sonar",
		},
		{
			name: "LLM error - graceful degradation",
			categoriesCfg: &config.CategoriesConfig{
				Categories: map[string]config.Category{
					"airfield": {Preground: true},
				},
			},
			llmHasProfile: true,
			llmErr:        errors.New("sonar failed"),
			poiCategory:   "airfield",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &MockLLM{
				Response:      tt.llmResponse,
				Err:           tt.llmErr,
				HasProfileVal: tt.llmHasProfile,
			}
			mockGeo := &MockGeo{Country: "DE"}

			// Create temp prompts dir with pregrounding template
			tmpDir := t.TempDir()
			// Create the context subdirectory and template for tests that reach the LLM
			if tt.llmHasProfile && tt.categoriesCfg != nil {
				ctxDir := tmpDir + "/context"
				_ = os.MkdirAll(ctxDir, 0o755)
				_ = os.WriteFile(ctxDir+"/pregrounding.tmpl", []byte("Research {{.Name}} in {{.Country}}"), 0o644)
			}
			pm, _ := prompts.NewManager(tmpDir)

			svc := &AIService{
				llm:           mockLLM,
				geoSvc:        mockGeo,
				prompts:       pm,
				categoriesCfg: tt.categoriesCfg,
			}

			poi := &model.POI{
				WikidataID: "Q123",
				NameEn:     "Test POI",
				Category:   tt.poiCategory,
				Lat:        49.0,
				Lon:        8.0,
			}

			got := svc.fetchPregroundContext(context.Background(), poi)
			if got != tt.want {
				t.Errorf("fetchPregroundContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSampleNarrationLength_WordCount(t *testing.T) {
	mockStore := &MockStore{
		State: make(map[string]string),
	}
	svc := &AIService{
		cfg: &config.Config{
			Narrator: config.NarratorConfig{
				NarrationLengthShortWords: 50,
				NarrationLengthLongWords:  200,
			},
		},
		st: mockStore,
	}

	tests := []struct {
		name        string
		strategy    string
		wikiWords   int
		preWords    int
		textLength  string // user setting "1".."5"
		expectWords int
	}{
		// --- Multiplier Tests (High Depth) ---
		{
			name:        "Short Target - Mult 1.0 (50x1.0)",
			strategy:    StrategyMinSkew,
			wikiWords:   500,
			preWords:    0,
			textLength:  "1",
			expectWords: 50,
		},
		{
			name:        "Short Target - Mult 2.0 (50x2.0)",
			strategy:    StrategyMinSkew,
			wikiWords:   500,
			preWords:    0,
			textLength:  "5",
			expectWords: 100,
		},
		{
			name:        "Long Target - Mult 1.0 (200x1.0)",
			strategy:    StrategyMaxSkew,
			wikiWords:   500,
			preWords:    0,
			textLength:  "1",
			expectWords: 200,
		},
		{
			name:        "Long Target - Mult 2.0 (200x2.0)",
			strategy:    StrategyMaxSkew,
			wikiWords:   1000,
			preWords:    0,
			textLength:  "5",
			expectWords: 400,
		},

		// --- Source Depth Capping Tests ---
		{
			name:        "Capped by Wiki depth (40/2=20)",
			strategy:    StrategyMinSkew,
			wikiWords:   40,
			preWords:    0,
			textLength:  "1",
			expectWords: 20,
		},
		{
			name:        "Boosted by Pregrounding (40+160 = 200/2=100)",
			strategy:    StrategyMaxSkew,
			wikiWords:   40,
			preWords:    160,
			textLength:  "1",
			expectWords: 100,
		},
		{
			name:        "Zero Info = Zero Words (min(0, target))",
			strategy:    StrategyMaxSkew,
			wikiWords:   0,
			preWords:    0,
			textLength:  "1",
			expectWords: 0,
		},
		{
			name:        "Pregrounding Unavailable (Degrade to Wiki)",
			strategy:    StrategyMaxSkew,
			wikiWords:   400,
			preWords:    0,
			textLength:  "1",
			expectWords: 200, // min(400/2, 200*1.0)
		},
		{
			name:        "Low Info vs High Multiplier",
			strategy:    StrategyMaxSkew,
			wikiWords:   100, // sourceLimit = 50
			preWords:    0,
			textLength:  "5", // targetLimit = 400
			expectWords: 50,  // min(50, 400)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore.State["text_length"] = tt.textLength
			got, _ := svc.sampleNarrationLength(&model.POI{}, tt.strategy, tt.wikiWords+tt.preWords)
			if got != tt.expectWords {
				t.Errorf("%s: got %d, want %d", tt.name, got, tt.expectWords)
			}
		})
	}
}
