package wikidata

import (
	"context"
	"log/slog"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"
)

// --- Tests for Service Pipeline Helpers ---

// TestAssignRescueCategory covers assignRescueCategory logic
func TestAssignRescueCategory(t *testing.T) {
	pl := &Pipeline{
		logger: slog.Default(),
	}

	tests := []struct {
		name         string
		h, l, area   float64
		wantCategory string
	}{
		{"By Area", 0, 0, 100, "Area"},
		{"By Height", 50, 0, 0, "Height"},
		{"By Length", 0, 50, 0, "Length"},
		{"Priority Area over others", 0, 10, 100, "Area"},
		{"Priority Height over Length", 50, 50, 0, "Height"},
		{"Fallback Landmark", 0, 0, 0, "Landmark"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Article{QID: "QTEST", LocalTitles: map[string]string{"en": "Test"}}
			pl.assignRescueCategory(a, tt.h, tt.l, tt.area)
			if a.Category != tt.wantCategory {
				t.Errorf("got %q, want %q", a.Category, tt.wantCategory)
			}
		})
	}
}

// StubDimClassifier for testing
type StubDimClassifier struct {
	cfg *config.CategoriesConfig
}

func (s *StubDimClassifier) ResetDimensions()                                      {}
func (s *StubDimClassifier) ObserveDimensions(h, l, a float64)                     {}
func (s *StubDimClassifier) FinalizeDimensions()                                   {}
func (s *StubDimClassifier) ShouldRescue(h, l, a float64, instances []string) bool { return false }
func (s *StubDimClassifier) GetMultiplier(h, l, a float64) float64                 { return 1.0 }
func (s *StubDimClassifier) GetConfig() *config.CategoriesConfig                   { return s.cfg }
func (s *StubDimClassifier) Classify(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	return nil, nil
}
func (s *StubDimClassifier) ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
	return nil
}

// TestGetSitelinksMin covers getSitelinksMin logic
func TestGetSitelinksMin(t *testing.T) {
	mockCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"city": {SitelinksMin: 10},
		},
	}
	stub := &StubDimClassifier{cfg: mockCfg}
	pl := &Pipeline{}

	tests := []struct {
		name     string
		dc       DimClassifier
		category string
		want     int
	}{
		{
			name:     "Known Category",
			dc:       stub,
			category: "city",
			want:     10,
		},
		{
			name:     "Unknown Category",
			dc:       stub,
			category: "unknown",
			want:     0,
		},
		{
			name:     "Nil Classifier",
			dc:       nil,
			category: "city",
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pl.getSitelinksMin(tt.dc, tt.category)
			if got != tt.want {
				t.Errorf("getSitelinksMin() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestGetArticleDimensions covers the helper function logic
func TestGetArticleDimensions(t *testing.T) {
	val10 := 10.0
	val20 := 20.0
	val30 := 30.0

	tests := []struct {
		name     string
		article  *Article
		wantH    float64
		wantL    float64
		wantArea float64
	}{
		{
			name: "All Set",
			article: &Article{
				Height: &val10,
				Length: &val20,
				Area:   &val30,
			},
			wantH:    10.0,
			wantL:    20.0,
			wantArea: 30.0,
		},
		{
			name:     "Nil Pointers",
			article:  &Article{},
			wantH:    0.0,
			wantL:    0.0,
			wantArea: 0.0,
		},
		{
			name: "Partial Set",
			article: &Article{
				Height: &val10,
			},
			wantH:    10.0,
			wantL:    0.0,
			wantArea: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, l, area := getArticleDimensions(tt.article)
			if h != tt.wantH {
				t.Errorf("h = %f, want %f", h, tt.wantH)
			}
			if l != tt.wantL {
				t.Errorf("l = %f, want %f", l, tt.wantL)
			}
			if area != tt.wantArea {
				t.Errorf("area = %f, want %f", area, tt.wantArea)
			}
		})
	}
}

// TestGetIcon covers getIcon logic (case insensitive lookup)
func TestGetIcon(t *testing.T) {
	mockCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"city": {Icon: "city-hall"},
		},
	}
	stub := &StubDimClassifier{cfg: mockCfg}
	// Note: getIcon relies on p.classifier internally if we call via method,
	// BUT in pipeline.go, getIcon implementation casts p.classifier.
	// So we need to set the classifier field in Pipeline.
	pl := &Pipeline{classifier: stub}

	tests := []struct {
		name     string
		category string
		want     string
	}{
		{
			name:     "Known Category Lowercase",
			category: "city",
			want:     "city-hall",
		},
		{
			name:     "Known Category MixedCase",
			category: "CiTy", // Code uses strings.ToLower
			want:     "city-hall",
		},
		{
			name:     "Unknown Category",
			category: "alien_base",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pl.getIcon(tt.category)
			if got != tt.want {
				t.Errorf("getIcon(%q) = %q, want %q", tt.category, got, tt.want)
			}
		})
	}
}

