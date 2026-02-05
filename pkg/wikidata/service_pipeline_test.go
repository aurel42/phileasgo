package wikidata

import (
	"context"
	"log/slog"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"
)

// --- Tests for Service Pipeline Helpers ---

// StubClassifier for testing
type StubClassifier struct {
	cfg *config.CategoriesConfig
}

func (s *StubClassifier) GetConfig() *config.CategoriesConfig { return s.cfg }
func (s *StubClassifier) Classify(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	return nil, nil
}
func (s *StubClassifier) ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
	return nil
}

// TestGetSitelinksMin covers getSitelinksMin logic
func TestGetSitelinksMin(t *testing.T) {
	mockCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"city": {SitelinksMin: 10},
		},
	}
	stub := &StubClassifier{cfg: mockCfg}
	pl := &Pipeline{classifier: stub}

	tests := []struct {
		name     string
		category string
		want     int
	}{
		{
			name:     "Known Category",
			category: "city",
			want:     10,
		},
		{
			name:     "Unknown Category",
			category: "unknown",
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pl.getSitelinksMin(tt.category)
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
	stub := &StubClassifier{cfg: mockCfg}
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

// MockBatchClassifier for testing
type MockBatchClassifier struct {
	BatchFunc func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
}

func (m *MockBatchClassifier) GetConfig() *config.CategoriesConfig { return nil }
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
				"Q1": {Claims: map[string][]string{"P31": {"Q123"}}},
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
			pl := &Pipeline{classifier: mock, logger: slog.Default(), store: &mockStore{}}
			got := pl.runBatchClassification(context.Background(), []Article{{QID: "Q1"}}, tt.input)
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
}

func TestFindBestLocalCandidate(t *testing.T) {
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
				LocalTitles: map[string]string{"pl": "Eiffel", "it": "Torre Eiffel"},
			},
			lengths: map[string]map[string]int{
				"pl": {"Eiffel": 200},
				"it": {"Torre Eiffel": 200},
			},
			localLangs: []string{"it", "pl"}, // Prefer IT
			wantLang:   "it",
			wantTitle:  "Torre Eiffel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestPipeline(&mockStore{})
			gotLang, gotTitle, _, _ := p.findBestLocalCandidate(tt.article, tt.lengths, tt.localLangs)
			if gotLang != tt.wantLang {
				t.Errorf("gotLang %q, want %q", gotLang, tt.wantLang)
			}
			if gotTitle != tt.wantTitle {
				t.Errorf("gotTitle %q, want %q", gotTitle, tt.wantTitle)
			}
		})
	}
}
func getArticleDimensions(a *Article) (h, l, area float64) {
	if a.Height != nil {
		h = *a.Height
	}
	if a.Length != nil {
		l = *a.Length
	}
	if a.Area != nil {
		area = *a.Area
	}
	return h, l, area
}
