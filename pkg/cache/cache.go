package cache

import (
	"context"
	"phileasgo/pkg/db"
)

// Cacher defines the caching interface.
type Cacher interface {
	GetCache(ctx context.Context, key string) ([]byte, bool)
	SetCache(ctx context.Context, key string, val []byte) error

	// Geodata-specific: routes to cache_geodata table with radius metadata
	GetGeodataCache(ctx context.Context, key string) (data []byte, radiusM int, found bool)
	SetGeodataCache(ctx context.Context, key string, val []byte, radiusM int, lat, lon float64) error
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
	// Stub: Always miss (real caching done via store.Store)
	return nil, false
}

func (c *SQLiteCache) SetCache(ctx context.Context, key string, val []byte) error {
	// Stub: Do nothing (real caching done via store.Store)
	return nil
}

func (c *SQLiteCache) GetGeodataCache(ctx context.Context, key string) (data []byte, radiusM int, found bool) {
	// Stub: Always miss (real caching done via store.Store)
	return nil, 0, false
}

func (c *SQLiteCache) SetGeodataCache(ctx context.Context, key string, val []byte, radiusM int, lat, lon float64) error {
	// Stub: Do nothing (real caching done via store.Store)
	return nil
}
