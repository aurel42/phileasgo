package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"phileasgo/pkg/db"
	"phileasgo/pkg/model"
)

// Store defines the repository interface.
// It composes all sub-interfaces for full store access.
// Consumers should depend on specific sub-interfaces when possible.
type Store interface {
	POIStore
	CacheStore
	GeodataStore
	HierarchyStore
	ArticleStore
	SeenEntityStore
	MSFSPOIStore
	StateStore

	// Close closes the store connection.
	Close() error
}

// SQLiteStore implements Store.
type SQLiteStore struct {
	db *db.DB // Changed from *data.DB
}

// NewSQLiteStore creates a new store.
func NewSQLiteStore(db *db.DB) *SQLiteStore { // Changed from *data.DB
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- POI ---

// --- POI ---

func (s *SQLiteStore) GetPOI(ctx context.Context, wikidataID string) (*model.POI, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT wikidata_id, source, category, specific_category, lat, lon, sitelinks, name_en, name_local, name_user, wp_url, wp_article_length, trigger_qid, last_played, created_at, is_msfs_poi
		 FROM poi WHERE wikidata_id = ?`, wikidataID)

	var p model.POI
	var lastPlayed sql.NullTime
	var specificCategory sql.NullString

	err := row.Scan(
		&p.WikidataID, &p.Source, &p.Category, &specificCategory,
		&p.Lat, &p.Lon, &p.Sitelinks,
		&p.NameEn, &p.NameLocal, &p.NameUser,
		&p.WPURL, &p.WPArticleLength,
		&p.TriggerQID, &lastPlayed, &p.CreatedAt, &p.IsMSFSPOI,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		// Fallback for old schema if column missing (useful during dev)
		if err.Error() == "no such column: is_msfs_poi" {
			// Retry without the column
			return s.getPOILegacy(ctx, wikidataID)
		}
		return nil, err
	}

	if lastPlayed.Valid {
		p.LastPlayed = lastPlayed.Time
	}
	if specificCategory.Valid {
		p.SpecificCategory = specificCategory.String
	}

	return &p, nil
}

// Fallback for getting POI without new columns
func (s *SQLiteStore) getPOILegacy(ctx context.Context, wikidataID string) (*model.POI, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT wikidata_id, source, category, specific_category, lat, lon, sitelinks, name_en, name_local, name_user, wp_url, wp_article_length, trigger_qid, last_played, created_at
		 FROM poi WHERE wikidata_id = ?`, wikidataID)

	var p model.POI
	var lastPlayed sql.NullTime
	var specificCategory sql.NullString

	err := row.Scan(
		&p.WikidataID, &p.Source, &p.Category, &specificCategory,
		&p.Lat, &p.Lon, &p.Sitelinks,
		&p.NameEn, &p.NameLocal, &p.NameUser,
		&p.WPURL, &p.WPArticleLength,
		&p.TriggerQID, &lastPlayed, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if lastPlayed.Valid {
		p.LastPlayed = lastPlayed.Time
	}
	if specificCategory.Valid {
		p.SpecificCategory = specificCategory.String
	}
	return &p, nil
}

func (s *SQLiteStore) GetPOIsBatch(ctx context.Context, wikidataIDs []string) (map[string]*model.POI, error) {
	if len(wikidataIDs) == 0 {
		return make(map[string]*model.POI), nil
	}

	query := `SELECT wikidata_id, source, category, specific_category, lat, lon, sitelinks, name_en, name_local, name_user, wp_url, wp_article_length, trigger_qid, last_played, created_at, is_msfs_poi
			  FROM poi WHERE wikidata_id IN (`
	args := make([]any, len(wikidataIDs))
	for i, id := range wikidataIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Fallback for batch? Tricky. Easier to assume schema is migrated or fail.
		// To avoid breaking, can check error like above, but batch logic is complex to duplicated.
		// Assuming migration script runs or AutoMigrate works.
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]*model.POI)
	for rows.Next() {
		var p model.POI
		var lastPlayed sql.NullTime
		var specificCategory sql.NullString
		err := rows.Scan(
			&p.WikidataID, &p.Source, &p.Category, &specificCategory,
			&p.Lat, &p.Lon, &p.Sitelinks,
			&p.NameEn, &p.NameLocal, &p.NameUser,
			&p.WPURL, &p.WPArticleLength,
			&p.TriggerQID, &lastPlayed, &p.CreatedAt, &p.IsMSFSPOI,
		)
		if err != nil {
			return nil, err
		}
		if lastPlayed.Valid {
			p.LastPlayed = lastPlayed.Time
		}
		if specificCategory.Valid {
			p.SpecificCategory = specificCategory.String
		}
		results[p.WikidataID] = &p
	}
	return results, nil
}

