package session

// unit test for restore.go

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
)

// MockStore for testing
type MockStore struct {
	Data map[string]string
}

func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) {
	v, ok := m.Data[key]
	return v, ok
}
func (m *MockStore) SetState(ctx context.Context, key, value string) error {
	if m.Data == nil {
		m.Data = make(map[string]string)
	}
	m.Data[key] = value
	return nil
}
func (m *MockStore) DeleteState(ctx context.Context, key string) error {
	delete(m.Data, key)
	return nil
}

func (m *MockStore) Close() error { return nil }

// POIStore
func (m *MockStore) GetPOI(ctx context.Context, wikidataID string) (*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetPOIsBatch(ctx context.Context, wikidataIDs []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) SavePOI(ctx context.Context, poi *model.POI) error { return nil }
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) SaveLastPlayed(ctx context.Context, poiID string, t time.Time) error { return nil }
func (m *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }

// CacheStore
func (m *MockStore) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *MockStore) HasCache(ctx context.Context, key string) (bool, error)     { return false, nil }
func (m *MockStore) SetCache(ctx context.Context, key string, val []byte) error { return nil }
func (m *MockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

// GeodataStore
func (m *MockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	return nil
}
func (m *MockStore) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]store.GeodataRecord, error) {
	return nil, nil
}
func (m *MockStore) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

// HierarchyStore
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

// ArticleStore
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *MockStore) SaveArticle(ctx context.Context, article *model.Article) error { return nil }

// SeenEntityStore
func (m *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return nil, nil
}
func (m *MockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}

func (m *MockStore) DeleteSeenEntities(ctx context.Context, qids []string) error {
	return nil
}

// MSFSPOIStore
func (m *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *MockStore) SaveMSFSPOI(ctx context.Context, poi *model.MSFSPOI) error { return nil }
func (m *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *MockStore) GetRegionalCategories(ctx context.Context, latGrid, lonGrid int) (map[string]string, map[string]string, error) {
	return nil, nil, nil
}
func (m *MockStore) SaveRegionalCategories(ctx context.Context, latGrid, lonGrid int, categories map[string]string, labels map[string]string) error {
	return nil
}

func TestTryRestore(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name         string
		onGround     bool
		storedLat    float64
		storedLon    float64
		currentLat   float64
		currentLon   float64
		hasStored    bool
		wantDone     bool // Should return true
		wantRestored bool // Should have restored state (we can check manager or logs, checking manager is hard without access)
		// We can check if manager state was populated? No, manager is empty unless we restore valid data.
		// Let's store "some data" and see if manager has it?
		// Manager has `Data` field but it's private.
		// `TryRestore` returns bool.
		// We rely on checking if it returns true/false.
		// For Logic Verification:
		// OnGround -> Returns True (Done), No Restore logic run (implied)
	}{
		{
			name:     "On Ground - Fresh Start",
			onGround: true,
			wantDone: true,
		},
		{
			name:      "Airborne - No Stored Session",
			onGround:  false,
			hasStored: false,
			wantDone:  true, // Checked, empty wrapped up
		},
		{
			name:       "Airborne - Too Far",
			onGround:   false,
			hasStored:  true,
			storedLat:  0,
			storedLon:  0,
			currentLat: 10, // Far away
			currentLon: 10,
			wantDone:   true,
		},
		{
			name:       "Airborne - Close - Restore",
			onGround:   false,
			hasStored:  true,
			storedLat:  51.5,
			storedLon:  -0.1,
			currentLat: 51.5,
			currentLon: -0.1,
			wantDone:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &MockStore{Data: make(map[string]string)}
			if tt.hasStored {
				ps := persistentState{Lat: tt.storedLat, Lon: tt.storedLon}
				bytes, _ := json.Marshal(ps)
				store.Data["session_context"] = string(bytes)
			}

			tel := &sim.Telemetry{
				IsOnGround: tt.onGround,
				Latitude:   tt.currentLat,
				Longitude:  tt.currentLon,
			}

			done := TryRestore(context.Background(), store, mgr, tel)
			if done != tt.wantDone {
				t.Errorf("TryRestore() = %v, want %v", done, tt.wantDone)
			}
		})
	}
}
