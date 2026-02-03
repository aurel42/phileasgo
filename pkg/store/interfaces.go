package store

import (
	"context"
	"time"

	"phileasgo/pkg/model"
)

// POIStore handles Point of Interest persistence.
type POIStore interface {
	GetPOI(ctx context.Context, wikidataID string) (*model.POI, error)
	GetPOIsBatch(ctx context.Context, wikidataIDs []string) (map[string]*model.POI, error)
	SavePOI(ctx context.Context, poi *model.POI) error
	GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error)
	ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error
}

// CacheStore handles generic key-value caching.
type CacheStore interface {
	GetCache(ctx context.Context, key string) ([]byte, bool)
	HasCache(ctx context.Context, key string) (bool, error)
	SetCache(ctx context.Context, key string, val []byte) error
	ListCacheKeys(ctx context.Context, prefix string) ([]string, error)
}

// GeodataRecord represents metadata for a cached tile.
type GeodataRecord struct {
	Key    string
	Lat    float64
	Lon    float64
	Radius int
}

// GeodataStore handles geodata-specific caching with radius metadata.
type GeodataStore interface {
	GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool)
	SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error
	GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]GeodataRecord, error)
	ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error)
}

// HierarchyStore handles Wikidata classification hierarchy.
type HierarchyStore interface {
	GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error)
	SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error
	GetClassification(ctx context.Context, qid string) (category string, found bool, err error)
	SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error
}

// ArticleStore handles Wikipedia article persistence.
type ArticleStore interface {
	GetArticle(ctx context.Context, uuid string) (*model.Article, error)
	SaveArticle(ctx context.Context, article *model.Article) error
}

// SeenEntityStore handles negative caching for seen entities.
type SeenEntityStore interface {
	GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error)
	MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error
}

// MSFSPOIStore handles Microsoft Flight Simulator POI data.
type MSFSPOIStore interface {
	GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error)
	SaveMSFSPOI(ctx context.Context, poi *model.MSFSPOI) error
	CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error)
}

// StateStore handles persistent application state.
type StateStore interface {
	GetState(ctx context.Context, key string) (string, bool)
	SetState(ctx context.Context, key, val string) error
	DeleteState(ctx context.Context, key string) error
}
