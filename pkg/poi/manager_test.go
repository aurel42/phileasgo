package poi

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
)

// MockStore for testing
type MockStore struct {
	savedPOIs map[string]*model.POI
}

func NewMockStore() *MockStore {
	return &MockStore{savedPOIs: make(map[string]*model.POI)}
}

func (s *MockStore) SavePOI(ctx context.Context, p *model.POI) error {
	s.savedPOIs[p.WikidataID] = p
	return nil
}

func (s *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (s *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }

// Stubs for other interface methods...
func (s *MockStore) GetPOI(ctx context.Context, id string) (*model.POI, error) { return nil, nil }
func (s *MockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (s *MockStore) SaveMSFSPOI(ctx context.Context, p *model.MSFSPOI) error { return nil }
func (s *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (s *MockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (s *MockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error { return nil }
func (s *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (s *MockStore) SaveArticle(ctx context.Context, a *model.Article) error    { return nil }
func (s *MockStore) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (s *MockStore) SetCache(ctx context.Context, key string, val []byte) error { return nil }
func (s *MockStore) HasCache(ctx context.Context, key string) (bool, error)     { return false, nil }
func (s *MockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (s *MockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (s *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int) error {
	return nil
}
func (s *MockStore) GetState(ctx context.Context, key string) (string, bool) { return "", false }
func (s *MockStore) SetState(ctx context.Context, key, val string) error     { return nil }
func (s *MockStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	return "", false, nil
}
func (s *MockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}
func (s *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return make(map[string][]string), nil
}
func (s *MockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}
func (s *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (s *MockStore) Close() error { return nil }

func TestManager_ActiveTracking(t *testing.T) {
	mockStore := NewMockStore()
	mgr := NewManager(&config.Config{}, mockStore, nil)
	ctx := context.Background()

	p1 := &model.POI{WikidataID: "Q1", NameEn: "P1", CreatedAt: time.Now()}
	p2 := &model.POI{WikidataID: "Q2", NameEn: "P2", CreatedAt: time.Now()}

	// 1. Upsert
	if err := mgr.UpsertPOI(ctx, p1); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if err := mgr.UpsertPOI(ctx, p2); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// 2. Verify Tracking
	tracked := mgr.GetTrackedPOIs()
	if len(tracked) != 2 {
		t.Errorf("Expected 2 tracked POIs, got %d", len(tracked))
	}

	// 3. Verify Thread Safety (Race detector will catch issues if run with -race)
	go func() {
		_ = mgr.UpsertPOI(ctx, &model.POI{WikidataID: "Q3", NameEn: "P3"})
	}()
	_ = mgr.GetTrackedPOIs()

}

func TestManager_Prune(t *testing.T) {
	mockStore := NewMockStore()
	mgr := NewManager(&config.Config{}, mockStore, nil)
	ctx := context.Background()

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now()

	pOld := &model.POI{WikidataID: "QOld", CreatedAt: oldTime}
	pNew := &model.POI{WikidataID: "QNew", CreatedAt: newTime}

	_ = mgr.UpsertPOI(ctx, pOld)
	_ = mgr.UpsertPOI(ctx, pNew)

	// Prune older than 1 hour
	count := mgr.PruneTracked(1 * time.Hour)
	if count != 1 {
		t.Errorf("Expected 1 pruned POI, got %d", count)
	}

	tracked := mgr.GetTrackedPOIs()
	if len(tracked) != 1 {
		t.Fatalf("Expected 1 currently tracked POI, got %d", len(tracked))
	}
	if tracked[0].WikidataID != "QNew" {
		t.Errorf("Expected QNew to remain, got %s", tracked[0].WikidataID)
	}
}

func TestManager_UpdateExistingPOI_LastPlayed_Safety(t *testing.T) {
	mockStore := NewMockStore()
	mgr := NewManager(&config.Config{}, mockStore, nil)
	ctx := context.Background()

	// 1. Initial State provided by "Simulate Narrator"
	t1 := time.Now()
	p := &model.POI{
		WikidataID: "QTest",
		LastPlayed: t1,
	}
	// Add to Manager (simulate Narrator play and track)
	if err := mgr.TrackPOI(ctx, p); err != nil {
		t.Fatalf("Failed to track POI: %v", err)
	} // LastPlayed = T1 in memory

	// 2. Hydrate with OLDER data (simulate delayed DB read)
	t0 := t1.Add(-10 * time.Minute)
	pOld := &model.POI{
		WikidataID: "QTest",
		LastPlayed: t0,
	}

	// Simulate Wikidata hydrating from DB
	if err := mgr.TrackPOI(ctx, pOld); err != nil {
		t.Fatalf("Failed to track old POI: %v", err)
	}

	// 3. Verify State
	finalP, _ := mgr.GetPOI(ctx, "QTest")
	if finalP.LastPlayed.Equal(t0) {
		t.Errorf("LastPlayed reverted to older timestamp! Got %v, expected %v", finalP.LastPlayed, t1)
	}
	if !finalP.LastPlayed.Equal(t1) {
		t.Errorf("LastPlayed mismatch! Got %v, expected %v", finalP.LastPlayed, t1)
	}
}

func TestManager_CandidateLogic(t *testing.T) {
	mgr := NewManager(&config.Config{}, NewMockStore(), nil)
	ctx := context.Background()

	pois := []*model.POI{
		{WikidataID: "P1", Score: 10.0}, // Best
		{WikidataID: "P2", Score: 5.0},
		{WikidataID: "P3", Score: 8.0},
		{WikidataID: "P4", Score: 2.0}, // Worst
	}

	for _, p := range pois {
		_ = mgr.TrackPOI(ctx, p)
	}

	if mgr.ActiveCount() != 4 {
		t.Errorf("Expected 4 active POIs, got %d", mgr.ActiveCount())
	}

	// 1. GetBestCandidate
	best := mgr.GetBestCandidate()
	if best == nil || best.WikidataID != "P1" {
		t.Errorf("GetBestCandidate failed. Want P1, got %v", best)
	}

	// 2. CountScoredAbove
	c8 := mgr.CountScoredAbove(8.0, 100)
	if c8 != 1 { // Only P1(10) > 8. P3(8) is not > 8
		t.Errorf("CountScoredAbove(8.0) expected 1, got %d", c8)
	}
	c4 := mgr.CountScoredAbove(4.0, 100)
	if c4 != 3 { // P1, P3, P2
		t.Errorf("CountScoredAbove(4.0) expected 3, got %d", c4)
	}

	// 3. GetCandidates (Sort)
	sorted := mgr.GetCandidates(3)
	if len(sorted) != 3 {
		t.Fatalf("GetCandidates(3) expected 3, got %d", len(sorted))
	}
	if sorted[0].WikidataID != "P1" || sorted[1].WikidataID != "P3" || sorted[2].WikidataID != "P2" {
		t.Errorf("Candidates not sorted correctly. Got %v %v %v", sorted[0].WikidataID, sorted[1].WikidataID, sorted[2].WikidataID)
	}
}

func TestManager_ResetLastPlayed(t *testing.T) {
	mgr := NewManager(&config.Config{}, NewMockStore(), nil)
	ctx := context.Background()

	p := &model.POI{
		WikidataID: "Q1",
		LastPlayed: time.Now(),
	}
	_ = mgr.TrackPOI(ctx, p)

	mgr.ResetLastPlayed(ctx, 0, 0, 1000)

	// Check Memory State using GetPOI
	got, _ := mgr.GetPOI(ctx, "Q1")
	if !got.LastPlayed.IsZero() {
		t.Error("ResetLastPlayed failed to clear memory cache")
	}
}
