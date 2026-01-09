package wikidata

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/request"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/wikipedia"
)

// SimStateProvider defines what we need from the sim package.
type SimStateProvider interface {
	GetTelemetry(ctx context.Context) (sim.Telemetry, error)
	GetState() sim.State
}

// Service orchestrates the Wikidata fetching.
type Service struct {
	store      store.Store
	sim        SimStateProvider
	client     ClientInterface
	wiki       WikipediaProvider // Wikipedia Client
	geo        *geo.Service      // Geo Service
	poi        *poi.Manager      // POI Manager
	scheduler  *Scheduler
	tracker    *tracker.Tracker
	classifier Classifier
	cfg        config.WikidataConfig
	logger     *slog.Logger

	// In-memory cache to avoid spamming the DB for tiles we verified recently
	recentMu    sync.RWMutex
	recentTiles map[string]time.Time

	// Spatial Deduplication
	inflightMu    sync.Mutex
	inflightTiles map[string]bool
	mapper        *LanguageMapper

	// Configuration

	// Configuration
	userLang string
}

// Classifier interface for dependency injection
// Classifier interface for dependency injection
type Classifier interface {
	Classify(ctx context.Context, qid string) (*model.ClassificationResult, error)
	ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
}

// WikipediaProvider abstracts Wikipedia client for testing
type WikipediaProvider interface {
	GetArticleLengths(ctx context.Context, titles []string, lang string) (map[string]int, error)
	GetArticleContent(ctx context.Context, title, lang string) (string, error)
}

// DimClassifier extends Classifier with dimension capabilities
type DimClassifier interface {
	ResetDimensions()
	ObserveDimensions(h, l, a float64)
	FinalizeDimensions()
	ShouldRescue(h, l, a float64, instances []string) bool
	GetMultiplier(h, l, a float64) float64
	GetConfig() *config.CategoriesConfig
}

// NewService creates a new Wikidata Service.
func NewService(st store.Store, sim SimStateProvider, tr *tracker.Tracker, cl Classifier, rc *request.Client, geoSvc *geo.Service, poiMgr *poi.Manager, cfg config.WikidataConfig, userLang string) *Service {
	// Normalize userLang (e.g. "en-US" -> "en")
	normalizedLang := userLang
	if len(userLang) > 2 {
		normalizedLang = strings.Split(userLang, "-")[0]
	}

	return &Service{
		store:         st,
		sim:           sim,
		client:        NewClient(rc, slog.With("component", "wikidata_client")),
		wiki:          wikipedia.NewClient(rc),
		geo:           geoSvc,
		poi:           poiMgr,
		scheduler:     NewScheduler(cfg.Area.MaxDist),
		tracker:       tr,
		classifier:    cl,
		cfg:           cfg,
		logger:        slog.With("component", "wikidata"),
		recentTiles:   make(map[string]time.Time),
		inflightTiles: make(map[string]bool),
		userLang:      normalizedLang,
		mapper:        NewLanguageMapper(st, rc, slog.With("component", "mapper")),
	}
}

// Start begins the background fetch loop.
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	s.logger.Info("Wikidata Service Started")

	// Start Language Mapper with dedicated timeout (detached from main startup deadline if any)
	// This ensures we have enough time to fetch the map even if startup was tight.
	// We run this synchronously before the loop to ensure the mapper is ready.
	initCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := s.mapper.Start(initCtx); err != nil {
		s.logger.Warn("LanguageMapper failed to start (continuing with defaults)", "error", err)
	}
	cancel()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Wikidata Service Stopped")
			return
		case <-ticker.C:
			s.processTick(ctx)
		}
	}
}

// WikipediaClient returns the internal Wikipedia client.
func (s *Service) WikipediaClient() WikipediaProvider {
	return s.wiki
}

// GeoService returns the internal Geo service.
func (s *Service) GeoService() *geo.Service {
	return s.geo
}

// GetLanguageInfo returns language details for a country code (implements LanguageResolver).
func (s *Service) GetLanguageInfo(countryCode string) model.LanguageInfo {
	return s.mapper.GetLanguage(countryCode)
}

