package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/db"
	"phileasgo/pkg/model"
)

// setupTestStore creates a test database and store for each test.
func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	d, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	store := NewSQLiteStore(d)
	cleanup := func() { d.Close() }
	return store, cleanup
}

// =============================================================================
// POIStore Tests
// =============================================================================

func TestPOIStore_GetRecentlyPlayedPOIs(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name    string
		setup   func(s *SQLiteStore, now time.Time)
		since   time.Time
		wantLen int
		wantIDs []string
	}{
		{
			name:    "empty database",
			setup:   func(s *SQLiteStore, now time.Time) {},
			since:   now.Add(-1 * time.Hour),
			wantLen: 0,
		},
		{
			name: "returns recently played POIs",
			setup: func(s *SQLiteStore, now time.Time) {
				// Played 30 minutes ago
				_ = s.SavePOI(ctx, &model.POI{WikidataID: "Q1", LastPlayed: now.Add(-30 * time.Minute)})
				// Played 2 hours ago (should not be returned for 1hr query)
				_ = s.SavePOI(ctx, &model.POI{WikidataID: "Q2", LastPlayed: now.Add(-2 * time.Hour)})
				// Never played
				_ = s.SavePOI(ctx, &model.POI{WikidataID: "Q3"})
			},
			since:   now.Add(-1 * time.Hour),
			wantLen: 1,
			wantIDs: []string{"Q1"},
		},
		{
			name: "returns multiple POIs sorted by last_played DESC",
			setup: func(s *SQLiteStore, now time.Time) {
				_ = s.SavePOI(ctx, &model.POI{WikidataID: "Q10", LastPlayed: now.Add(-10 * time.Minute)})
				_ = s.SavePOI(ctx, &model.POI{WikidataID: "Q20", LastPlayed: now.Add(-20 * time.Minute)})
				_ = s.SavePOI(ctx, &model.POI{WikidataID: "Q30", LastPlayed: now.Add(-30 * time.Minute)})
			},
			since:   now.Add(-1 * time.Hour),
			wantLen: 3,
			wantIDs: []string{"Q10", "Q20", "Q30"}, // DESC order
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()
			tt.setup(store, now)

			got, err := store.GetRecentlyPlayedPOIs(ctx, tt.since)
			if err != nil {
				t.Fatalf("GetRecentlyPlayedPOIs() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("GetRecentlyPlayedPOIs() got %d POIs, want %d", len(got), tt.wantLen)
			}
			for i, wantID := range tt.wantIDs {
				if i < len(got) && got[i].WikidataID != wantID {
					t.Errorf("GetRecentlyPlayedPOIs()[%d] = %s, want %s", i, got[i].WikidataID, wantID)
				}
			}
		})
	}
}

// =============================================================================
// MSFSPOIStore Tests
// =============================================================================

