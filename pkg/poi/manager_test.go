package poi

import (
	"context"
	"fmt"
	"math"
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
func (s *MockStore) GetPOI(ctx context.Context, id string) (*model.POI, error) {
	return s.savedPOIs[id], nil
}
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
func (s *MockStore) DeleteState(ctx context.Context, key string) error       { return nil }
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

// Mocks for River Detection
type MockRiverSentinel struct {
	candidate *model.RiverCandidate
}

func (m *MockRiverSentinel) Update(lat, lon, heading float64) *model.RiverCandidate {
	return m.candidate
}

type MockLoader struct {
	poiMap map[string][]*model.POI
	stored map[string]*model.POI // Map to "hydrate" into the mock store
	store  *MockStore            // Connection to the store
	err    error
}

func (m *MockLoader) EnsurePOIsLoaded(ctx context.Context, qids []string, lat, lon float64) error {
	if m.err != nil {
		return m.err
	}
	for _, qid := range qids {
		if p, ok := m.stored[qid]; ok && m.store != nil {
			m.store.SavePOI(ctx, p)
		}
	}
	return nil
}

func (m *MockLoader) GetPOIsNear(ctx context.Context, lat, lon, radiusMeters float64) ([]*model.POI, error) {
	key := fmt.Sprintf("%.1f,%.1f", lat, lon)
	return m.poiMap[key], m.err
}

func TestManager_TrackPOI_Nameless(t *testing.T) {
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), NewMockStore(), nil)
	ctx := context.Background()

	p := &model.POI{
		WikidataID: "Q_Nameless",
	}
	// TrackPOI calls upsertInternal which should now drop nameless POIs
	err := mgr.TrackPOI(ctx, p)
	if err != nil {
		t.Errorf("TrackPOI returned error for nameless POI: %v", err)
	}

	// Verify it's NOT tracked in memory
	tracked := mgr.GetTrackedPOIs()
	if len(tracked) != 0 {
		t.Errorf("Expected 0 tracked POIs, got %d", len(tracked))
	}
}

func TestManager_ActiveTracking(t *testing.T) {
	mockStore := NewMockStore()
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), mockStore, nil)
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
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), mockStore, nil)
	ctx := context.Background()

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now()

	pOld := &model.POI{WikidataID: "QOld", NameEn: "Old POI", CreatedAt: oldTime}
	pNew := &model.POI{WikidataID: "QNew", NameEn: "New POI", CreatedAt: newTime}

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
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), mockStore, nil)
	ctx := context.Background()

	// 1. Initial State provided by "Simulate Narrator"
	t1 := time.Now()
	p := &model.POI{
		WikidataID: "QTest",
		NameEn:     "Test POI",
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
		NameEn:     "Test POI",
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
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), NewMockStore(), nil)
	ctx := context.Background()

	pois := []*model.POI{
		{WikidataID: "P1", NameEn: "P1", Score: 10.0, Visibility: 1.0, IsVisible: true}, // Best
		{WikidataID: "P2", NameEn: "P2", Score: 5.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P3", NameEn: "P3", Score: 8.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P4", NameEn: "P4", Score: 2.0, Visibility: 1.0, IsVisible: true}, // Worst
	}

	for _, p := range pois {
		_ = mgr.TrackPOI(ctx, p)
	}

	if mgr.ActiveCount() != 4 {
		t.Errorf("Expected 4 active POIs, got %d", mgr.ActiveCount())
	}

	// 1. GetNarrationCandidates (Best)
	candidates := mgr.GetNarrationCandidates(1, nil)
	if len(candidates) == 0 || candidates[0].WikidataID != "P1" {
		t.Errorf("GetNarrationCandidates(limit=1) failed. Want P1, got %v", candidates)
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

	// 3. GetNarrationCandidates (Sort)
	sorted := mgr.GetNarrationCandidates(3, nil)
	if len(sorted) != 3 {
		t.Fatalf("GetNarrationCandidates(3) expected 3, got %d", len(sorted))
	}
	if sorted[0].WikidataID != "P1" || sorted[1].WikidataID != "P3" || sorted[2].WikidataID != "P2" {
		t.Errorf("Candidates not sorted correctly. Got %v %v %v", sorted[0].WikidataID, sorted[1].WikidataID, sorted[2].WikidataID)
	}
}

