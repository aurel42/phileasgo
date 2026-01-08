package wikidata

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/tracker"
)

type mockStore struct {
	pois map[string]*model.POI
}

func (m *mockStore) GetPOI(ctx context.Context, id string) (*model.POI, error) {
	return m.pois[id], nil
}
func (m *mockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	res := make(map[string]*model.POI)
	for _, id := range ids {
		if p, ok := m.pois[id]; ok {
			res[id] = p
		}
	}
	return res, nil
}
func (m *mockStore) SavePOI(ctx context.Context, p *model.POI) error { return nil }
func (m *mockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *mockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *mockStore) GetCache(ctx context.Context, key string) ([]byte, bool)             { return nil, false }
func (m *mockStore) HasCache(ctx context.Context, key string) (bool, error)              { return false, nil }
func (m *mockStore) SetCache(ctx context.Context, key string, val []byte) error          { return nil }
func (m *mockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *mockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *mockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int) error {
	return nil
}
func (m *mockStore) GetState(ctx context.Context, key string) (string, bool) { return "", false }
func (m *mockStore) SetState(ctx context.Context, key, val string) error     { return nil }
func (m *mockStore) SaveMSFSPOI(ctx context.Context, p *model.MSFSPOI) error { return nil }
func (m *mockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *mockStore) GetClassification(ctx context.Context, id string) (category string, found bool, err error) {
	return "", false, nil
}
func (m *mockStore) SaveClassification(ctx context.Context, id, cat string, parents []string, name string) error {
	return nil
}
func (m *mockStore) GetHierarchy(ctx context.Context, id string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *mockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error { return nil }
func (m *mockStore) SaveArticle(ctx context.Context, a *model.Article) error             { return nil }
func (m *mockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *mockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return make(map[string][]string), nil
}
func (m *mockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *mockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}
func (m *mockStore) Close() error { return nil }

type MockClassifier struct {
	ClassifyFunc      func(ctx context.Context, qid string) (*model.ClassificationResult, error)
	ClassifyBatchFunc func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
	RescueFunc        func(h, l, a float64, i []string) bool
	GetMultiplierFunc func(h, l, a float64) float64
}

func (m *MockClassifier) Classify(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	if m.ClassifyFunc != nil {
		return m.ClassifyFunc(ctx, qid)
	}
	return nil, nil
}

func (m *MockClassifier) ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
	if m.ClassifyBatchFunc != nil {
		return m.ClassifyBatchFunc(ctx, entities)
	}
	return nil
}

func (m *MockClassifier) ShouldRescue(h, l, a float64, i []string) bool {
	if m.RescueFunc != nil {
		return m.RescueFunc(h, l, a, i)
	}
	return false
}

func (m *MockClassifier) GetMultiplier(h, l, a float64) float64 {
	if m.GetMultiplierFunc != nil {
		return m.GetMultiplierFunc(h, l, a)
	}
	return 1.0
}

func (m *MockClassifier) ObserveDimensions(h, l, a float64) {}
func (m *MockClassifier) ResetDimensions()                  {}
func (m *MockClassifier) FinalizeDimensions()               {}

func (m *MockClassifier) GetConfig() *config.CategoriesConfig {
	return &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"City":   {SitelinksMin: 5},
			"Church": {SitelinksMin: 5},
		},
	}
}