func (s *SQLiteStore) SavePOI(ctx context.Context, p *model.POI) error {
	query := `INSERT OR REPLACE INTO poi (
		wikidata_id, source, category, specific_category, lat, lon, sitelinks, 
		name_en, name_local, name_user, wp_url, wp_article_length,
		trigger_qid, last_played, created_at, is_msfs_poi
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	createdAt := p.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, query,
		p.WikidataID, p.Source, p.Category, p.SpecificCategory, p.Lat, p.Lon, p.Sitelinks,
		p.NameEn, p.NameLocal, p.NameUser, p.WPURL, p.WPArticleLength,
		p.TriggerQID, p.LastPlayed, createdAt, p.IsMSFSPOI,
	)
	return err
}

func (s *SQLiteStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	query := `SELECT wikidata_id, source, category, specific_category, lat, lon, sitelinks, name_en, name_local, name_user, wp_url, wp_article_length, trigger_qid, last_played, created_at, is_msfs_poi
			  FROM poi WHERE last_played > ? ORDER BY last_played DESC LIMIT 10`

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*model.POI
	for rows.Next() {
		var p model.POI
		var lastPlayed sql.NullTime
		var specificCategory sql.NullString
		err := rows.Scan(
			&p.WikidataID, &p.Source, &p.Category, &specificCategory,
			&p.Lat, &p.Lon, &p.Sitelinks,
			&p.NameEn, &p.NameLocal, &p.NameUser,
			&p.WPURL, &p.WPArticleLength,
			&p.TriggerQID, &lastPlayed, &p.CreatedAt, &p.IsMSFSPOI,
		)
		if err != nil {
			return nil, err
		}
		if lastPlayed.Valid {
			p.LastPlayed = lastPlayed.Time
		}
		if specificCategory.Valid {
			p.SpecificCategory = specificCategory.String
		}
		results = append(results, &p)
	}
	return results, nil
}

func (s *SQLiteStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error {
	// Crude approx: 1 deg lat ~= 111km.
	// radius is in meters.
	degRadius := (radius / 1000.0) / 111.0
	// Slightly conservative box
	minLat, maxLat := lat-degRadius, lat+degRadius
	minLon, maxLon := lon-degRadius, lon+degRadius

	query := `UPDATE poi SET last_played = NULL 
			  WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ?`

	// Note: We don't do strict Great Circle check here for efficiency,
	// relying on the box approximation which is fine for "nearby" reset.
	// If needed, we can select IDs first with Go-based distance check and then update.

	_, err := s.db.ExecContext(ctx, query, minLat, maxLat, minLon, maxLon)
	return err
}

// --- MSFS ---

func (s *SQLiteStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, type, name, ident, lat, lon, elevation FROM msfs_poi WHERE id = ?`, id)

	var p model.MSFSPOI
	err := row.Scan(&p.ID, &p.Type, &p.Name, &p.Ident, &p.Lat, &p.Lon, &p.Elevation)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *SQLiteStore) SaveMSFSPOI(ctx context.Context, p *model.MSFSPOI) error {
	query := `INSERT OR REPLACE INTO msfs_poi (type, name, ident, lat, lon, elevation) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, p.Type, p.Name, p.Ident, p.Lat, p.Lon, p.Elevation)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		p.ID = id
	}
	return err
}

func (s *SQLiteStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	// Crude approx: 1 deg lat ~= 111km.
	// radius is in meters.
	degRadius := (radius / 1000.0) / 111.0
	// Slightly conservative box
	minLat, maxLat := lat-degRadius, lat+degRadius
	minLon, maxLon := lon-degRadius, lon+degRadius

	query := `SELECT lat, lon FROM msfs_poi WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ?`

	rows, err := s.db.QueryContext(ctx, query, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	const R = 6371000.0 // Earth radius in meters

	for rows.Next() {
		var pLat, pLon float64
		if err := rows.Scan(&pLat, &pLon); err != nil {
			return false, err
		}

		// Haversine Distance
		dLat := (pLat - lat) * (math.Pi / 180.0)
		dLon := (pLon - lon) * (math.Pi / 180.0)
		lat1 := lat * (math.Pi / 180.0)
		lat2 := pLat * (math.Pi / 180.0)

		a := math.Sin(dLat/2)*math.Sin(dLat/2) +
			math.Cos(lat1)*math.Cos(lat2)*
				math.Sin(dLon/2)*math.Sin(dLon/2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
		dist := R * c

		if dist <= radius {
			return true, nil
		}
	}
	return false, nil
}

// --- Hierarchy ---

func (s *SQLiteStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	// Added category and updated_at to the query
	row := s.db.QueryRowContext(ctx,
		`SELECT qid, name, parents, category, created_at FROM wikidata_hierarchy WHERE qid = ?`, qid)

	var h model.WikidataHierarchy
	var parentsJSON string
	// Scan category
	err := row.Scan(&h.QID, &h.Name, &parentsJSON, &h.Category, &h.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if parentsJSON != "" {
		_ = json.Unmarshal([]byte(parentsJSON), &h.Parents)
	}
	return &h, nil
}

func (s *SQLiteStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error {
	parentsJSON, _ := json.Marshal(h.Parents)
	createdAt := h.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// Upsert with category and updated_at
	query := `INSERT OR REPLACE INTO wikidata_hierarchy (qid, name, parents, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, h.QID, h.Name, string(parentsJSON), h.Category, createdAt, time.Now())
	return err
}