func TestManager_ResetLastPlayed(t *testing.T) {
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), NewMockStore(), nil)
	ctx := context.Background()

	p := &model.POI{
		WikidataID: "Q1",
		NameEn:     "Q1",
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

func TestManager_GetPOIsForUI(t *testing.T) {
	cfg := &config.Config{}
	cfg.Narrator.RepeatTTL = config.Duration(1 * time.Hour)
	mgr := NewManager(config.NewProvider(cfg, nil), NewMockStore(), nil)
	ctx := context.Background()

	now := time.Now()
	pois := []*model.POI{
		{WikidataID: "P1", NameEn: "P1", Score: 10.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P2", NameEn: "P2", Score: 8.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P3", NameEn: "P3", Score: 8.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P4", NameEn: "P4", Score: 5.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P5", NameEn: "P5", Score: 2.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P_Played", NameEn: "P_Played", Score: 1.0, Visibility: 1.0, IsVisible: true, LastPlayed: now},
		{WikidataID: "P_Played_High", NameEn: "P_Played_High", Score: 20.0, Visibility: 1.0, IsVisible: true, LastPlayed: now},
		{WikidataID: "P_Invisible", NameEn: "P_Invisible", Score: 15.0, Visibility: 1.0, IsVisible: false},
		{WikidataID: "P_Airport", NameEn: "P_Airport", Score: 5.0, Visibility: 1.0, IsVisible: true, Category: "Aerodrome"},
	}

	for _, p := range pois {
		_ = mgr.TrackPOI(ctx, p)
	}

	tests := []struct {
		name          string
		mode          string
		targetCount   int
		minScore      float64
		wantIDs       []string
		wantThreshold float64
	}{
		{
			name:          "Fixed Mode - Threshold 7",
			mode:          "fixed",
			minScore:      7.0,
			wantIDs:       []string{"P1", "P2", "P3", "P_Played", "P_Played_High"},
			wantThreshold: 7.0,
		},
		{
			name:          "Fixed Mode - Threshold 11",
			mode:          "fixed",
			minScore:      11.0,
			wantIDs:       []string{"P_Played", "P_Played_High"},
			wantThreshold: 11.0,
		},
		{
			name:          "Adaptive Mode - Target 1",
			mode:          "adaptive",
			targetCount:   1,
			wantIDs:       []string{"P1", "P_Played", "P_Played_High"},
			wantThreshold: 10.0,
		},
		{
			name:          "Adaptive Mode - Target 2 (Ties included)",
			mode:          "adaptive",
			targetCount:   2,
			wantIDs:       []string{"P1", "P2", "P3", "P_Played", "P_Played_High"},
			wantThreshold: 8.0,
		},
		{
			name:          "Adaptive Mode - Target 4",
			mode:          "adaptive",
			targetCount:   4,
			wantIDs:       []string{"P1", "P2", "P3", "P4", "P_Played", "P_Played_High", "P_Airport"},
			wantThreshold: 5.0,
		},
		{
			name:          "Adaptive Mode - Target 10 (Exhausted)",
			mode:          "adaptive",
			targetCount:   10,
			wantIDs:       []string{"P1", "P2", "P3", "P4", "P5", "P_Played", "P_Played_High", "P_Airport"},
			wantThreshold: -math.MaxFloat64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, threshold := mgr.GetPOIsForUI(tt.mode, tt.targetCount, tt.minScore)
			if threshold != tt.wantThreshold {
				t.Errorf("got threshold %v, want %v", threshold, tt.wantThreshold)
			}

			if len(got) != len(tt.wantIDs) {
				t.Errorf("got %d POIs, want %d", len(got), len(tt.wantIDs))
			}

			foundIDs := make(map[string]bool)
			for _, p := range got {
				foundIDs[p.WikidataID] = true
			}

			for _, wantID := range tt.wantIDs {
				if !foundIDs[wantID] {
					t.Errorf("expected POI %s not found in result", wantID)
				}
			}
		})
	}
}

func TestManager_GetNarrationCandidates(t *testing.T) {
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), NewMockStore(), nil)
	ctx := context.Background()

	now := time.Now()
	cfg := config.Config{}
	cfg.Narrator.RepeatTTL = config.Duration(1 * time.Hour) // Cooldown of 1 hour
	mgr.config = config.NewProvider(&cfg, nil)

	pois := []*model.POI{
		{WikidataID: "P1", NameEn: "P1", Score: 10.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P2", NameEn: "P2", Score: 8.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P_Cooldown", NameEn: "P_Cooldown", Score: 12.0, Visibility: 1.0, IsVisible: true, LastPlayed: now}, // Best but Cooldown
		{WikidataID: "P_Invisible", NameEn: "P_Invisible", Score: 15.0, Visibility: 1.0, IsVisible: false},               // Best but Invisible
		{WikidataID: "P_Airport", NameEn: "P_Airport", Score: 5.0, Visibility: 1.0, IsVisible: true, Category: "Aerodrome"},
	}

	for _, p := range pois {
		_ = mgr.TrackPOI(ctx, p)
	}

	floatPtr := func(v float64) *float64 { return &v }

	tests := []struct {
		name       string
		isOnGround bool
		limit      int
		minScore   *float64
		wantIDs    []string
	}{
		{
			name:       "Airborne - Best Candidates (Cooldown/Invisible Filtered)",
			isOnGround: false,
			limit:      10,
			minScore:   nil,
			wantIDs:    []string{"P1", "P2", "P_Airport"},
		},
		{
			name:       "Ground - No longer filtered (Stage check handled by Narrator)",
			isOnGround: true,
			limit:      10,
			minScore:   nil,
			wantIDs:    []string{"P1", "P2", "P_Airport"},
		},
		{
			name:       "Score Threshold (MinScore 9.0)",
			isOnGround: false,
			limit:      10,
			minScore:   floatPtr(9.0),
			wantIDs:    []string{"P1"}, // Only P1(10) > 9
		},
		{
			name:       "Limit Constraints (Top 1)",
			isOnGround: false,
			limit:      1,
			minScore:   nil,
			wantIDs:    []string{"P1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.GetNarrationCandidates(tt.limit, tt.minScore)

			if len(got) != len(tt.wantIDs) {
				t.Errorf("got %d candidates, want %d", len(got), len(tt.wantIDs))
			}

			for i, wantID := range tt.wantIDs {
				if i < len(got) && got[i].WikidataID != wantID {
					t.Errorf("rank %d: got %s, want %s", i, got[i].WikidataID, wantID)
				}
			}
		})
	}

	// Double Check BestCandidate logic
	best := mgr.GetNarrationCandidates(1, nil)
	if len(best) == 0 || best[0].WikidataID != "P1" {
		t.Errorf("GetNarrationCandidates(1): Expected P1, got %v", best)
	}
}

