package store

import (
	"context"
	"path/filepath"
	"testing"

	"phileasgo/pkg/db"
	"phileasgo/pkg/model"
)

func TestSQLiteStore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Init DB
	d, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer d.Close()

	store := NewSQLiteStore(d)
	ctx := context.Background()

	testPOI(t, ctx, store)
	testPOIsBatch(t, ctx, store)
	testMSFS(t, ctx, store)
	testHierarchy(t, ctx, store)
	testArticles(t, ctx, store)
	testCache(t, ctx, store)
	testState(t, ctx, store)
}

func testPOI(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("POI", func(t *testing.T) {
		poi := &model.POI{
			WikidataID:       "Q123",
			Source:           "wikidata",
			Category:         "City",
			SpecificCategory: "Medieval City",
			Lat:              10.0,
			Lon:              20.0,
			Sitelinks:        5,
			TriggerQID:       "Q999",
			NameEn:           "Test City",
		}

		if err := store.SavePOI(ctx, poi); err != nil {
			t.Errorf("SavePOI failed: %v", err)
		}

		loadedPOI, err := store.GetPOI(ctx, "Q123")
		if err != nil {
			t.Errorf("GetPOI failed: %v", err)
		}
		if loadedPOI == nil {
			t.Fatal("GetPOI returned nil")
		}
		if loadedPOI.WikidataID != "Q123" {
			t.Errorf("Expected Q123, got %s", loadedPOI.WikidataID)
		}
		if loadedPOI.NameEn != "Test City" {
			t.Errorf("NameEn mismatch: %v", loadedPOI.NameEn)
		}
		// Test SpecificCategory persistence
		if loadedPOI.SpecificCategory != "Medieval City" {
			t.Errorf("SpecificCategory mismatch: expected 'Medieval City', got '%s'", loadedPOI.SpecificCategory)
		}
	})
}

func testPOIsBatch(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("POIsBatch", func(t *testing.T) {
		p1 := &model.POI{WikidataID: "QB1", Category: "A"}
		p2 := &model.POI{WikidataID: "QB2", Category: "B"}
		_ = store.SavePOI(ctx, p1)
		_ = store.SavePOI(ctx, p2)

		batch, err := store.GetPOIsBatch(ctx, []string{"QB1", "QB2", "QB_MISSING"})
		if err != nil {
			t.Fatalf("GetPOIsBatch failed: %v", err)
		}
		if len(batch) != 2 {
			t.Errorf("Expected 2 POIs, got %d", len(batch))
		}
		if batch["QB1"].Category != "A" || batch["QB2"].Category != "B" {
			t.Errorf("Batch content mismatch")
		}
		if _, ok := batch["QB_MISSING"]; ok {
			t.Errorf("Did not expect missing POI in batch")
		}
	})
}

func testMSFS(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("MSFS_POI", func(t *testing.T) {
		msfs := &model.MSFSPOI{
			Type:      "Airport",
			Name:      "Test Airport",
			Ident:     "KTEST",
			Lat:       30.0,
			Lon:       -80.0,
			Elevation: 100.0,
		}
		if err := store.SaveMSFSPOI(ctx, msfs); err != nil {
			t.Errorf("SaveMSFSPOI failed: %v", err)
		}
		if msfs.ID == 0 {
			t.Errorf("Expected auto-increment ID to be set")
		}
		loadedMSFS, err := store.GetMSFSPOI(ctx, msfs.ID)
		if err != nil {
			t.Errorf("GetMSFSPOI failed: %v", err)
		}
		if loadedMSFS.Ident != "KTEST" {
			t.Errorf("Expected KTEST, got %s", loadedMSFS.Ident)
		}
	})
}

func testHierarchy(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("Hierarchy", func(t *testing.T) {
		h := &model.WikidataHierarchy{
			QID:     "Q888",
			Name:    "Some Place",
			Parents: []string{"Q999", "Q1000"},
		}
		if err := store.SaveHierarchy(ctx, h); err != nil {
			t.Errorf("SaveHierarchy failed: %v", err)
		}
		loadedH, err := store.GetHierarchy(ctx, "Q888")
		if err != nil {
			t.Errorf("GetHierarchy failed: %v", err)
		}
		if len(loadedH.Parents) != 2 {
			t.Errorf("Hierarchy parents count expectation failed")
		}
	})
}

func testArticles(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("Articles", func(t *testing.T) {
		art := &model.Article{
			UUID:    "uuid-1",
			Title:   "Article 1",
			Names:   map[string]string{"en": "Article 1"},
			Lengths: map[string]int{"en": 500},
		}
		if err := store.SaveArticle(ctx, art); err != nil {
			t.Errorf("SaveArticle failed: %v", err)
		}
		loadedArt, err := store.GetArticle(ctx, "uuid-1")
		if err != nil {
			t.Errorf("GetArticle failed: %v", err)
		}
		if loadedArt.Title != "Article 1" {
			t.Errorf("Article title mismatch")
		}
	})
}

func testCache(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("Cache", func(t *testing.T) {
		if err := store.SetCache(ctx, "foo", []byte("bar")); err != nil {
			t.Errorf("SetCache failed: %v", err)
		}
		val, hit := store.GetCache(ctx, "foo")
		if !hit {
			t.Error("Expected cache hit")
		}
		if string(val) != "bar" {
			t.Errorf("Expected 'bar', got '%s'", string(val))
		}
	})
}

func testState(t *testing.T, ctx context.Context, store *SQLiteStore) {
	t.Run("State", func(t *testing.T) {
		if err := store.SetState(ctx, "my_key", "my_val"); err != nil {
			t.Errorf("SetState failed: %v", err)
		}
		sVal, sHit := store.GetState(ctx, "my_key")
		if !sHit {
			t.Error("Expected state hit")
		}
		if sVal != "my_val" {
			t.Errorf("Expected 'my_val', got '%s'", sVal)
		}
	})
}