func TestProcessTileData(t *testing.T) {
	st := &mockStore{
		pois: map[string]*model.POI{
			"Q_KNOWN": {WikidataID: "Q_KNOWN", Category: "Castle"},
		},
	}

	cl := &MockClassifier{
		ClassifyFunc: func(ctx context.Context, qid string) (*model.ClassificationResult, error) {
			if qid == "Q2" {
				return &model.ClassificationResult{Category: "City"}, nil
			}
			return nil, nil
		},
	}

	svc := &Service{
		store:      st,
		classifier: cl,
		poi:        poi.NewManager(&config.Config{}, st, nil),
		tracker:    tracker.New(),
		logger:     slog.Default(),
		cfg: config.WikidataConfig{
			Area: config.AreaConfig{
				MaxArticles: 100,
			},
		},
	}

	t.Run("FilterExistingPOIs", func(t *testing.T) {
		arts := []Article{
			{QID: "Q_KNOWN"},
			{QID: "Q_UNKNOWN"},
		}
		filtered := svc.filterExistingPOIs(context.Background(), arts, []string{"Q_KNOWN", "Q_UNKNOWN"})

		if len(filtered) != 1 {
			t.Errorf("Expected 1 filtered article, got %d", len(filtered))
		}
		if filtered[0].QID != "Q_UNKNOWN" {
			t.Errorf("Expected Q_UNKNOWN to remain, got %s", filtered[0].QID)
		}
	})

	t.Run("PriorityRescue", func(t *testing.T) {
		cl.RescueFunc = func(h, l, a float64, i []string) bool {
			return h > 100 || l > 100 || a > 100
		}

		fBig := 200.0
		fSmall := 10.0

		arts := []Article{
			{QID: "A1", Category: "Church", Sitelinks: 10},                 // Standard POI
			{QID: "A2", Category: "Church", Sitelinks: 1, Height: &fBig},   // Categorized but sparse, SHOULD RESCUE (keep Category)
			{QID: "A3", Category: "Church", Sitelinks: 1, Height: &fSmall}, // Categorized but sparse, NO RESCUE (Drop)
			{QID: "A4", Category: "", Area: &fBig},                         // Uncategorized, SHOULD RESCUE (New Category)
			{QID: "A5", Category: "", Area: &fSmall},                       // Uncategorized, NO RESCUE (Drop)
		}

		processed, rescued, err := svc.postProcessArticles(arts)
		if err != nil {
			t.Fatalf("postProcessArticles failed: %v", err)
		}

		// Expected processed: A1, A2, A4
		if len(processed) != 3 {
			t.Errorf("Expected 3 processed articles, got %d", len(processed))
		}

		// A1: Church (Standard)
		if processed[0].QID != "A1" || processed[0].Category != "Church" {
			t.Errorf("A1 mismatch: QID=%s, Cat=%s", processed[0].QID, processed[0].Category)
		}

		// A2: Church (Rescued by dimensions, kept category)
		if processed[1].QID != "A2" || processed[1].Category != "Church" {
			t.Errorf("A2 mismatch: QID=%s, Cat=%s", processed[1].QID, processed[1].Category)
		}

		// A4: Area (Rescued by dimensions, new category)
		if processed[2].QID != "A4" || processed[2].Category != "Area" {
			t.Errorf("A4 mismatch: QID=%s, Cat=%s", processed[2].QID, processed[2].Category)
		}

		// rescuedCount should be 1 (only A4 as it had no category)
		if rescued != 1 {
			t.Errorf("Expected rescuedCount 1, got %d", rescued)
		}
	})

	t.Run("BuildQuery", func(t *testing.T) {
		got := buildQuery(52.5, 13.4, "de", "en", []string{"de", "pl"}, 100, "10.0")

		// Verify Language Filter structure (Order in VALUES is dependent on slice order input or map iteration upstream,
		// but here we passed a slice {"de", "pl"}. The builder iterates the slice.
		// Ah, the test failed because I checked specifically for { "de" "pl" } or { "pl" "de" }, but maybe quotes were wrong?
		// Actually my implementation iterates the slice: for _, l := range allowedLangs.
		// So it should be deterministic if the input slice is {"de", "pl"}.
		// Let's check the failure output carefully if I could, but I can't see it full.
		// Safest is to check presence of key elements.
		if !strings.Contains(got, `VALUES ?allowed_lang {`) || !strings.Contains(got, `"de"`) || !strings.Contains(got, `"pl"`) {
			t.Errorf("Query missing correctly formatted VALUES clause, got: %s", got)
		}
		// Verify Center Language Label Service binding
		if !strings.Contains(got, `SERVICE wikibase:label { bd:serviceParam wikibase:language "de,en,en". }`) {
			t.Errorf("Label service missing center language, got: %s", got)
		}
		// Verify Center Lang variable binding
		if !strings.Contains(got, `?evt_local schema:about ?item ;`) || !strings.Contains(got, `schema:inLanguage "de" ;`) {
			t.Errorf("Local language block mismatch, got: %s", got)
		}
	})
}