func (s *Service) processTick(ctx context.Context) {
	// 1. Check Sim State - only proceed if actively flying
	if s.sim.GetState() != sim.StateActive {
		return
	}

	// 2. Get Telemetry
	telemetry, err := s.sim.GetTelemetry(ctx)
	if err != nil {
		// Reduce log noise if not connected
		return
	}

	lat := telemetry.Latitude
	lon := telemetry.Longitude
	hdg := telemetry.Heading
	isAirborne := !telemetry.IsOnGround

	// 3. Get Candidates
	candidates := s.scheduler.GetCandidates(lat, lon, hdg, isAirborne)

	// 4. Find first uncached candidate
	for _, c := range candidates {
		key := c.Tile.Key()

		// Memory Cache Check
		s.recentMu.RLock()
		_, ok := s.recentTiles[key]
		s.recentMu.RUnlock()
		if ok {
			continue // Checked recently, skip
		}

		s.recentMu.Lock()
		s.recentTiles[key] = time.Now() // Mark before fetch to prevent immediate retry logic overlap
		s.recentMu.Unlock()

		s.logger.Debug("Checking tile", "key", key, "dist_km", fmt.Sprintf("%.1f", c.Dist), "airborne", isAirborne, "hdg", int(hdg))
		s.fetchTile(ctx, c)
		return // One fetch per tick
	}

}

func (s *Service) fetchTile(ctx context.Context, c Candidate) {
	// 1. In-flight check
	key := c.Tile.Key()
	s.inflightMu.Lock()
	if s.inflightTiles[key] {
		s.inflightMu.Unlock()
		s.logger.Debug("Skipping in-flight tile", "key", key)
		return
	}
	s.inflightTiles[key] = true
	s.inflightMu.Unlock()

	defer func() {
		s.inflightMu.Lock()
		delete(s.inflightTiles, key)
		s.inflightMu.Unlock()
	}()

	// 2. Construct Query
	centerLat, centerLon := s.gridCenter(c.Tile)

	// Calculate precise radius in meters for this specific tile geometry
	// Round up to the next 10m (User Request), remove fixed 50m buffer
	rawRadius := s.scheduler.grid.TileRadius(c.Tile) * 1000
	radiusMeters := int(math.Ceil(rawRadius/10.0) * 10)

	// STRICT CAP: 10km (Wikidata API Limit)
	if radiusMeters > 10000 {
		radiusMeters = 10000
	}

	// Create formatted string for SPARQL (e.g. "9.810") - query expects KM
	radiusStr := fmt.Sprintf("%.3f", float64(radiusMeters)/1000.0)

	query := buildCheapQuery(centerLat, centerLon, radiusStr)

	// 2. Execute (Caching now handled internally by QuerySPARQL via PostWithGeodataCache)
	articles, rawJSON, err := s.client.QuerySPARQL(ctx, query, c.Tile.Key(), radiusMeters)
	if err != nil {
		s.logger.Error("SPARQL Failed", "error", err)
		return
	}
	_ = rawJSON // rawJSON no longer needed here; caching is internal

	// 4. Process, Enrich, and Save
	processed, rescued, err := s.ProcessTileData(ctx, []byte(rawJSON), centerLat, centerLon, false)
	if err == nil {
		s.logger.Debug("Fetched and Saved new tile",
			"key", c.Tile.Key(),
			"raw", len(articles),
			"saved", len(processed),
			"rescued", rescued)
	}
}

func (s *Service) gridCenter(t HexTile) (lat, lon float64) {
	// Expose grid center via scheduler -> grid
	return s.scheduler.grid.TileCenter(t)
}

// CachedTile represents a visualization circle on the map.
type CachedTile struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Radius int     `json:"radius"`
}

