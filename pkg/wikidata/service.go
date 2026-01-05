package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
}

// Service orchestrates the Wikidata fetching.
type Service struct {
	store      store.Store
	sim        SimStateProvider
	client     *Client
	wiki       *wikipedia.Client // Wikipedia Client
	geo        *geo.Service      // Geo Service
	poi        *poi.Manager      // POI Manager
	scheduler  *Scheduler
	tracker    *tracker.Tracker
	classifier Classifier
	cfg        config.WikidataConfig
	logger     *slog.Logger

	// In-memory cache to avoid spamming the DB for tiles we verified recently
	recentTiles map[string]time.Time

	// Spatial Deduplication
	inflightMu    sync.Mutex
	inflightTiles map[string]bool

	// Configuration
	userLang string
}

// Classifier interface for dependency injection
// Classifier interface for dependency injection
type Classifier interface {
	Classify(ctx context.Context, qid string) (*model.ClassificationResult, error)
	ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
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
	}
}

// Start begins the background fetch loop.
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	s.logger.Info("Wikidata Service Started")

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
func (s *Service) WikipediaClient() *wikipedia.Client {
	return s.wiki
}

// GeoService returns the internal Geo service.
func (s *Service) GeoService() *geo.Service {
	return s.geo
}

func (s *Service) processTick(ctx context.Context) {
	// 1. Get Sim State
	telemetry, err := s.sim.GetTelemetry(ctx)

	if err != nil {
		// Reduce log noise if not connected
		return
	}

	lat := telemetry.Latitude
	lon := telemetry.Longitude
	hdg := telemetry.Heading
	isAirborne := !telemetry.IsOnGround

	// 2. Get Candidates
	candidates := s.scheduler.GetCandidates(lat, lon, hdg, isAirborne)

	// 3. Find first uncached candidate
	for _, c := range candidates {
		key := c.Tile.Key()

		// Memory Cache Check
		if _, ok := s.recentTiles[key]; ok {
			continue // Checked recently, skip
		}

		s.recentTiles[key] = time.Now() // Mark before fetch to prevent immediate retry logic overlap

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

	// Dynamic Language
	country := s.geo.GetCountry(centerLat, centerLon)
	localLang := s.geo.GetLanguage(country)

	query := buildQuery(centerLat, centerLon, localLang, s.userLang, s.cfg.Area.MaxArticles)

	// 2. Execute (Requester handles caching and core tracking)
	articles, rawJSON, err := s.client.QuerySPARQL(ctx, query, c.Tile.Key())
	if err != nil {
		s.logger.Error("SPARQL Failed", "error", err)
		return
	}

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

func buildQuery(lat, lon float64, localLang, userLang string, maxArticles int) string {
	// Radius 10km fixed
	radius := "10"
	if localLang == "" {
		localLang = "en"
	}
	if userLang == "" {
		userLang = "en"
	}
	if maxArticles <= 0 {
		maxArticles = 100 // Default fallback
	}

	query := fmt.Sprintf(`SELECT DISTINCT ?item ?lat ?lon ?sitelinks 
            (GROUP_CONCAT(DISTINCT ?instance_of_uri; separator=",") AS ?instances) 
            ?title_local_val ?title_en_val ?title_user_val
            ?area ?height ?length ?width
        WHERE { 
            SERVICE wikibase:around { 
                ?item wdt:P625 ?location . 
                bd:serviceParam wikibase:center "Point(%f %f)"^^geo:wktLiteral . 
                bd:serviceParam wikibase:radius "%s" . 
            } 
            ?item p:P625/psv:P625 [ wikibase:geoLatitude ?lat ; wikibase:geoLongitude ?lon ] . 
            OPTIONAL { ?item wdt:P31 ?instance_of_uri . } 
            OPTIONAL { ?item wikibase:sitelinks ?sitelinks . } 
            OPTIONAL { ?item wdt:P2046 ?area . }
            OPTIONAL { ?item wdt:P2048 ?height . }
            OPTIONAL { ?item wdt:P2043 ?length . }
            OPTIONAL { ?item wdt:P2049 ?width . }
            FILTER EXISTS { 
                VALUES ?allowed_lang { "%s" "%s" "en" }
                ?article_check schema:about ?item ; 
                schema:inLanguage ?allowed_lang .
            } 
            OPTIONAL { 
                ?evt_local schema:about ?item ; 
                schema:inLanguage "%s" ; 
                schema:isPartOf <https://%s.wikipedia.org/> ; 
                schema:name ?title_local_val . 
            } 
            OPTIONAL { 
                ?evt_en schema:about ?item ; 
                schema:inLanguage "en" ; 
                schema:isPartOf <https://en.wikipedia.org/> ; 
                schema:name ?title_en_val . 
            } 
            OPTIONAL { 
                ?evt_user schema:about ?item ; 
                schema:inLanguage "%s" ; 
                schema:isPartOf <https://%s.wikipedia.org/> ; 
                schema:name ?title_user_val . 
            } 
        } 
        GROUP BY ?item ?lat ?lon ?sitelinks ?title_local_val ?title_en_val ?title_user_val ?area ?height ?length ?width
        ORDER BY DESC(?sitelinks) 
        LIMIT %d`, lon, lat, radius, localLang, userLang, localLang, localLang, userLang, userLang, maxArticles)

	return query
}

// GetArticlesForTile retrieves raw JSON from cache, re-parses and re-classifies it.
func (s *Service) GetArticlesForTile(ctx context.Context, tileKey string) ([]Article, error) {
	raw, ok := s.store.GetCache(ctx, tileKey)
	if !ok {
		return nil, fmt.Errorf("tile not found in cache: %s", tileKey)
	}
	// Actually GetArticlesForTile is likely used for debugging.
	articles, _, err := s.ProcessTileData(ctx, raw, 0, 0, false)
	return articles, err
}

// ReprocessNearTiles forces a re-ingestion of cached tiles near the given location.
// This is used when dynamic interests update (e.g. "Steel Mill" becomes interesting),
// effectively attempting to "rescue" entities that were previously classified as boring.
func (s *Service) ReprocessNearTiles(ctx context.Context, lat, lon, radiusKm float64) error {
	s.logger.Info("ReprocessNearTiles triggered", "lat", lat, "lon", lon, "radius", radiusKm)

	// List all cached tiles.
	// Potential Optimization: If cache is huge, this list is slow.
	// Better to have a spatial index of cached tiles, but for now this works.
	keys, err := s.store.ListCacheKeys(ctx, "wd_hex_")
	if err != nil {
		return fmt.Errorf("failed to list cache keys: %w", err)
	}

	// 1. Force Process
	tilesChecked := 0
	rescuedCount := 0
	totalArticles := 0
	for _, k := range keys {
		// Key format: wd_hex_{row}_{col}
		if len(k) <= 7 {
			continue
		}
		parts := strings.Split(k[7:], "_")
		if len(parts) != 2 {
			continue
		}

		var row, col int
		if _, err := fmt.Sscanf(parts[0], "%d", &row); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &col); err != nil {
			continue
		}

		tile := HexTile{Row: row, Col: col}
		cLat, cLon := s.gridCenter(tile)

		dist := DistKm(lat, lon, cLat, cLon)
		if dist <= radiusKm {
			tilesChecked++
			// Reprocess this tile!
			raw, ok := s.store.GetCache(ctx, k)
			if !ok {
				continue
			}

			// Force reprocessing (bypass seen filter)
			reprocessed, rescued, err := s.ProcessTileData(ctx, raw, cLat, cLon, true)
			if err != nil {
				s.logger.Warn("Failed to reprocess tile", "key", k, "error", err)
			} else {
				totalArticles += len(reprocessed)
				if rescued > 0 {
					rescuedCount += rescued
					s.logger.Debug("Reprocessed tile and rescued entities", "key", k, "rescued", rescued)
				}
			}
		}
	}
	s.logger.Info("ReprocessNearTiles complete", "component", "wikidata", "tiles_checked", tilesChecked, "dynamic_pois_added", rescuedCount, "total_articles", totalArticles)
	return nil
}

// ProcessTileData takes raw SPARQL JSON, parses it, runs classification, ENRICHES, and SAVES to DB.
func (s *Service) ProcessTileData(ctx context.Context, rawJSON []byte, centerLat, centerLon float64, force bool) (articles []Article, rescuedCount int, err error) {
	var result sparqlResponse
	if err := json.Unmarshal(rawJSON, &result); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal raw json: %w", err)
	}

	rawArticles := parseBindings(result)
	qids := make([]string, len(rawArticles))
	for i := range rawArticles {
		qids[i] = rawArticles[i].QID
	}

	// 1. Filter out already existing POIs (Drop them immediately)
	rawArticles = s.filterExistingPOIs(ctx, rawArticles, qids)

	// 2. Filter seen articles (drop them immediately), UNLESS forced
	if !force {
		rawArticles = s.filterSeenArticles(ctx, rawArticles)
	}

	// 3. Batch Classification for new articles (also filters out ignored)
	rawArticles = s.classifyAndFilterArticles(ctx, rawArticles)

	// 4. Post-processing (Rescue & Filters)
	processed, rescued, err := s.postProcessArticles(rawArticles)
	if err != nil {
		return nil, 0, err
	}

	// 5. Enrichment & Saving
	country := s.geo.GetCountry(centerLat, centerLon)
	localLang := s.geo.GetLanguage(country)

	if len(processed) > 0 {
		err = s.enrichAndSave(ctx, processed, localLang, "en")
	}

	// 6. Mark remaining unprocessed articles as seen (those that failed filters like sitelinks)
	processedQIDs := make(map[string]bool)
	for i := range processed {
		processedQIDs[processed[i].QID] = true
	}

	toMark := make(map[string][]string)
	for i := range rawArticles {
		qid := rawArticles[i].QID
		if !processedQIDs[qid] {
			toMark[qid] = rawArticles[i].Instances
		}
	}
	if len(toMark) > 0 {
		if errMark := s.store.MarkEntitiesSeen(ctx, toMark); errMark != nil {
			s.logger.Warn("Failed to mark entities as seen", "error", errMark)
		}
	}

	return processed, rescued, err
}

