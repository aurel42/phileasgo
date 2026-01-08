package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Register driver
)

// DB wraps the sql.DB connection.
type DB struct {
	*sql.DB
}

// Init opens the database and runs migrations.
func Init(path string) (*DB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	// Enable WAL mode for better concurrency and set busy timeout
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=30000;"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	d := &DB{db}
	// Enforce single connection to avoid SQLITE_BUSY errors during concurrent writes
	db.SetMaxOpenConns(1)

	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return d, nil
}

// PruneCache removes cache entries older than the specified duration.
func (d *DB) PruneCache(olderThan time.Duration) error {
	// Format time compatible with SQLite DEFAULT CURRENT_TIMESTAMP (YYYY-MM-DD HH:MM:SS)
	deadline := time.Now().Add(-olderThan).UTC().Format("2006-01-02 15:04:05")
	_, err := d.Exec("DELETE FROM cache WHERE created_at < ?", deadline)
	return err
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS poi (
			wikidata_id TEXT PRIMARY KEY,
			source TEXT,
			category TEXT,
			specific_category TEXT,
			lat REAL,
			lon REAL,
			sitelinks INTEGER,
			name_en TEXT,
			name_local TEXT,
			name_user TEXT,
			wp_url TEXT,
			wp_article_length INTEGER,
			trigger_qid TEXT,
			last_played DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_msfs_poi BOOLEAN DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS msfs_poi (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,
			name TEXT,
			ident TEXT,
			lat REAL,
			lon REAL,
			elevation REAL
		);`,
		`CREATE TABLE IF NOT EXISTS wikidata_hierarchy (
			qid TEXT PRIMARY KEY,
			name TEXT,
			parents TEXT,
			category TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS wikipedia_articles (
			uuid TEXT PRIMARY KEY,
			title TEXT,
			url TEXT,
			names TEXT,
			text TEXT,
			lengths TEXT,
			thumbnail_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS persistent_state (
			key TEXT PRIMARY KEY,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY,
			value BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS cache_geodata (
			key TEXT PRIMARY KEY,
			data BLOB,
			radius_m INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS seen_entities (
			qid TEXT PRIMARY KEY,
			instances TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("exec error: %w query: %s", err, q)
		}
	}

	// Migration: Add is_msfs_poi if missing
	var colCount int
	err := d.QueryRow("SELECT count(*) FROM pragma_table_info('poi') WHERE name='is_msfs_poi'").Scan(&colCount)
	if err == nil && colCount == 0 {
		if _, err := d.Exec("ALTER TABLE poi ADD COLUMN is_msfs_poi BOOLEAN DEFAULT 0"); err != nil {
			return fmt.Errorf("failed to add is_msfs_poi column: %w", err)
		}
	}

	return nil
}
