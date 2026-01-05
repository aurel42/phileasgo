package maintenance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/db"
	"phileasgo/pkg/store"
)

func TestMaintenance(t *testing.T) {
	// Setup DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "maint_test.db")
	d, err := db.Init(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	s := store.NewSQLiteStore(d)
	ctx := context.Background()

	// 1. Test ImportMSFS
	csvPath := filepath.Join(tempDir, "Master.csv")
	// Simulate BOM by prepending \ufeff
	csvContent := "\ufeffType,Name,Ident,Latitude,Longitude,Elevation\n" +
		"Airport,Test Airport,KTEST,30.0,-80.0,100.0\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run Import (via Run, but Run does Prune too, so we test both effectively if we setup cache first)

	// Setup Cache for Pruning Test
	// Insert old entry (40 days old)
	oldDeadline := time.Now().Add(-40 * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	_, err = d.Exec("INSERT INTO cache (key, value, created_at) VALUES (?, ?, ?)", "old-key", "old-val", oldDeadline)
	if err != nil {
		t.Fatal(err)
	}
	// Insert new entry (1 day old)
	newDeadline := time.Now().Add(-1 * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	_, err = d.Exec("INSERT INTO cache (key, value, created_at) VALUES (?, ?, ?)", "new-key", "new-val", newDeadline)
	if err != nil {
		t.Fatal(err)
	}

	// Run Maintenance
	if err := Run(ctx, s, d, csvPath); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify Import
	// We lack a specific "GetMSFSPOIByIdent" in Store to easily verify, but we can verify State was set.
	// We can check the TYPE field now.
	row := d.QueryRow("SELECT type, ident FROM msfs_poi LIMIT 1")
	var pType, pIdent string
	if err := row.Scan(&pType, &pIdent); err != nil {
		t.Errorf("Failed to query imported POI: %v", err)
	}
	if pIdent != "KTEST" {
		t.Errorf("Expected Ident KTEST, got %s", pIdent)
	}
	if pType != "Airport" {
		t.Errorf("Expected Type 'Airport', got '%s'. Suspect BOM issue.", pType)
	}
	// Verify State
	_, found := s.GetState(ctx, msfsPOITableStateKey)
	if !found {
		t.Error("State not updated after import")
	}

	// Verify Pruning
	// Old key should be gone
	var count int
	if err := d.QueryRow("SELECT count(*) FROM cache WHERE key = ?", "old-key").Scan(&count); err != nil {
		t.Errorf("Failed to query cache count: %v", err)
	}
	if count != 0 {
		t.Error("Old cache entry was not pruned")
	}
	// New key should remain
	if err := d.QueryRow("SELECT count(*) FROM cache WHERE key = ?", "new-key").Scan(&count); err != nil {
		t.Errorf("Failed to query cache count: %v", err)
	}
	if count != 1 {
		t.Error("New cache entry was incorrectly pruned")
	}
}