// GetCachedTiles returns a list of cached tiles with their centers and queried radii.
func (s *Service) GetCachedTiles(ctx context.Context, lat, lon, radiusKm float64) ([]CachedTile, error) {
	keys, err := s.store.ListGeodataCacheKeys(ctx, "wd_h3_")
	if err != nil {
		return nil, fmt.Errorf("failed to list cache keys: %w", err)
	}

	var results []CachedTile

	for _, k := range keys {
		// Key format: wd_h3_{index}
		if len(k) <= 6 {
			continue
		}
		index := k[6:]

		tile := HexTile{Index: index}
		// Uses named return: lat, lon
		cLat, cLon := s.gridCenter(tile)

		// Optimization: Check distance before fetching metadata
		dist := DistKm(lat, lon, cLat, cLon)
		if dist > radiusKm {
			continue
		}

		// Fetch metadata (radius)
		_, r, ok := s.store.GetGeodataCache(ctx, k)
		if !ok || r <= 0 {
			r = 9800 // Default 9.8km if missing in metadata
		}

		results = append(results, CachedTile{Lat: cLat, Lon: cLon, Radius: r})
	}

	return results, nil
}

// ReprocessNearTiles forces a re-ingestion of cached tiles near the given location.
func (s *Service) ReprocessNearTiles(ctx context.Context, lat, lon, radiusKm float64) error {
	keys, err := s.store.ListCacheKeys(ctx, "wd_h3_")
	if err != nil {
		s.logger.Error("Failed to list cache keys for reprocessing", "error", err)
		return err
	}

	s.logger.Info("ReprocessNearTiles triggered", "lat", lat, "lon", lon, "radius", radiusKm)

	// Iterate all keys to find matches
	tilesChecked := 0
	rescuedCount := 0
	totalArticles := 0

	for _, k := range keys {
		// Key format: wd_h3_{index}
		if len(k) <= 6 {
			continue
		}
		index := k[6:]
		t := HexTile{Index: index}
		cLat, cLon := s.gridCenter(t)

		if DistKm(lat, lon, cLat, cLon) <= radiusKm {
			tilesChecked++
			// Get Raw Data from Cache
			raw, _, ok := s.store.GetGeodataCache(ctx, k)
			if !ok {
				continue
			}

			// Reprocess this tile!
			// Force reprocessing (bypass seen filter)
			reprocessed, rescued, err := s.ProcessTileData(ctx, raw, cLat, cLon, true)
			if err != nil {
				s.logger.Warn("Failed to reprocess tile", "key", k, "error", err)
			} else {
				totalArticles += len(reprocessed)
				rescuedCount += rescued
				if rescued > 0 {
					s.logger.Debug("Reprocessed tile and rescued entities", "key", k, "rescued", rescued)
				}
			}
		}
	}
	s.logger.Info("ReprocessNearTiles complete", "component", "wikidata", "tiles_checked", tilesChecked, "dynamic_pois_added", rescuedCount, "total_articles", totalArticles)
	return nil
}

// EvictFarTiles removes tiles from the recent cache if they are beyond the max distance.
func (s *Service) EvictFarTiles(lat, lon, thresholdKm float64) int {
	// 1. Snapshot keys to avoid deep locking issues if logic is complex
	// But simple distance check is fast.
	s.recentMu.Lock()
	defer s.recentMu.Unlock()

	count := 0
	for key, t := range s.recentTiles {
		if !strings.HasPrefix(key, "wd_h3_") {
			continue
		}
		index := strings.TrimPrefix(key, "wd_h3_")

		// Use scheduler grid to get center
		cLat, cLon := s.scheduler.grid.TileCenter(HexTile{Index: index})

		// Check Distance (geo.Distance returns meters)
		distKm := geo.Distance(geo.Point{Lat: lat, Lon: lon}, geo.Point{Lat: cLat, Lon: cLon}) / 1000.0

		if distKm > thresholdKm {
			delete(s.recentTiles, key)
			count++
		}
		// Also clean up old entries by time?
		// For now just distance as requested.
		_ = t // unused
	}

	if count > 0 {
		s.logger.Debug("Evicted far tiles from memory", "count", count, "threshold_km", thresholdKm)
	}
	return count
}