func (s *SQLiteStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT category FROM wikidata_hierarchy WHERE qid = ?", qid).Scan(&category)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil // Not found in DB
	}
	if err != nil {
		return "", false, err
	}
	return category, true, nil // Found (even if empty)
}

func (s *SQLiteStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	parentsJSON, _ := json.Marshal(parents)

	slog.Debug("Store: Saving Classification", "qid", qid, "cat", category)

	query := `INSERT INTO wikidata_hierarchy (qid, name, parents, category, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?)
			  ON CONFLICT(qid) DO UPDATE SET
			  name=excluded.name,
			  parents=excluded.parents,
			  category=excluded.category,
			  updated_at=excluded.updated_at`

	_, err := s.db.ExecContext(ctx, query, qid, label, string(parentsJSON), category, time.Now(), time.Now())
	if err != nil {
		slog.Error("Store: SaveClassification Failed", "qid", qid, "error", err)
	}
	return err
}

// --- Articles ---

func (s *SQLiteStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT uuid, title, url, names, text, lengths, thumbnail_url, created_at FROM wikipedia_articles WHERE uuid = ?`, uuid)

	var a model.Article
	var namesJSON, lengthsJSON string
	err := row.Scan(&a.UUID, &a.Title, &a.URL, &namesJSON, &a.Text, &lengthsJSON, &a.ThumbnailURL, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if namesJSON != "" {
		_ = json.Unmarshal([]byte(namesJSON), &a.Names)
	}
	if lengthsJSON != "" {
		_ = json.Unmarshal([]byte(lengthsJSON), &a.Lengths)
	}
	return &a, nil
}

func (s *SQLiteStore) SaveArticle(ctx context.Context, a *model.Article) error {
	namesJSON, _ := json.Marshal(a.Names)
	lengthsJSON, _ := json.Marshal(a.Lengths)
	createdAt := a.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	query := `INSERT OR REPLACE INTO wikipedia_articles (
		uuid, title, url, names, text, lengths, thumbnail_url, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		a.UUID, a.Title, a.URL, string(namesJSON), a.Text, string(lengthsJSON), a.ThumbnailURL, createdAt,
	)
	return err
}

// --- Seen Entities ---

func (s *SQLiteStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	if len(qids) == 0 {
		return make(map[string][]string), nil
	}

	query := `SELECT qid, instances FROM seen_entities WHERE qid IN (`
	args := make([]interface{}, len(qids))
	for i, id := range qids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string][]string)
	for rows.Next() {
		var qid string
		var instancesJSON sql.NullString
		if err := rows.Scan(&qid, &instancesJSON); err != nil {
			return nil, err
		}

		var instances []string
		if instancesJSON.Valid && instancesJSON.String != "" {
			_ = json.Unmarshal([]byte(instancesJSON.String), &instances)
		}
		seen[qid] = instances
	}
	return seen, nil
}