func (s *Service) enrichAndSave(ctx context.Context, articles []Article, localLang, userLang string) error {
	// 4a. Collect titles for batch fetch of lengths
	// We need lengths for: Local, En, User (if different)
	// Map: Lang -> []Titles
	titlesByLang := make(map[string][]string)

	for i := range articles {
		a := &articles[i]
		// Local
		if a.Title != "" {
			titlesByLang[localLang] = append(titlesByLang[localLang], a.Title)
		}
		// En
		if a.TitleEn != "" {
			titlesByLang["en"] = append(titlesByLang["en"], a.TitleEn)
		}
		// User
		if a.TitleUser != "" && userLang != "en" && userLang != localLang {
			titlesByLang[userLang] = append(titlesByLang[userLang], a.TitleUser)
		}
	}

	// 4b. Fetch Lengths
	lengths := make(map[string]map[string]int) // Lang -> Title -> Length
	for lang, titles := range titlesByLang {
		if len(titles) == 0 {
			continue
		}
		res, err := s.wiki.GetArticleLengths(ctx, titles, lang)
		if err != nil {
			s.logger.Warn("Failed to fetch article lengths", "lang", lang, "error", err)
			continue
		}
		lengths[lang] = res
	}

	// 4c. Construct POIs and Save
	// 4c. Construct POIs and Save
	// 4c. Construct POIs
	var candidates []*model.POI

	for i := range articles {
		a := &articles[i]
		// Identify Lengths
		bestURL, maxLength := s.determineBestArticle(a, lengths, localLang, userLang)

		poi := &model.POI{
			WikidataID:          a.QID,
			Source:              "wikidata",
			Category:            a.Category,
			Lat:                 a.Lat,
			Lon:                 a.Lon,
			Sitelinks:           a.Sitelinks,
			NameEn:              a.TitleEn,
			NameLocal:           a.Title,
			NameUser:            a.TitleUser,
			WPURL:               bestURL,
			WPArticleLength:     maxLength,
			TriggerQID:          "",
			CreatedAt:           time.Now(),
			DimensionMultiplier: a.DimensionMultiplier,
		}

		// Populate Icon from Classifier Config
		poi.Icon = s.getIcon(a.Category)
		candidates = append(candidates, poi)
	}

	// 5. MERGE DUPLICATES (Spatial Gobbling)
	var finalPOIs []*model.POI = candidates
	if dc, ok := s.classifier.(DimClassifier); ok {
		finalPOIs = MergePOIs(candidates, dc.GetConfig(), s.logger)
	}

	// 6. Save Valid POIs
	for _, p := range finalPOIs {
		if err := s.poi.UpsertPOI(ctx, p); err != nil {
			s.logger.Error("Failed to save POI", "qid", p.WikidataID, "error", err)
		}
	}
	return nil
}

