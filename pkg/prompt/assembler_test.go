package prompt

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"strings"
	"testing"
	"time"
)

// Mock objects for testing

type MockGeo struct {
	Country string
	City    string
}

func (m *MockGeo) GetLocation(lat, lon float64) model.LocationInfo {
	return model.LocationInfo{CountryCode: m.Country, CityName: m.City}
}

type MockWikipedia struct{}

func (m *MockWikipedia) GetArticleHTML(ctx context.Context, title, lang string) (string, error) {
	return "<html><body>Prose</body></html>", nil
}

type MockStore struct {
	State map[string]string
}

func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *MockStore) SaveArticle(ctx context.Context, art *model.Article) error { return nil }
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) {
	val, ok := m.State[key]
	return val, ok
}

type MockLLM struct{}

func (m *MockLLM) HasProfile(name string) bool { return false }
func (m *MockLLM) GenerateText(ctx context.Context, profile, prompt string) (string, error) {
	return "", nil
}

type MockPOIProvider struct {
	Rivals int
}

func (m *MockPOIProvider) CountScoredAbove(threshold float64, limit int) int { return m.Rivals }

type MockRenderer struct{}

func (m *MockRenderer) Render(name string, data any) (string, error) { return "Rendered", nil }

func TestAssembler_NewPromptData(t *testing.T) {
	a := &Assembler{
		cfg: &config.Config{
			Narrator: config.NarratorConfig{
				TargetLanguage: "en-US",
			},
		},
	}
	session := SessionState{
		Events: []model.TripEvent{
			{
				Timestamp: time.Now(),
				Type:      "narration",
				Title:     "Test POI",
				Summary:   "Interesting summary",
			},
		},
		LastSentence: "Last",
	}
	pd := a.NewPromptData(session)

	summary := pd["TripSummary"].(string)
	if !strings.Contains(summary, "Test POI") || !strings.Contains(summary, "Interesting summary") {
		t.Errorf("Expected summary to contain event info, got %v", summary)
	}
	if pd["LastSentence"] != "Last" {
		t.Errorf("Expected Last, got %v", pd["LastSentence"])
	}
}

func TestAssembler_DetermineSkewStrategy(t *testing.T) {
	a := &Assembler{
		poiMgr: &MockPOIProvider{Rivals: 5},
	}
	poi := &model.POI{Score: 10}

	// Many rivals -> min_skew
	strat := a.DetermineSkewStrategy(poi, false)
	if strat != StrategyMinSkew {
		t.Errorf("Expected %s, got %s", StrategyMinSkew, strat)
	}

	// On ground -> max_skew
	strat = a.DetermineSkewStrategy(poi, true)
	if strat != StrategyMaxSkew {
		t.Errorf("Expected %s, got %s", StrategyMaxSkew, strat)
	}
}

func TestAssembler_ForPOI_NilTelemetry(t *testing.T) {
	a := &Assembler{
		cfg: &config.Config{
			Narrator: config.NarratorConfig{
				TargetLanguage: "en-US",
			},
		},
		geoSvc:    &MockGeo{Country: "Test", City: "TestCity"},
		st:        &MockStore{State: map[string]string{}},
		prompts:   &MockRenderer{},
		wikipedia: &MockWikipedia{},
		poiMgr:    &MockPOIProvider{Rivals: 0},
		llm:       &MockLLM{},
	}

	session := SessionState{
		Events:       []model.TripEvent{},
		LastSentence: "",
	}
	// Minimal POI
	p := &model.POI{
		Lat:        10,
		Lon:        10,
		WikidataID: "Q123",
		Score:      1.0,
	}

	// Should not panic (handle nil telemetry)
	pd := a.ForPOI(context.Background(), p, nil, "", session)

	if pd["NavInstruction"] != "" {
		t.Errorf("Expected empty nav instruction for nil telemetry, got %v", pd["NavInstruction"])
	}
}
