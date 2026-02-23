package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/store"
	"time"
)

// Simplified MockStore for API testing
type apiMockStore struct {
	ResetCalled bool
	ResetRadius float64
}

func (m *apiMockStore) SaveLastPlayed(ctx context.Context, poiID string, t time.Time) error {
	return nil
}
func (m *apiMockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error {
	m.ResetCalled = true
	m.ResetRadius = radius
	return nil
}

// Stubs for other interface methods...
func (m *apiMockStore) GetPOI(ctx context.Context, id string) (*model.POI, error) { return nil, nil }
func (m *apiMockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (m *apiMockStore) SavePOI(ctx context.Context, p *model.POI) error { return nil }
func (m *apiMockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *apiMockStore) SaveMSFSPOI(ctx context.Context, p *model.MSFSPOI) error { return nil }
func (m *apiMockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *apiMockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *apiMockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error {
	return nil
}
func (m *apiMockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *apiMockStore) SaveArticle(ctx context.Context, a *model.Article) error    { return nil }
func (m *apiMockStore) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *apiMockStore) SetCache(ctx context.Context, key string, val []byte) error { return nil }
func (m *apiMockStore) HasCache(ctx context.Context, key string) (bool, error)     { return false, nil }
func (m *apiMockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *apiMockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *apiMockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	return nil
}
func (m *apiMockStore) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]store.GeodataRecord, error) {
	return nil, nil
}
func (m *apiMockStore) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *apiMockStore) GetState(ctx context.Context, key string) (string, bool) {
	if key == "filter_mode" {
		return "adaptive", true
	}
	if key == "target_poi_count" {
		return "5", true
	}
	return "", false
}
func (m *apiMockStore) SetState(ctx context.Context, key, val string) error { return nil }
func (m *apiMockStore) DeleteState(ctx context.Context, key string) error   { return nil }
func (m *apiMockStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	return "", false, nil
}
func (m *apiMockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}
func (m *apiMockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return make(map[string][]string), nil
}
func (m *apiMockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}

func (m *apiMockStore) DeleteSeenEntities(ctx context.Context, qids []string) error {
	return nil
}
func (m *apiMockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *apiMockStore) GetRegionalCategories(ctx context.Context, latGrid, lonGrid int) (map[string]string, map[string]string, error) {
	return nil, nil, nil
}
func (m *apiMockStore) SaveRegionalCategories(ctx context.Context, latGrid, lonGrid int, categories map[string]string, labels map[string]string) error {
	return nil
}
func (m *apiMockStore) Close() error { return nil }

func TestHandleResetLastPlayed(t *testing.T) {
	mockStore := &apiMockStore{}
	cfg := config.NewProvider(config.DefaultConfig(), nil)
	mgr := poi.NewManager(cfg, mockStore, nil)
	handler := NewPOIHandler(mgr, nil, mockStore, cfg, nil, nil) // WP Client nil is fine here

	t.Run("Success", func(t *testing.T) {
		reqBody := map[string]float64{
			"lat": 10.0,
			"lon": 20.0,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/pois/reset-last-played", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		handler.HandleResetLastPlayed(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", w.Code)
		}
		if !mockStore.ResetCalled {
			t.Error("Expected ResetLastPlayed to be called")
		}
		if mockStore.ResetRadius != 100000.0 {
			t.Errorf("Expected 100km (100000m) radius, got %f", mockStore.ResetRadius)
		}
	})

	t.Run("InvalidMethod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/pois/reset-last-played", nil)
		w := httptest.NewRecorder()

		handler.HandleResetLastPlayed(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405 Method Not Allowed, got %d", w.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/pois/reset-last-played", bytes.NewBufferString("invalid json"))
		w := httptest.NewRecorder()

		handler.HandleResetLastPlayed(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestHandleTracked(t *testing.T) {
	mockStore := &apiMockStore{}
	cfg := config.NewProvider(config.DefaultConfig(), nil)
	mgr := poi.NewManager(cfg, mockStore, nil)
	// Add some POIs to the manager
	mgr.TrackPOI(context.Background(), &model.POI{WikidataID: "P1", NameEn: "POI 1", Score: 10.0, IsVisible: true})
	mgr.TrackPOI(context.Background(), &model.POI{WikidataID: "P2", NameEn: "POI 2", Score: 8.0, IsVisible: true})

	handler := NewPOIHandler(mgr, nil, mockStore, cfg, nil, nil)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/pois/tracked", nil)
		w := httptest.NewRecorder()

		handler.HandleTracked(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", w.Code)
		}

		// Verify headers
		if w.Header().Get("X-Phileas-Effective-Threshold") == "" {
			t.Error("Expected X-Phileas-Effective-Threshold header")
		}

		var resp []*model.POI
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// We mocked GetState to return "adaptive" with target 5.
		// Since we have 2 visible POIs, they should both be returned.
		if len(resp) != 2 {
			t.Errorf("Expected 2 POIs, got %d", len(resp))
		}
	})
}
