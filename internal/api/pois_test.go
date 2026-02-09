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
func (m *apiMockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *apiMockStore) Close() error { return nil }

func TestHandleResetLastPlayed(t *testing.T) {
	mockStore := &apiMockStore{}
	mgr := poi.NewManager(config.NewProvider(&config.Config{}, nil), mockStore, nil)
	handler := NewPOIHandler(mgr, nil, mockStore, nil, nil) // WP Client nil is fine here

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
	mgr := poi.NewManager(config.NewProvider(&config.Config{}, nil), mockStore, nil)
	// Add some POIs to the manager
	mgr.TrackPOI(context.Background(), &model.POI{WikidataID: "P1", NameEn: "POI 1", Score: 10.0, IsVisible: true})
	mgr.TrackPOI(context.Background(), &model.POI{WikidataID: "P2", NameEn: "POI 2", Score: 8.0, IsVisible: true})

	handler := NewPOIHandler(mgr, nil, mockStore, nil, nil)

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

func TestHandleSettlements(t *testing.T) {
	mockStore := &apiMockStore{}
	configProvider := config.NewProvider(&config.Config{}, nil)
	mgr := poi.NewManager(configProvider, mockStore, nil)

	// Setup Tracked POIs
	// City
	city := &model.POI{WikidataID: "Q1", NameEn: "City", Category: "city", Lat: 10.0, Lon: 10.0, Score: 100}
	mgr.TrackPOI(context.Background(), city)

	// Town
	town := &model.POI{WikidataID: "Q2", NameEn: "Town", Category: "town", Lat: 10.1, Lon: 10.1, Score: 50}
	mgr.TrackPOI(context.Background(), town)

	// Village
	village := &model.POI{WikidataID: "Q3", NameEn: "Village", Category: "village", Lat: 10.2, Lon: 10.2, Score: 10}
	mgr.TrackPOI(context.Background(), village)

	// Out of bounds
	farCity := &model.POI{WikidataID: "Q4", NameEn: "Far City", Category: "city", Lat: 20.0, Lon: 20.0, Score: 100}
	mgr.TrackPOI(context.Background(), farCity)

	handler := NewPOIHandler(mgr, nil, mockStore, nil, nil)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		verify         func(t *testing.T, pois []*model.POI)
	}{
		{
			name:           "Missing Bounds",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			verify:         nil,
		},
		{
			name:           "All in View (City Priority)",
			queryParams:    "?minLat=9&maxLat=11&minLon=9&maxLon=11",
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, pois []*model.POI) {
				if len(pois) != 1 {
					t.Errorf("Expected 1 POI (City), got %d", len(pois))
					return
				}
				if pois[0].Category != "city" {
					t.Errorf("Expected city, got %s", pois[0].Category)
				}
			},
		},
		{
			name:        "Zoom on Town (City out of bounds)",
			queryParams: "?minLat=10.05&maxLat=10.25&minLon=10.05&maxLon=10.25",
			// Bounds exclude 10.0,10.0 (City) but include 10.1 (Town) and 10.2 (Village)
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, pois []*model.POI) {
				if len(pois) != 1 {
					t.Errorf("Expected 1 POI (Town), got %d", len(pois))
					return
				}
				if pois[0].Category != "town" {
					t.Errorf("Expected town, got %s", pois[0].Category)
				}
			},
		},
		{
			name:        "Zoom on Village (Only)",
			queryParams: "?minLat=10.15&maxLat=10.25&minLon=10.15&maxLon=10.25",
			// Bounds exclude City and Town, include Village
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, pois []*model.POI) {
				if len(pois) != 1 {
					t.Errorf("Expected 1 POI, got %d", len(pois))
					return
				}
				if pois[0].Category != "village" {
					t.Errorf("Expected village, got %s", pois[0].Category)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/map/settlements"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			handler.HandleSettlements(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.verify != nil && w.Code == http.StatusOK {
				var resp []*model.POI
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				tt.verify(t, resp)
			}
		})
	}
}
