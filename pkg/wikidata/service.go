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
	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/request"
	"phileasgo/pkg/rescue"
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
	recentTiles map[string]TileWrapper

	// Spatial Deduplication
	inflightMu    sync.Mutex
	inflightTiles map[string]bool
	mapper        *LanguageMapper

	// Configuration

	// Configuration
	// Configuration
	// Configuration
	userLang string

	// Core logic pipeline
	pipeline *Pipeline
}

// Classifier interface for dependency injection
type Classifier interface {
	Classify(ctx context.Context, qid string) (*model.ClassificationResult, error)
	ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
	GetConfig() *config.CategoriesConfig
}

// WikipediaProvider abstracts Wikipedia client for testing
type WikipediaProvider interface {
	GetArticleLengths(ctx context.Context, titles []string, lang string) (map[string]int, error)
	GetArticleContent(ctx context.Context, title, lang string) (string, error)
	GetArticleHTML(ctx context.Context, title, lang string) (string, error)
}

// NewService creates a new Wikidata Service.
func NewService(st store.Store, sim SimStateProvider, tr *tracker.Tracker, cl Classifier, rc *request.Client, geoSvc *geo.Service, poiMgr *poi.Manager, cfg config.WikidataConfig, userLang string) *Service {
	// Normalize userLang (e.g. "en-US" -> "en")
	normalizedLang := userLang
	if len(userLang) > 2 {
		normalizedLang = strings.Split(userLang, "-")[0]
	}

	client := NewClient(rc, slog.With("component", "wikidata_client"))
	wiki := wikipedia.NewClient(rc)
	sched := NewScheduler(float64(cfg.Area.MaxDist) / 1000.0) // Config is meters, Scheduler wants KM
	logger := slog.With("component", "wikidata")
	mapper := NewLanguageMapper(st, rc, slog.With("component", "mapper"))

	pipeline := NewPipeline(st, client, wiki, geoSvc, poiMgr, sched.grid, mapper, cl, cfg, logger, normalizedLang)

	svc := &Service{
		pipeline:      pipeline,
		store:         st,
		sim:           sim,
		client:        client,
		wiki:          wiki,
		geo:           geoSvc,
		poi:           poiMgr,
		scheduler:     sched,
		tracker:       tr,
		classifier:    cl,
		cfg:           cfg,
		logger:        logger,
		recentTiles:   make(map[string]TileWrapper),
		inflightTiles: make(map[string]bool),
		userLang:      normalizedLang,
		mapper:        mapper,
	}
	return svc
}

type TileWrapper struct {
	SeenAt time.Time
	Stats  rescue.TileStats
}