func (s *Service) determineBestArticle(a *Article, lengths map[string]map[string]int, localLang, userLang string) (url string, length int) {
	// Identify Lengths
	lenLocal := lengths[localLang][a.Title]
	lenEn := lengths["en"][a.TitleEn]
	lenUser := 0
	if userLang != "en" && userLang != localLang {
		lenUser = lengths[userLang][a.TitleUser]
	}

	// Determine Best Article (Max Length)
	maxLength := lenLocal
	bestURL := fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", localLang, strings.ReplaceAll(a.Title, " ", "_"))

	// Check En
	if lenEn > maxLength {
		maxLength = lenEn
		bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", strings.ReplaceAll(a.TitleEn, " ", "_"))
	}
	// Check User
	if lenUser > maxLength {
		maxLength = lenUser
		bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, strings.ReplaceAll(a.TitleUser, " ", "_"))
	}

	// Safety: If no lengths found (API fail), default to local URL or En
	if maxLength == 0 {
		// Fallback preference: User > En > Local
		if a.TitleUser != "" {
			bestURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", userLang, strings.ReplaceAll(a.TitleUser, " ", "_"))
		} else if a.TitleEn != "" {
			bestURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", strings.ReplaceAll(a.TitleEn, " ", "_"))
		}
	}
	return bestURL, maxLength
}