func TestManager_CountScoredAbove_Competition(t *testing.T) {
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), NewMockStore(), nil)
	ctx := context.Background()
	cfg := config.Config{}
	cfg.Narrator.RepeatTTL = config.Duration(1 * time.Hour)
	mgr.config = config.NewProvider(&cfg, nil)

	now := time.Now()

	// Scenario:
	// P1: Score 10, Visibility 1.0, Playable -> Combined 10, should count
	// P2: Score 10, Visibility 1.0, Played Recently (Cooldown) -> Should NOT count (Silent)
	// P3: Score 10, Visibility 0.0, Playable -> Combined 0, should NOT count (behind/invisible)
	// P4: Score 10, Visibility 0.5, Playable -> Combined 5, should count (threshold 4)
	pois := []*model.POI{
		{WikidataID: "P1", NameEn: "P1", Score: 10.0, Visibility: 1.0, IsVisible: true},
		{WikidataID: "P2", NameEn: "P2", Score: 10.0, Visibility: 1.0, IsVisible: true, LastPlayed: now},
		{WikidataID: "P3", NameEn: "P3", Score: 10.0, Visibility: 0.0, IsVisible: false}, // Behind aircraft
		{WikidataID: "P4", NameEn: "P4", Score: 10.0, Visibility: 0.5, IsVisible: true},  // Partially visible
	}

	for _, p := range pois {
		_ = mgr.TrackPOI(ctx, p)
	}

	// Test 1: Threshold 5.0
	// P1: 10*1.0=10 > 5 ✓
	// P2: On cooldown ✗
	// P3: 10*0.0=0 > 5 ✗
	// P4: 10*0.5=5 > 5 ✗ (not strictly greater)
	count := mgr.CountScoredAbove(5.0, 100)
	if count != 1 {
		t.Errorf("CountScoredAbove(5.0) expected 1, got %d", count)
	}

	// Test 2: Threshold 4.0
	// P1: 10 > 4 ✓
	// P2: Cooldown ✗
	// P3: 0 > 4 ✗
	// P4: 5 > 4 ✓
	count = mgr.CountScoredAbove(4.0, 100)
	if count != 2 {
		t.Errorf("CountScoredAbove(4.0) expected 2, got %d", count)
	}

	// Test 3: Zero visibility POI never counts
	count = mgr.CountScoredAbove(0.0, 100)
	// P1: 10 > 0 ✓, P4: 5 > 0 ✓ (P2 cooldown, P3 combined=0 not > 0)
	if count != 2 {
		t.Errorf("CountScoredAbove(0.0) expected 2, got %d", count)
	}
}