func TestMSFSPOIStore_CheckMSFSPOI(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		setup  func(s *SQLiteStore)
		lat    float64
		lon    float64
		radius float64
		want   bool
	}{
		{
			name:   "no POIs in database",
			setup:  func(s *SQLiteStore) {},
			lat:    0.0,
			lon:    0.0,
			radius: 1000.0,
			want:   false,
		},
		{
			name: "POI within radius",
			setup: func(s *SQLiteStore) {
				_ = s.SaveMSFSPOI(ctx, &model.MSFSPOI{
					Type: "Airport", Name: "Test", Ident: "KTEST",
					Lat: 52.0, Lon: 13.0, Elevation: 100,
				})
			},
			lat:    52.001,
			lon:    13.001,
			radius: 500.0, // ~150m distance, within 500m
			want:   true,
		},
		{
			name: "POI outside radius",
			setup: func(s *SQLiteStore) {
				_ = s.SaveMSFSPOI(ctx, &model.MSFSPOI{
					Type: "Airport", Name: "Far", Ident: "KFAR",
					Lat: 52.0, Lon: 13.0, Elevation: 100,
				})
			},
			lat:    52.1,
			lon:    13.1,
			radius: 500.0, // ~14km distance, outside 500m
			want:   false,
		},
		{
			name: "multiple POIs, one within radius",
			setup: func(s *SQLiteStore) {
				_ = s.SaveMSFSPOI(ctx, &model.MSFSPOI{Lat: 10.0, Lon: 10.0})
				_ = s.SaveMSFSPOI(ctx, &model.MSFSPOI{Lat: 52.0, Lon: 13.0})
			},
			lat:    52.0005,
			lon:    13.0005,
			radius: 200.0,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()
			tt.setup(store)

			got, err := store.CheckMSFSPOI(ctx, tt.lat, tt.lon, tt.radius)
			if err != nil {
				t.Fatalf("CheckMSFSPOI() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("CheckMSFSPOI() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// HierarchyStore Tests
// =============================================================================

func TestHierarchyStore_Classification(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func(s *SQLiteStore)
		qid          string
		wantCategory string
		wantFound    bool
	}{
		{
			name:         "not found",
			setup:        func(s *SQLiteStore) {},
			qid:          "Q12345",
			wantCategory: "",
			wantFound:    false,
		},
		{
			name: "found with category",
			setup: func(s *SQLiteStore) {
				_ = s.SaveClassification(ctx, "Q100", "City", []string{"Q200"}, "Berlin")
			},
			qid:          "Q100",
			wantCategory: "City",
			wantFound:    true,
		},
		{
			name: "found with empty category",
			setup: func(s *SQLiteStore) {
				_ = s.SaveClassification(ctx, "Q200", "", []string{"Q300"}, "Unknown")
			},
			qid:          "Q200",
			wantCategory: "",
			wantFound:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()
			tt.setup(store)

			gotCat, gotFound, err := store.GetClassification(ctx, tt.qid)
			if err != nil {
				t.Fatalf("GetClassification() error = %v", err)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetClassification() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotCat != tt.wantCategory {
				t.Errorf("GetClassification() category = %q, want %q", gotCat, tt.wantCategory)
			}
		})
	}
}

func TestHierarchyStore_SaveClassification(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		qid      string
		category string
		parents  []string
		label    string
	}{
		{
			name:     "save with all fields",
			qid:      "Q1",
			category: "Building",
			parents:  []string{"Q2", "Q3"},
			label:    "Test Building",
		},
		{
			name:     "save with empty parents",
			qid:      "Q2",
			category: "City",
			parents:  []string{},
			label:    "Test City",
		},
		{
			name:     "save with nil parents",
			qid:      "Q3",
			category: "Landmark",
			parents:  nil,
			label:    "Test Landmark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()

			err := store.SaveClassification(ctx, tt.qid, tt.category, tt.parents, tt.label)
			if err != nil {
				t.Fatalf("SaveClassification() error = %v", err)
			}

			// Verify via GetClassification
			gotCat, gotFound, err := store.GetClassification(ctx, tt.qid)
			if err != nil {
				t.Fatalf("GetClassification() error = %v", err)
			}
			if !gotFound {
				t.Error("GetClassification() not found after save")
			}
			if gotCat != tt.category {
				t.Errorf("GetClassification() = %q, want %q", gotCat, tt.category)
			}

			// Verify via GetHierarchy (includes parents)
			h, err := store.GetHierarchy(ctx, tt.qid)
			if err != nil {
				t.Fatalf("GetHierarchy() error = %v", err)
			}
			if h == nil {
				t.Fatal("GetHierarchy() returned nil")
			}
			if h.Name != tt.label {
				t.Errorf("GetHierarchy().Name = %q, want %q", h.Name, tt.label)
			}
		})
	}
}

// =============================================================================
// SeenEntityStore Tests
// =============================================================================

func TestSeenEntityStore_MarkAndGet(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		entities map[string][]string
		lookup   []string
		wantLen  int
	}{
		{
			name:     "empty entities",
			entities: map[string][]string{},
			lookup:   []string{"Q1"},
			wantLen:  0,
		},
		{
			name: "mark and retrieve",
			entities: map[string][]string{
				"Q1": {"Q100", "Q200"},
				"Q2": {"Q300"},
			},
			lookup:  []string{"Q1", "Q2", "Q3"},
			wantLen: 2,
		},
		{
			name: "mark with empty instances",
			entities: map[string][]string{
				"Q5": {},
			},
			lookup:  []string{"Q5"},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()

			// Mark
			err := store.MarkEntitiesSeen(ctx, tt.entities)
			if err != nil {
				t.Fatalf("MarkEntitiesSeen() error = %v", err)
			}

			// Get
			got, err := store.GetSeenEntitiesBatch(ctx, tt.lookup)
			if err != nil {
				t.Fatalf("GetSeenEntitiesBatch() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("GetSeenEntitiesBatch() got %d, want %d", len(got), tt.wantLen)
			}

			// Verify instances
			for qid, instances := range tt.entities {
				if gotInst, ok := got[qid]; ok {
					if len(gotInst) != len(instances) {
						t.Errorf("instances for %s: got %d, want %d", qid, len(gotInst), len(instances))
					}
				}
			}
		})
	}
}

