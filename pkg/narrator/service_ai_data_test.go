package narrator

import (
	"context"
	"errors"
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
	mockWiki := &MockWikipedia{Content: "WikiText"}
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
	svc := NewAIService(cfg, nil, nil, pm, nil, nil, nil, mockGeo, mockSim, mockStore, mockWiki, nil, nil, nil, nil)

	mockPOI := &MockPOIProvider{
		CountScoredAboveFunc: func(threshold float64, limit int) int {
			return 0
		},
	}
	svc.poiMgr = mockPOI

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
		// Wiki text should be fetched
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
			wikiResult: "Fresh",
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
			if got != tt.want {
				t.Errorf("fetchWikipediaText() = %q, want %q", got, tt.want)
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
			_, strat := svc.sampleNarrationLength(&model.POI{Score: 10}, "")
			if strat != tt.expectStrat {
				t.Errorf("Expected strategy %s, got %s", tt.expectStrat, strat)
			}
		})
	}
}

func TestScriptHistory(t *testing.T) {
	cfg := &config.Config{
		Narrator: config.NarratorConfig{
			ContextHistorySize: 3,
		},
	}
	// Mock Manager
	mockPOI := &MockPOIProvider{
		GetPOIFunc: func(ctx context.Context, qid string) (*model.POI, error) {
			if qid == "Q2" || qid == "Q3" {
				return &model.POI{WikidataID: qid}, nil
			}
			return nil, errors.New("evicted")
		},
	}

	svc := &AIService{cfg: cfg, poiMgr: mockPOI}
	svc.scriptHistory = make([]ScriptEntry, 0, 10)

	// Add 3 scripts
	svc.addScriptToHistory("Q1", "POI 1", "Script 1")
	svc.addScriptToHistory("Q2", "POI 2", "Script 2")
	svc.addScriptToHistory("", "Regional Essay", "Script 3") // Essay has no QID

	// Get history
	history := svc.getScriptHistory()

	// Q1 is evicted (not in mock GetPOIFunc), Q2 is kept, Essay (empty QID) is kept.
	if len(history) != 2 {
		t.Fatalf("Expected 2 entries (Q2 and Essay), got %d", len(history))
	}

	if history[0].QID != "Q2" {
		t.Errorf("Expected first remaining script to be Q2, got %s", history[0].QID)
	}
	if history[1].QID != "" {
		t.Errorf("Expected second script to be Essay (empty QID), got %s", history[1].QID)
	}

	// Add another script to trigger limit check
	svc.addScriptToHistory("Q3", "POI 3", "Script 4")
	history = svc.getScriptHistory()

	// Limit is 3. We have Q2, Essay, Q3. All are valid.
	if len(history) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(history))
	}
}