func (s *Service) filterExistingPOIs(ctx context.Context, rawArticles []Article, qids []string) []Article {
	if len(rawArticles) == 0 {
		return rawArticles
	}
	poisBatch, err := s.store.GetPOIsBatch(ctx, qids)
	if err != nil {
		s.logger.Warn("POI batch lookup failed", "error", err)
		return rawArticles
	}

	filtered := make([]Article, 0, len(rawArticles))
	for i := range rawArticles {
		if p, ok := poisBatch[rawArticles[i].QID]; !ok {
			filtered = append(filtered, rawArticles[i])
		} else {
			// CRITICAL: Even if we filter it out of the current ingestion pipeline (to avoid redundant enrichment),
			// we MUST ensure it is added to the active tracking map in poi.Manager.
			// This is especially important after a server restart when the in-memory map is empty.
			if err := s.poi.TrackPOI(ctx, p); err != nil {
				s.logger.Warn("Failed to track existing POI", "qid", p.WikidataID, "error", err)
			}
		}
	}
	return filtered
}

func (s *Service) filterSeenArticles(ctx context.Context, rawArticles []Article) []Article {
	if len(rawArticles) == 0 {
		return rawArticles
	}

	qids := make([]string, len(rawArticles))
	for i := range rawArticles {
		qids[i] = rawArticles[i].QID
	}

	seen, err := s.store.GetSeenEntitiesBatch(ctx, qids)
	if err != nil {
		s.logger.Warn("Failed to fetch seen entities", "error", err)
		return rawArticles
	}

	filtered := make([]Article, 0, len(rawArticles))
	for i := range rawArticles {
		if _, exists := seen[rawArticles[i].QID]; !exists {
			filtered = append(filtered, rawArticles[i])
		}
	}

	if len(rawArticles) != len(filtered) {
		s.logger.Debug("Filtered seen articles", "count", len(rawArticles)-len(filtered))
	}

	return filtered
}

func (s *Service) classifyAndFilterArticles(ctx context.Context, rawArticles []Article) []Article {
	candidates := s.collectUnclassifiedQIDs(rawArticles)
	if len(candidates) == 0 {
		return rawArticles
	}

	ignoredQIDs := s.classifyInChunks(ctx, rawArticles, candidates)
	if len(ignoredQIDs) > 0 {
		s.logger.Debug("Classification ignored articles", "count", len(ignoredQIDs))
	}

	// MarkEntitiesSeen is now handled inside classifyInChunks

	return s.filterByQIDs(rawArticles, ignoredQIDs)
}

func (s *Service) collectUnclassifiedQIDs(articles []Article) []string {
	result := make([]string, 0)
	for i := range articles {
		if articles[i].Category == "" {
			result = append(result, articles[i].QID)
		}
	}
	return result
}

