package cache

import (
	"context"
	"phileasgo/pkg/db"
)

// Cacher defines the caching interface.
type Cacher interface {
	GetCache(ctx context.Context, key string) ([]byte, bool)
	SetCache(ctx context.Context, key string, val []byte) error
}

// SQLiteCache implements Cacher using pkg/db.
type SQLiteCache struct {
	db *db.DB
}

// NewSQLiteCache creates a new cache.
func NewSQLiteCache(d *db.DB) *SQLiteCache {
	return &SQLiteCache{db: d}
}

func (c *SQLiteCache) GetCache(ctx context.Context, key string) ([]byte, bool) {
	// Stub: Always miss
	return nil, false
}

func (c *SQLiteCache) SetCache(ctx context.Context, key string, val []byte) error {
	// Stub: Do nothing
	return nil
}
