package db_test

import (
	"path/filepath"
	"phileasgo/pkg/db"
	"testing"
)

func TestDB(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "db_test.db")

	d, err := db.Init(path)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if d == nil {
		t.Fatal("Init() returned nil DB")
	}
	d.Close()
}