func (s *Service) classifyInChunks(ctx context.Context, rawArticles []Article, candidates []string) []string {
	// 1. Check for locally known "seen" instances (Optimization for reprocessing)
	seenMap, err := s.store.GetSeenEntitiesBatch(ctx, candidates)
	if err != nil {
		s.logger.Warn("Failed to GetSeenEntitiesBatch", "error", err)
		seenMap = make(map[string][]string)
	}

	// 2. Identify Metadata Source
	metaCache := make(map[string]EntityMetadata)
	toFetch := make([]string, 0) // Truly unknown

	for _, qid := range candidates {
		if insts, ok := seenMap[qid]; ok && len(insts) > 0 {
			metaCache[qid] = EntityMetadata{Claims: map[string][]string{"P31": insts}}
		} else {
			toFetch = append(toFetch, qid)
		}
	}

	// 3. Fetch missing metadata
	if err := s.fetchMissingMetadata(ctx, toFetch, metaCache); err != nil {
		s.logger.Warn("Failed to fetch missing metadata", "error", err)
	}

	// 4. Batch Classification
	return s.runBatchClassification(ctx, rawArticles, metaCache)
}

func (s *Service) fetchMissingMetadata(ctx context.Context, toFetch []string, metaCache map[string]EntityMetadata) error {
	if len(toFetch) == 0 {
		return nil
	}
	chunkSize := 50
	for i := 0; i < len(toFetch); i += chunkSize {
		end := i + chunkSize
		if end > len(toFetch) {
			end = len(toFetch)
		}
		chunk := toFetch[i:end]

		meta, err := s.client.GetEntitiesBatch(ctx, chunk)
		if err != nil {
			s.logger.Warn("Wikidata batch fetch failed", "error", err, "chunk_size", len(chunk))
			continue
		}
		for id, m := range meta {
			metaCache[id] = m
		}
	}
	return nil
}

func (s *Service) runBatchClassification(ctx context.Context, rawArticles []Article, metaCache map[string]EntityMetadata) []string {
	ignoredQIDs := make([]string, 0)
	toMark := make(map[string][]string)

	batchRes := s.classifier.ClassifyBatch(ctx, metaCache)

	for qid, res := range batchRes {
		if res == nil {
			continue
		}
		if res.Ignored {
			s.setIgnoredByQID(rawArticles, qid)
			ignoredQIDs = append(ignoredQIDs, qid)
			// Prepare for saving instances
			if m, ok := metaCache[qid]; ok {
				if insts, ok := m.Claims["P31"]; ok {
					toMark[qid] = insts
				}
			}
		} else {
			s.setCategoryByQID(rawArticles, qid, res.Category)
		}
	}

	// 5. Persist ignored entities with their instances
	if len(toMark) > 0 {
		if err := s.store.MarkEntitiesSeen(ctx, toMark); err != nil {
			s.logger.Warn("Failed to mark ignored entities as seen", "error", err)
		}
	}

	return ignoredQIDs
}

func (s *Service) setCategoryByQID(articles []Article, qid, category string) {
	for j := range articles {
		if articles[j].QID == qid {
			articles[j].Category = category
			return
		}
	}
}

func (s *Service) setIgnoredByQID(articles []Article, qid string) {
	for j := range articles {
		if articles[j].QID == qid {
			articles[j].Ignored = true
			return
		}
	}
}

func (s *Service) filterByQIDs(articles []Article, excludeQIDs []string) []Article {
	excludeSet := make(map[string]bool)
	for _, qid := range excludeQIDs {
		excludeSet[qid] = true
	}

	filtered := make([]Article, 0, len(articles))
	for i := range articles {
		if !excludeSet[articles[i].QID] {
			filtered = append(filtered, articles[i])
		}
	}
	return filtered
}

func (s *Service) postProcessArticles(rawArticles []Article) (processed []Article, rescuedCount int, err error) {
	dc, isDim := s.classifier.(DimClassifier)
	if isDim {
		dc.ResetDimensions()
		for i := range rawArticles {
			h, l, area := getArticleDimensions(&rawArticles[i])
			dc.ObserveDimensions(h, l, area)
		}
	}

	processed = make([]Article, 0, len(rawArticles))
	for i := range rawArticles {
		a := &rawArticles[i]
		isPOI, rescued := s.checkPOIStatus(a, dc)

		if rescued {
			rescuedCount++
		}
		if isPOI {
			processed = append(processed, *a)
		}
	}

	if isDim {
		dc.FinalizeDimensions()
	}
	return processed, rescuedCount, nil
}