func (s *SQLiteStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	if len(entities) == 0 {
		return nil
	}

	query := `INSERT OR IGNORE INTO seen_entities (qid, instances, created_at) VALUES (?, ?, ?)`
	now := time.Now()

	for qid, instances := range entities {
		var instancesJSON string
		if len(instances) > 0 {
			b, _ := json.Marshal(instances)
			instancesJSON = string(b)
		}
		if _, err := s.db.ExecContext(ctx, query, qid, instancesJSON, now); err != nil {
			return err
		}
	}
	return nil
}

// --- Cache ---

// Get implements cache.Cacher interface.
func (s *SQLiteStore) Get(key string) ([]byte, bool) {
	return s.GetCache(context.Background(), key)
}

// Set implements cache.Cacher interface.
func (s *SQLiteStore) Set(key string, val []byte) error {
	return s.SetCache(context.Background(), key, val)
}

func (s *SQLiteStore) GetCache(ctx context.Context, key string) ([]byte, bool) {
	var val []byte
	err := s.db.QueryRowContext(ctx, "SELECT value FROM cache WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		// Log error? For cache, we might treat error as miss or return false
		return nil, false
	}

	// Transparent Decompression
	if len(val) > 2 && val[0] == 0x1f && val[1] == 0x8b {
		decompressed, err := decompress(val)
		if err == nil {
			return decompressed, true
		}
		// If decompression fails, maybe it's not actually gzipped or corrupted
		// For now return raw, or false? Let's return raw as fallback.
	}

	return val, true
}

// --- Compression Pooling ---

var (
	// Pool for gzip writers to reuse flate state
	gzipWriterPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}
	// Pool for generic byte buffers
	bufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

func compress(data []byte) ([]byte, error) {
	// Get Buffer
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Get Writer
	w := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(w)

	// Reset Writer to write to our buffer
	w.Reset(buf)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	// Must copy because buf is returned to pool
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

func decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (s *SQLiteStore) HasCache(ctx context.Context, key string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM cache WHERE key = ?", key).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLiteStore) SetCache(ctx context.Context, key string, val []byte) error {
	// Transparent Compression
	compressed, err := compress(val)
	if err == nil {
		val = compressed
	}

	query := `INSERT OR REPLACE INTO cache (key, value, created_at) VALUES (?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query, key, val, time.Now())
	return err
}

// --- Geodata Cache ---

func (s *SQLiteStore) GetGeodataCache(ctx context.Context, key string) (data []byte, radius int, found bool) {
	err := s.db.QueryRowContext(ctx, "SELECT data, radius_m FROM cache_geodata WHERE key = ?", key).Scan(&data, &radius)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, false
	}
	if err != nil {
		return nil, 0, false
	}

	// Transparent Decompression
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		decompressed, err := decompress(data)
		if err == nil {
			return decompressed, radius, true
		}
	}

	return data, radius, true
}

func (s *SQLiteStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	// Transparent Compression
	compressed, err := compress(val)
	if err == nil {
		val = compressed
	}

	query := `INSERT OR REPLACE INTO cache_geodata (key, data, radius_m, lat, lon, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query, key, val, radius, lat, lon, time.Now())
	return err
}

func (s *SQLiteStore) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]GeodataRecord, error) {
	query := `SELECT key, lat, lon, radius_m FROM cache_geodata 
	          WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ?`

	rows, err := s.db.QueryContext(ctx, query, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GeodataRecord
	for rows.Next() {
		var r GeodataRecord
		if err := rows.Scan(&r.Key, &r.Lat, &r.Lon, &r.Radius); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *SQLiteStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key FROM cache WHERE key LIKE ?", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListGeodataCacheKeys lists all keys with the given prefix from cache_geodata table.
func (s *SQLiteStore) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key FROM cache_geodata WHERE key LIKE ?", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// --- State ---

func (s *SQLiteStore) GetState(ctx context.Context, key string) (string, bool) {
	var val string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM persistent_state WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false
	}
	return val, true
}

func (s *SQLiteStore) SetState(ctx context.Context, key, val string) error {
	query := `INSERT OR REPLACE INTO persistent_state (key, value, created_at) VALUES (?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, key, val, time.Now())
	return err
}
func (s *SQLiteStore) DeleteState(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM persistent_state WHERE key = ?", key)
	return err
}