func TestSeenEntityStore_GetEmptyBatch(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	got, err := store.GetSeenEntitiesBatch(ctx, []string{})
	if err != nil {
		t.Fatalf("GetSeenEntitiesBatch() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("GetSeenEntitiesBatch([]) got %d, want 0", len(got))
	}
}

// =============================================================================
// CacheStore Tests
// =============================================================================

func TestCacheStore_HasCache(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		setup func(s *SQLiteStore)
		key   string
		want  bool
	}{
		{
			name:  "key not found",
			setup: func(s *SQLiteStore) {},
			key:   "missing_key",
			want:  false,
		},
		{
			name: "key found",
			setup: func(s *SQLiteStore) {
				_ = s.SetCache(ctx, "existing_key", []byte("value"))
			},
			key:  "existing_key",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()
			tt.setup(store)

			got, err := store.HasCache(ctx, tt.key)
			if err != nil {
				t.Fatalf("HasCache() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasCache() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheStore_ListCacheKeys(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(s *SQLiteStore)
		prefix  string
		wantLen int
	}{
		{
			name:    "empty cache",
			setup:   func(s *SQLiteStore) {},
			prefix:  "wd_",
			wantLen: 0,
		},
		{
			name: "matching prefix",
			setup: func(s *SQLiteStore) {
				_ = s.SetCache(ctx, "wd_tile_1", []byte("a"))
				_ = s.SetCache(ctx, "wd_tile_2", []byte("b"))
				_ = s.SetCache(ctx, "other_key", []byte("c"))
			},
			prefix:  "wd_",
			wantLen: 2,
		},
		{
			name: "no matching prefix",
			setup: func(s *SQLiteStore) {
				_ = s.SetCache(ctx, "foo", []byte("a"))
				_ = s.SetCache(ctx, "bar", []byte("b"))
			},
			prefix:  "baz_",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()
			tt.setup(store)

			got, err := store.ListCacheKeys(ctx, tt.prefix)
			if err != nil {
				t.Fatalf("ListCacheKeys() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("ListCacheKeys() got %d keys, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// =============================================================================
// GeodataStore Tests
// =============================================================================

func TestGeodataStore_SetAndGet(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		key       string
		data      []byte
		radius    int
		wantFound bool
	}{
		{
			name:      "store and retrieve",
			key:       "tile_123",
			data:      []byte(`{"lat":52.0,"lon":13.0}`),
			radius:    8500,
			wantFound: true,
		},
		{
			name:      "large data (tests compression)",
			key:       "tile_big",
			data:      make([]byte, 10000), // 10KB of zeros
			radius:    10000,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := setupTestStore(t)
			defer cleanup()

			// Set
			err := store.SetGeodataCache(ctx, tt.key, tt.data, tt.radius)
			if err != nil {
				t.Fatalf("SetGeodataCache() error = %v", err)
			}

			// Get
			gotData, gotRadius, gotFound := store.GetGeodataCache(ctx, tt.key)
			if gotFound != tt.wantFound {
				t.Errorf("GetGeodataCache() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotRadius != tt.radius {
				t.Errorf("GetGeodataCache() radius = %d, want %d", gotRadius, tt.radius)
			}
			if len(gotData) != len(tt.data) {
				t.Errorf("GetGeodataCache() data len = %d, want %d", len(gotData), len(tt.data))
			}
		})
	}
}

func TestGeodataStore_GetMissing(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _, found := store.GetGeodataCache(ctx, "nonexistent")
	if found {
		t.Error("GetGeodataCache() should return false for missing key")
	}
}

// =============================================================================
// Cache Interface Tests (Get/Set without context)
// =============================================================================

func TestCacheInterface_GetSet(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Test Set (implements cache.Cacher)
	err := store.Set("interface_key", []byte("interface_value"))
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Test Get (implements cache.Cacher)
	got, hit := store.Get("interface_key")
	if !hit {
		t.Error("Get() expected hit")
	}
	if string(got) != "interface_value" {
		t.Errorf("Get() = %q, want %q", string(got), "interface_value")
	}

	// Test miss
	_, hit = store.Get("missing")
	if hit {
		t.Error("Get() expected miss for nonexistent key")
	}
}