func TestManager_UpdateRivers(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), store, nil)

	sentinel := &MockRiverSentinel{}
	loader := &MockLoader{
		stored: make(map[string]*model.POI),
		store:  store,
	}
	mgr.SetRiverSentinel(sentinel)
	mgr.SetPOILoader(loader)

	// 1. No river ahead
	p, err := mgr.UpdateRivers(ctx, 0, 0, 0)
	if err != nil || p != nil {
		t.Errorf("Expected nil POI, got %v (err: %v)", p, err)
	}

	// 2. River detected, hydrated by QID
	sentinel.candidate = &model.RiverCandidate{
		Name:       "Rhine",
		WikidataID: "Q1",
		Distance:   100,
		ClosestLat: 49.9,
		ClosestLon: 8.1,
	}
	loader.stored["Q1"] = &model.POI{WikidataID: "Q1", Category: "Water", NameEn: "Rhine"}

	p, err = mgr.UpdateRivers(ctx, 49.9, 8.1, 0)
	if err != nil || p == nil {
		t.Fatalf("Hydration failed: %v", err)
	}
	if p.RiverContext == nil || p.RiverContext.DistanceM != 100 {
		t.Errorf("RiverContext not attached correctly: %+v", p.RiverContext)
	}

	// 3. Verify movement and tracking
	sentinel.candidate = &model.RiverCandidate{
		Name:       "Rhine",
		WikidataID: "Q1",
		Distance:   50,
		ClosestLat: 49.95,
		ClosestLon: 8.05,
	}

	p, err = mgr.UpdateRivers(ctx, 49.95, 8.05, 0)
	if err != nil || p == nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify coordinates updated to ClosestLat/Lon
	if p.Lat != 49.95 || p.Lon != 8.05 {
		t.Errorf("Coordinates not updated: got %f,%f want 49.95,8.05", p.Lat, p.Lon)
	}

	// Verify tracked in manager memory
	tracked := mgr.GetTrackedPOIs()
	found := false
	for _, tp := range tracked {
		if tp.WikidataID == "Q1" {
			found = true
			if tp.Lat != 49.95 {
				t.Errorf("Tracked POI coordinates not updated: got %f want 49.95", tp.Lat)
			}
			break
		}
	}
	if !found {
		t.Error("River POI not tracked in manager")
	}

	// 4. River detected, no POI found in store after hydration
	sentinel.candidate = &model.RiverCandidate{
		Name:       "Mystery River",
		WikidataID: "QMystery",
		Distance:   300,
		ClosestLat: 10.0,
		ClosestLon: 10.0,
	}
	// loader.stored doesn't have QMystery

	p, err = mgr.UpdateRivers(ctx, 10.0, 10.0, 0)
	if err != nil {
		t.Fatalf("Update mystery river failed: %v", err)
	}
	if p != nil {
		t.Errorf("Expected nil POI for unknown river, got %v", p)
	}
}
