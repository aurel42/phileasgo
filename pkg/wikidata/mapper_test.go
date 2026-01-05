package wikidata

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"phileasgo/pkg/model"
)

// MockStore for testing
type MockStore struct {
	cache map[string][]byte
}

func (m *MockStore) GetCache(ctx context.Context, key string) ([]byte, bool) {
	val, ok := m.cache[key]
	return val, ok
}

func (m *MockStore) SetCache(ctx context.Context, key string, data []byte) error {
	m.cache[key] = data
	return nil
}

// Stubs for interface compliance
// Stubs for interface compliance
func (m *MockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *MockStore) GetPOIsBatch(ctx context.Context, qids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return nil, nil
}
func (m *MockStore) MarkEntitiesSeen(ctx context.Context, data map[string][]string) error { return nil }
func (m *MockStore) SavePOI(ctx context.Context, poi *model.POI) error                    { return nil }

// Missing Stubs
func (m *MockStore) GetPOI(ctx context.Context, wikidataID string) (*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) HasCache(ctx context.Context, key string) (bool, error)  { return false, nil }
func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) { return "", false }
func (m *MockStore) SetState(ctx context.Context, key, val string) error     { return nil }
func (m *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *MockStore) SaveMSFSPOI(ctx context.Context, poi *model.MSFSPOI) error { return nil }
func (m *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *MockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *MockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error { return nil }
func (m *MockStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	return "", false, nil
}
func (m *MockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *MockStore) SaveArticle(ctx context.Context, article *model.Article) error { return nil }
func (m *MockStore) Close() error                                                  { return nil }

func TestLanguageMapper_GetLanguage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store := &MockStore{cache: make(map[string][]byte)}

	mapper := NewLanguageMapper(store, nil, logger) // Client is nil, we won't refresh

	// Test Empty
	if got := mapper.GetLanguage("CZ"); got != "en" {
		t.Errorf("GetLanguage(CZ) empty = %v, want en", got)
	}

	// Test Pre-loaded
	initial := map[string]string{"CZ": "cs", "FR": "fr"}
	data, _ := json.Marshal(initial)
	store.SetCache(context.Background(), langMapCacheKey, data)

	mapper.load(context.Background())

	if got := mapper.GetLanguage("CZ"); got != "cs" {
		t.Errorf("GetLanguage(CZ) = %v, want cs", got)
	}
	if got := mapper.GetLanguage("FR"); got != "fr" {
		t.Errorf("GetLanguage(FR) = %v, want fr", got)
	}
	if got := mapper.GetLanguage("XX"); got != "en" {
		t.Errorf("GetLanguage(XX) = %v, want en", got)
	}
}