// Start begins the background fetch loop.
func (s *Service) Start(ctx context.Context) {
	// Use configured interval (default 5s)
	interval := time.Duration(s.cfg.FetchInterval)
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
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

// GetLanguageInfo returns primary language details for a country code (implements LanguageResolver).
func (s *Service) GetLanguageInfo(countryCode string) model.LanguageInfo {
	langs := s.mapper.GetLanguages(countryCode)
	if len(langs) > 0 {
		return langs[0]
	}
	return model.LanguageInfo{Code: "en", Name: "English"}
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

	// Use Predicted Position to "pull" the search cone forward
	lat := telemetry.PredictedLatitude
	lon := telemetry.PredictedLongitude
	// Fallback for stationary/start
	if lat == 0 && lon == 0 {
		lat = telemetry.Latitude
		lon = telemetry.Longitude
	}

	hdg := telemetry.Heading
	isAirborne := !telemetry.IsOnGround

	// Prepare recent tiles map for scheduler consumption
	s.recentMu.RLock()
	recentKeys := make(map[string]bool, len(s.recentTiles))
	for k := range s.recentTiles {
		recentKeys[k] = true
	}
	s.recentMu.RUnlock()

	// 3. Get Candidates
	candidates := s.scheduler.GetCandidates(lat, lon, hdg, telemetry.GroundSpeed, isAirborne, recentKeys)

	// 4. Process candidates (fast-forward through cache)
	processedCount := 0
	const maxProcessedPerTick = 20

	for _, c := range candidates {
		if processedCount >= maxProcessedPerTick {
			break
		}

		key := c.Tile.Key()

		// Memory Cache Check
		s.recentMu.RLock()
		wrapper, ok := s.recentTiles[key]
		s.recentMu.RUnlock()
		if ok && time.Since(wrapper.SeenAt) < 24*time.Hour {
			continue // Checked recently, skip
		}

		// Calculate neighborhood medians
		medians := s.getNeighborhoodStats(c.Tile)

		// fetchTile now takes medians
		isCacheHit := s.fetchTile(ctx, c, medians)
		processedCount++

		if !isCacheHit {
			// If we did a network request (or tried validation), stop for this tick to respect rate limits
			return
		}
		// If cache hit, continue to next candidate immediately
	}
}

func (s *Service) fetchTile(ctx context.Context, c Candidate, medians rescue.MedianStats) bool {
	// 1. In-flight check
	key := c.Tile.Key()
	s.inflightMu.Lock()
	if s.inflightTiles[key] {
		s.inflightMu.Unlock()
		s.logger.Debug("Skipping in-flight tile", "tile", key)
		return true // Treat as "fast" / no-op to avoid blocking loop, or false? True is safer to keep loop going.
	}
	s.inflightTiles[key] = true
	s.inflightMu.Unlock()

	defer func() {
		s.inflightMu.Lock()
		delete(s.inflightTiles, key)
		s.inflightMu.Unlock()
	}()

	centerLat, centerLon := s.gridCenter(c.Tile)

	cachedBody, _, ok := s.store.GetGeodataCache(ctx, key)
	if ok && len(cachedBody) > 0 {
		logging.Trace(s.logger, "Cache Hit (Optimized)", "tile", key)
		// Pass medians to pipeline
		processed, rawArticles, rescued, err := s.pipeline.ProcessTileData(ctx, cachedBody, centerLat, centerLon, false, medians)
		if err == nil {
			// Update stats even on cache hit
			s.updateTileStats(key, centerLat, centerLon, rawArticles)

			logging.Trace(s.logger, "Processed cached tile",
				"tile", key,
				"saved", len(processed),
				"rescued", rescued)
		} else {
			s.logger.Warn("Failed to process cached tile", "tile", key, "error", err)
		}
		return true // Cache Hit = Fast
	}

	// 3. Construct Query (Network Path)
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

	// 4. Execute
	articles, rawJSON, err := s.client.QuerySPARQL(ctx, query, c.Tile.Key(), radiusMeters, centerLat, centerLon)
	if err != nil {
		s.logger.Error("SPARQL Failed", "error", err)
		return false // Network attempt failed, but consumed quota/time
	}
	_ = rawJSON // rawJSON no longer needed here; caching is internal

	processed, rawArticles, rescued, err := s.pipeline.ProcessTileData(ctx, []byte(rawJSON), centerLat, centerLon, false, medians)
	if err == nil {
		s.updateTileStats(key, centerLat, centerLon, rawArticles)
		s.logger.Debug("Fetched and Saved new tile",
			"tile", c.Tile.Key(),
			"raw", len(articles),
			"saved", len(processed),
			"rescued", rescued)
	}
	return false // Network request made = Slow
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

// GetCachedTiles returns a list of cached tiles within the provided bounding box.
func (s *Service) GetCachedTiles(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]CachedTile, error) {
	records, err := s.store.GetGeodataInBounds(ctx, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return nil, fmt.Errorf("failed to list cache keys: %w", err)
	}

	var results []CachedTile
	for _, r := range records {
		radius := r.Radius
		if radius <= 0 {
			radius = 9800 // Default
		}

		results = append(results, CachedTile{Lat: r.Lat, Lon: r.Lon, Radius: radius})
	}

	return results, nil
}

// GetGlobalCoverage returns aggregated coverage data (Res 4 tiles) for the world map.
func (s *Service) GetGlobalCoverage(ctx context.Context) ([]CachedTile, error) {
	keys, err := s.store.ListGeodataCacheKeys(ctx, "wd_h3_")
	if err != nil {
		return nil, fmt.Errorf("failed to list cache keys: %w", err)
	}

	// Deduplication Map for Res 4 parents
	parents := make(map[string]bool)

	for _, k := range keys {
		if len(k) <= 6 {
			continue
		}
		index := k[6:] // Strip "wd_h3_"
		tile := HexTile{Index: index}

		// Convert to Parent Res 4 (~22km edge)
		parent := s.scheduler.grid.Parent(tile, 4)
		if parent.Index != "" {
			parents[parent.Index] = true
		}
	}

	var results []CachedTile
	for idx := range parents {
		pTile := HexTile{Index: idx}
		cLat, cLon := s.scheduler.grid.TileCenter(pTile)
		radius := s.scheduler.grid.TileRadius(pTile) * 1000 // KM to Meters for API consistency

		// Round to int for cleanliness
		r := int(math.Ceil(radius))
		results = append(results, CachedTile{
			Lat:    cLat,
			Lon:    cLon,
			Radius: r,
		})
	}

	s.logger.Info("Aggregated global coverage", "raw_tiles", len(keys), "aggregated_tiles", len(results))
	return results, nil
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

func (s *Service) getNeighborhoodStats(tile HexTile) rescue.MedianStats {
	radiusKm := 20.0 // Default
	if s.cfg.Rescue.PromoteByDimension.RadiusKM > 0 {
		radiusKm = float64(s.cfg.Rescue.PromoteByDimension.RadiusKM)
	}

	centerLat, centerLon := s.gridCenter(tile)
	var neighbors []rescue.TileStats

	s.recentMu.RLock()
	defer s.recentMu.RUnlock()

	for _, wrapper := range s.recentTiles {
		distKm := geo.Distance(geo.Point{Lat: centerLat, Lon: centerLon}, geo.Point{Lat: wrapper.Stats.Lat, Lon: wrapper.Stats.Lon}) / 1000.0
		if distKm <= radiusKm {
			neighbors = append(neighbors, wrapper.Stats)
		}
	}

	return rescue.CalculateMedian(neighbors)
}

func (s *Service) updateTileStats(key string, lat, lon float64, articles []Article) {
	// Map non-Ignored wikidata.Article to rescue.Article for processing
	var rescueArticles []rescue.Article
	for i := range articles {
		if articles[i].Ignored {
			continue
		}
		rescueArticles = append(rescueArticles, rescue.Article{
			ID:     articles[i].QID,
			Height: articles[i].Height,
			Length: articles[i].Length,
			Area:   articles[i].Area,
		})
	}

	stats := rescue.AnalyzeTile(lat, lon, rescueArticles)

	s.recentMu.Lock()
	defer s.recentMu.Unlock()

	s.recentTiles[key] = TileWrapper{
		SeenAt: time.Now(),
		Stats:  stats,
	}
}

// EnsureTilesLoaded implements poi.TileFetcher - ensures the tile at (lat, lon) is loaded.
func (s *Service) EnsureTilesLoaded(ctx context.Context, lat, lon float64) error {
	// Convert lat/lon to tile key
	tile := s.scheduler.grid.TileAt(lat, lon)
	key := tile.Key()

	// Check if already in recent cache
	s.recentMu.RLock()
	_, ok := s.recentTiles[key]
	s.recentMu.RUnlock()
	if ok {
		return nil // Already have it
	}

	// Trigger fetch via existing logic
	c := Candidate{Tile: tile}
	medians := s.getNeighborhoodStats(tile)
	s.fetchTile(ctx, c, medians)

	return nil
}

// GetPOIsNear implements poi.TileFetcher - returns POIs near (lat, lon) from the Manager's cache.
func (s *Service) GetPOIsNear(ctx context.Context, lat, lon, radiusMeters float64) ([]*model.POI, error) {
	// Delegate to POI Manager which holds the tracked POIs
	return s.poi.GetPOIsNear(lat, lon, radiusMeters), nil
}
