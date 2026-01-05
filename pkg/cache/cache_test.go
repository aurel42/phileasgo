package cache

import (
	"context"
	"path/filepath"
	"phileasgo/pkg/db"
	"testing"
)

func TestSQLiteCache(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "cache_test.db")
	d, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer d.Close()
	c := NewSQLiteCache(d)

	// Test Get (Stub logic: always miss)
	val, hit := c.GetCache(context.Background(), "any-key")
	if hit {
		t.Error("Expected cache miss, got hit")
	}
	if val != nil {
		t.Error("Expected nil value, got bytes")
	}

	// Test Set (Stub logic: no error)
	err = c.SetCache(context.Background(), "any-key", []byte("data"))
	if err != nil {
		t.Errorf("Set returned error: %v", err)
	}
}
