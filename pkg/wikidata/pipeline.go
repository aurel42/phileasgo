package wikidata

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/logging"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/rescue"
	"phileasgo/pkg/store"
)

// Pipeline handles the logic of processing tiles, classifying articles, and enriching them.
type Pipeline struct {
	store      store.Store
	client     ClientInterface
	wiki       WikipediaProvider
	geo        *geo.Service
	poi        *poi.Manager
	grid       *Grid
	mapper     *LanguageMapper
	classifier Classifier
	cfgProv    config.Provider
	logger     *slog.Logger
}

// NewPipeline creates a new Pipeline.
func NewPipeline(st store.Store, cl ClientInterface, w WikipediaProvider, g *geo.Service, p *poi.Manager, gr *Grid, m *LanguageMapper, c Classifier, cfgProv config.Provider, log *slog.Logger) *Pipeline {
	return &Pipeline{
		store:      st,
		client:     cl,
		wiki:       w,
		geo:        g,
		poi:        p,
		grid:       gr,
		mapper:     m,
		classifier: c,
		cfgProv:    cfgProv,
		logger:     log,
	}
}

// ProcessTileData takes raw SPARQL JSON, parses it, runs classification, ENRICHES, and SAVES to DB.
func (p *Pipeline) ProcessTileData(ctx context.Context, rawJSON []byte, centerLat, centerLon float64, force bool, medians rescue.MedianStats) (articles, rawArticles []Article, rescuedCount int, err error) {
	// 1. Parse Response (Zero-Alloc Streaming)
	rawArticles, _, err = ParseSPARQLStreaming(strings.NewReader(string(rawJSON)))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%w: failed to parse sparql stream: %v", ErrParse, err)
	}
	qids := make([]string, len(rawArticles))
	for i := range rawArticles {
		qids[i] = rawArticles[i].QID
	}

	// 2. Filter out already existing POIs (Drop them immediately)
	rawArticles = p.filterExistingPOIs(ctx, rawArticles, qids)

	// 3. Filter seen articles (drop them immediately), UNLESS forced
	if !force {
		rawArticles = p.filterSeenArticles(ctx, rawArticles)
	}

	// 4. Unified Processing Flow
	processed, all, rescued, err := p.ProcessEntities(ctx, rawArticles, centerLat, centerLon, medians)
	return processed, all, rescued, err
}

// ProcessEntities takes a slice of Articles (usually from SPARQL parsing) and runs them through the full pipeline:
// Classification -> Filtering -> Hydration -> Enrichment -> Saving.
func (p *Pipeline) ProcessEntities(ctx context.Context, articles []Article, lat, lon float64, medians rescue.MedianStats) (processed, all []Article, rescuedCount int, err error) {
	if len(articles) == 0 {
		return nil, nil, 0, nil
	}

	// 1. Batch Classification for new articles (marks Ignored/Category)
	p.classifyArticlesOnly(ctx, articles)

	// Keep a copy of all articles for stats (including those marked Ignored)
	all = make([]Article, len(articles))
	copy(all, articles)

	// Filter out ignored for the rest of the pipeline
	articles = p.filterIgnoredArticles(articles)

	// 2. Compute Allowed Languages for Filter
	localLangs := p.getLangsForLocation(ctx, lat, lon)

	// 3. Hydrate & Filter
	processed, rescuedCount, err = p.processAndHydrate(ctx, articles, lat, lon, localLangs, medians)
	if err != nil {
		return nil, nil, 0, err
	}

	// 4. Enrich & Save
	if len(processed) > 0 {
		if err := p.enrichAndSave(ctx, processed, localLangs, "en"); err != nil {
			return nil, nil, 0, err
		}
	}

	return processed, all, rescuedCount, nil
}

func (p *Pipeline) getLangsForLocation(ctx context.Context, lat, lon float64) []string {
	countrySet := make(map[string]struct{})
	countrySet[p.geo.GetCountry(lat, lon)] = struct{}{}

	// Only check neighbors if this is a H3 tile center (approx check)
	tile := p.grid.TileAt(lat, lon)
	tLat, tLon := p.grid.TileCenter(tile)
	const precision = 0.001
	if math.Abs(lat-tLat) < precision && math.Abs(lon-tLon) < precision {
		for _, corner := range p.grid.TileCorners(tile) {
			countrySet[p.geo.GetCountry(corner.Lat, corner.Lon)] = struct{}{}
		}
	}

	langSet := make(map[string]struct{})
	for country := range countrySet {
		langs := p.mapper.GetLanguages(country)
		for _, l := range langs {
			code := l.Code
			if len(code) > 2 && strings.Contains(code, "-") {
				code = strings.Split(code, "-")[0]
			}
			langSet[code] = struct{}{}
		}
	}
	langSet["en"] = struct{}{}

	userLang := p.cfgProv.TargetLanguage(ctx)
	if userLang != "" {
		// Normalize userLang (e.g. "en-US" -> "en")
		normalizedLang := userLang
		if len(userLang) > 2 {
			normalizedLang = strings.Split(userLang, "-")[0]
		}
		langSet[normalizedLang] = struct{}{}
	}

	var localLangs []string
	for l := range langSet {
		localLangs = append(localLangs, l)
	}
	return localLangs
}

func (p *Pipeline) processAndHydrate(ctx context.Context, rawArticles []Article, centerLat, centerLon float64, allowedLangs []string, medians rescue.MedianStats) (processed []Article, rescuedCount int, err error) {
	processed, rescuedCount, err = p.postProcessArticles(rawArticles, centerLat, centerLon, medians)
	if err != nil {
		return nil, 0, err
	}

	if len(processed) == 0 {
		return nil, 0, nil
	}

	// Merge logic (now uses processed since they have categories)
	var candidates []Article
	candidates = processed

	// Remove old DimClassifier check, just use base classifier
	cfg := p.classifier.GetConfig()
	var rejected []string
	candidates, rejected = MergeArticles(candidates, cfg, p.logger)

	if len(rejected) > 0 {
		toMark := make(map[string][]string)
		for _, qid := range rejected {
			toMark[qid] = []string{"merged"}
		}
		if err := p.store.MarkEntitiesSeen(ctx, toMark); err != nil {
			p.logger.Warn("Failed to mark merged-away articles as seen", "error", err)
		}
	}

	logging.Trace(p.logger, "Pipeline: Hydrating candidates", "count", len(candidates), "qids", getQIDs(candidates))

	hydrated, err := p.hydrateCandidates(ctx, candidates, allowedLangs)
	if err != nil {
		return nil, 0, err
	}

	logging.Trace(p.logger, "Pipeline: Hydration complete", "input", len(candidates), "hydrated", len(hydrated))

	return hydrated, rescuedCount, nil
}

func getQIDs(articles []Article) []string {
	qids := make([]string, len(articles))
	for i := range articles {
		qids[i] = articles[i].QID
	}
	return qids
}