func (s *Service) checkPOIStatus(a *Article, dc DimClassifier) (isPOI, rescued bool) {
	// 0. Explicitly ignored check (from classifier)
	if a.Ignored {
		return false, false
	}

	// 1. Initial categorization check
	if a.Category != "" {
		minLinks := s.getSitelinksMin(dc, a.Category)
		if a.Sitelinks >= minLinks {
			isPOI = true
		} else {
			s.logger.Debug("Insufficient sitelinks for category", "qid", a.QID, "category", a.Category, "links", a.Sitelinks, "min", minLinks)
		}
	}

	// 2. Dimension check (highest, longest, largest)
	if dc != nil {
		h, l, area := getArticleDimensions(a)
		if dc.ShouldRescue(h, l, area, a.Instances) {
			isPOI = true
			if a.Category == "" {
				s.assignRescueCategory(a, h, l, area)
				rescued = true
			} else {
				s.logger.Debug("Article kept as Dimension Candidate", "qid", a.QID, "category", a.Category)
			}
		} else if a.Category == "" {
			// Not rescued
			s.logger.Debug("Article dropped: Unclassified and failed rescue", "qid", a.QID, "title", a.Title)
		}

		// Apply Multiplier (regardless of rescue status)
		a.DimensionMultiplier = dc.GetMultiplier(h, l, area)
		if a.DimensionMultiplier > 1.0 {
			s.logger.Debug("Dimension Multiplier applied", "qid", a.QID, "mult", a.DimensionMultiplier)
		}
	}

	return isPOI, rescued
}

func (s *Service) assignRescueCategory(a *Article, h, l, area float64) {
	switch {
	case area > 0:
		a.Category = "Area"
		s.logger.Debug("Rescued article by Area", "title", a.Title, "qid", a.QID)
	case h > 0:
		a.Category = "Height"
		s.logger.Debug("Rescued article by Height", "title", a.Title, "qid", a.QID)
	case l > 0:
		a.Category = "Length"
		s.logger.Debug("Rescued article by Length", "title", a.Title, "qid", a.QID)
	default:
		a.Category = "Landmark"
	}
}

func (s *Service) getSitelinksMin(dc DimClassifier, category string) int {
	if dc == nil {
		return 0
	}
	if cfg, ok := dc.GetConfig().Categories[category]; ok {
		return cfg.SitelinksMin
	}
	return 0
}

func (s *Service) getIcon(category string) string {
	// Attempt to get config from classifier
	type configProvider interface {
		GetConfig() *config.CategoriesConfig
	}
	if cp, ok := s.classifier.(configProvider); ok {
		if cfg, ok := cp.GetConfig().Categories[strings.ToLower(category)]; ok {
			return cfg.Icon
		}
	}
	return ""
}

func getArticleDimensions(a *Article) (h, l, area float64) {
	if a.Height != nil {
		h = *a.Height
	}
	if a.Length != nil {
		l = *a.Length
	}
	if a.Area != nil {
		area = *a.Area
	}
	return h, l, area
}

// GetCachedTiles returns a list of centers for cached tiles within radiusKm of (lat, lon).
func (s *Service) GetCachedTiles(ctx context.Context, lat, lon, radiusKm float64) ([]struct{ Lat, Lon float64 }, error) {
	keys, err := s.store.ListCacheKeys(ctx, "wd_hex_")
	if err != nil {
		return nil, fmt.Errorf("failed to list cache keys: %w", err)
	}

	var results []struct{ Lat, Lon float64 }

	for _, k := range keys {
		// Key format: wd_hex_{row}_{col}
		if len(k) <= 7 {
			continue
		}
		parts := strings.Split(k[7:], "_")
		if len(parts) != 2 {
			continue
		}

		var row, col int
		if _, err := fmt.Sscanf(parts[0], "%d", &row); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &col); err != nil {
			continue
		}

		tile := HexTile{Row: row, Col: col}
		// Uses named return: lat, lon
		cLat, cLon := s.gridCenter(tile)

		dist := DistKm(lat, lon, cLat, cLon)
		if dist <= radiusKm {
			results = append(results, struct{ Lat, Lon float64 }{Lat: cLat, Lon: cLon})
		}
	}

	return results, nil
}