// Stub with functional mocking for batch classification
type MockBatchClassifier struct {
	BatchFunc func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
}

func (m *MockBatchClassifier) ResetDimensions()                                      {}
func (m *MockBatchClassifier) ObserveDimensions(h, l, a float64)                     {}
func (m *MockBatchClassifier) FinalizeDimensions()                                   {}
func (m *MockBatchClassifier) ShouldRescue(h, l, a float64, instances []string) bool { return false }
func (m *MockBatchClassifier) GetMultiplier(h, l, a float64) float64                 { return 1.0 }
func (m *MockBatchClassifier) GetConfig() *config.CategoriesConfig                   { return nil }
func (m *MockBatchClassifier) Classify(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	return nil, nil
}
func (m *MockBatchClassifier) ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
	if m.BatchFunc != nil {
		return m.BatchFunc(ctx, entities)
	}
	return nil
}

func TestRunBatchClassification(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]EntityMetadata
		mockRet map[string]*model.ClassificationResult
		wantLen int
	}{
		{
			name: "Ignored Success",
			input: map[string]EntityMetadata{
				"Q1": {Labels: map[string]string{"en": "A"}},
			},
			mockRet: map[string]*model.ClassificationResult{
				"Q1": {Ignored: true},
			},
			wantLen: 1,
		},
		{
			name:    "Empty Input",
			input:   map[string]EntityMetadata{},
			mockRet: map[string]*model.ClassificationResult{},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockBatchClassifier{
				BatchFunc: func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
					return tt.mockRet
				},
			}
			pl := &Pipeline{classifier: mock}
			got := pl.runBatchClassification(context.Background(), []Article{}, tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("runBatchClassification got len %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSetIgnoredByQID(t *testing.T) {
	// Setup: Raw Articles
	articles := []Article{
		{QID: "Q1", Ignored: false},
		{QID: "Q2", Ignored: false},
	}
	pl := &Pipeline{}

	// Action: Ignore Q1
	pl.setIgnoredByQID(articles, "Q1")

	// Assert
	if !articles[0].Ignored {
		t.Error("Q1 should be ignored")
	}
	if articles[1].Ignored {
		t.Error("Q2 should NOT be ignored")
	}

	// Action: Ignore Unknown QID (Should assume safe/no-op)
	pl.setIgnoredByQID(articles, "Q99")
	// Crash check implied
}

func TestFindBestLocalCandidate(t *testing.T) {
	// Function under test: findBestLocalCandidate(a *Article, lengths map[string]map[string]int, localLangs []string) (bestLang, bestTitle string, maxLen int)

	tests := []struct {
		name       string
		article    *Article
		lengths    map[string]map[string]int
		localLangs []string
		wantLang   string
		wantTitle  string
	}{
		{
			name: "Pick Longest Article",
			article: &Article{
				LocalTitles: map[string]string{"en": "Eiffel", "fr": "Tour Eiffel"},
			},
			lengths: map[string]map[string]int{
				"en": {"Eiffel": 100},
				"fr": {"Tour Eiffel": 200},
			},
			localLangs: []string{"fr", "en"},
			wantLang:   "fr",
			wantTitle:  "Tour Eiffel",
		},
		{
			name: "Tie-Breaker Priority",
			article: &Article{
				LocalTitles: map[string]string{"en": "Eiffel", "fr": "Tour Eiffel"},
			},
			lengths: map[string]map[string]int{
				"en": {"Eiffel": 200},
				"fr": {"Tour Eiffel": 200},
			},
			localLangs: []string{"fr", "en"}, // Prefer FR
			wantLang:   "fr",
			wantTitle:  "Tour Eiffel",
		},
		{
			name: "Fallback (No Lengths)",
			article: &Article{
				LocalTitles: map[string]string{"en": "Eiffel", "es": "Torre"},
			},
			lengths:    map[string]map[string]int{},
			localLangs: []string{"es", "en"}, // Prefer ES
			wantLang:   "es",
			wantTitle:  "Torre",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLang, gotTitle, _ := findBestLocalCandidate(tt.article, tt.lengths, tt.localLangs)
			if gotLang != tt.wantLang {
				t.Errorf("gotLang %q, want %q", gotLang, tt.wantLang)
			}
			if gotTitle != tt.wantTitle {
				t.Errorf("gotTitle %q, want %q", gotTitle, tt.wantTitle)
			}
		})
	}
}
