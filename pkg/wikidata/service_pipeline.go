package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"phileasgo/pkg/config"
)

// ProcessTileData takes raw SPARQL JSON, parses it, runs classification, ENRICHES, and SAVES to DB.
func (s *Service) ProcessTileData(ctx context.Context, rawJSON []byte, centerLat, centerLon float64, force bool) (articles []Article, rescuedCount int, err error) {
	var result sparqlResponse
	if err := json.Unmarshal(rawJSON, &result); err != nil {
		return nil, 0, fmt.Errorf("%w: failed to unmarshal raw json: %v", ErrParse, err)
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

	// 4. Compute Allowed Languages for Filter
	// We need this BEFORE hydration to avoid fetching 300+ languages per item
	countrySet := make(map[string]struct{})
	// Sample Center
	countrySet[s.geo.GetCountry(centerLat, centerLon)] = struct{}{}
	// Sample Corners
	tile := s.scheduler.grid.TileAt(centerLat, centerLon)
	for _, corner := range s.scheduler.grid.TileCorners(tile) {
		countrySet[s.geo.GetCountry(corner.Lat, corner.Lon)] = struct{}{}
	}

	langSet := make(map[string]struct{})
	for country := range countrySet {
		langs := s.mapper.GetLanguages(country)
		for _, l := range langs {
			langSet[l.Code] = struct{}{}
		}
	}
	// Always allow English
	langSet["en"] = struct{}{}
	// Allow User Language
	if s.userLang != "" {
		langSet[s.userLang] = struct{}{}
	}

	var localLangs []string
	for l := range langSet {
		localLangs = append(localLangs, l)
	}

	// 5. Process, Filter, and Hydrate
	processed, rescued, err := s.processAndHydrate(ctx, rawArticles, centerLat, centerLon, localLangs)
	if err != nil {
		return nil, 0, err
	}

	if len(processed) > 0 {
		err = s.enrichAndSave(ctx, processed, localLangs, "en")
	}

	return processed, rescued, err
}

func (s *Service) processAndHydrate(ctx context.Context, rawArticles []Article, centerLat, centerLon float64, allowedLangs []string) (processed []Article, rescuedCount int, err error) {
	// 1. Post-processing (Rescue & Filters) - Operates on Skeleton Data (P31, Dimensions, Sitelinks)
	candidates, rescued, err := s.postProcessArticles(rawArticles)
	if err != nil {
		return nil, 0, err
	}

	if len(candidates) == 0 {
		return nil, 0, nil
	}

	// 1b. Optimization: Merge *BEFORE* Hydration (Save API calls)
	// We use "Sitelinks" as the proxy for importance here, instead of Article Length.
	// This reduces the number of items we need to fetch Titles/Lengths for.
	if dc, ok := s.classifier.(DimClassifier); ok {
		candidates = MergeArticles(candidates, dc.GetConfig(), s.logger)
	}

	// 2. Hydration (Fetch Titles/Labels for survivors)
	hydrated, err := s.hydrateCandidates(ctx, candidates, allowedLangs)
	if err != nil {
		// If hydration fails, we might still want to proceed partial?
		// No, without titles we can't get lengths or build URLs. Fail.
		return nil, 0, err
	}

	return hydrated, rescued, nil
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
	// We priorities SPARQL Instances (from rawArticles) as they are the source of truth for the tile.
	metaCache := make(map[string]EntityMetadata)

	// Build a lookup for candidate instances from rawArticles
	// Using a map for quick access
	candidateInstances := make(map[string][]string)
	for i := range rawArticles {
		art := &rawArticles[i]
		if len(art.Instances) > 0 {
			candidateInstances[art.QID] = art.Instances
		}
	}

	for _, qid := range candidates {
		var insts []string

		// Priority 1: Seen Map (Override from previous runs?) - Should we trust it?
		// Actually, standardizing on SPARQL results is safer for "current state".
		// But seenMap might contain more detailed history or merged instances.
		// Let's check seenMap first, then SPARQL.
		if seen, ok := seenMap[qid]; ok && len(seen) > 0 {
			insts = seen
		} else if sparqlInsts, ok := candidateInstances[qid]; ok {
			insts = sparqlInsts
		}

		// If we have instances, cache them.
		if len(insts) > 0 {
			metaCache[qid] = EntityMetadata{Claims: map[string][]string{"P31": insts}}
		}
		// If we DON'T have instances, we now accept we can't classify it.
		// We DO NOT fetch anymore.
	}

	// 3. (REMOVED) Fetch missing metadata
	// The call to s.fetchMissingMetadata is deleted to prevent redundant/fragile API calls.

	// 4. Batch Classification
	return s.runBatchClassification(ctx, rawArticles, metaCache)
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
		s.logger.Debug("Rescued article by Area", "title", a.LocalTitles, "qid", a.QID)
	case h > 0:
		a.Category = "Height"
		s.logger.Debug("Rescued article by Height", "title", a.LocalTitles, "qid", a.QID)
	case l > 0:
		a.Category = "Length"
		s.logger.Debug("Rescued article by Length", "title", a.LocalTitles, "qid", a.QID)
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
