package wikidata

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"phileasgo/pkg/geo"
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
	logger     *slog.Logger
	userLang   string
}

// NewPipeline creates a new Pipeline.
func NewPipeline(st store.Store, cl ClientInterface, w WikipediaProvider, g *geo.Service, p *poi.Manager, gr *Grid, m *LanguageMapper, c Classifier, log *slog.Logger, lang string) *Pipeline {
	return &Pipeline{
		store:      st,
		client:     cl,
		wiki:       w,
		geo:        g,
		poi:        p,
		grid:       gr,
		mapper:     m,
		classifier: c,
		logger:     log,
		userLang:   lang,
	}
}

// ProcessTileData takes raw SPARQL JSON, parses it, runs classification, ENRICHES, and SAVES to DB.
func (p *Pipeline) ProcessTileData(ctx context.Context, rawJSON []byte, centerLat, centerLon float64, force bool, medians rescue.MedianStats) (articles []Article, rescuedCount int, err error) {
	// Use the exposed streaming parser from client.go
	// Note: We need a Reader, so we wrap the byte slice
	rawArticles, _, err := parseSPARQLStreaming(strings.NewReader(string(rawJSON)))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: failed to parse sparql stream: %v", ErrParse, err)
	}
	qids := make([]string, len(rawArticles))
	for i := range rawArticles {
		qids[i] = rawArticles[i].QID
	}

	// 1. Filter out already existing POIs (Drop them immediately)
	rawArticles = p.filterExistingPOIs(ctx, rawArticles, qids)

	// 2. Filter seen articles (drop them immediately), UNLESS forced
	if !force {
		rawArticles = p.filterSeenArticles(ctx, rawArticles)
	}

	// 3. Batch Classification for new articles (also filters out ignored)
	rawArticles = p.classifyAndFilterArticles(ctx, rawArticles)

	// 4. Compute Allowed Languages for Filter
	countrySet := make(map[string]struct{})
	countrySet[p.geo.GetCountry(centerLat, centerLon)] = struct{}{}
	tile := p.grid.TileAt(centerLat, centerLon)
	for _, corner := range p.grid.TileCorners(tile) {
		countrySet[p.geo.GetCountry(corner.Lat, corner.Lon)] = struct{}{}
	}

	langSet := make(map[string]struct{})
	for country := range countrySet {
		langs := p.mapper.GetLanguages(country)
		for _, l := range langs {
			langSet[l.Code] = struct{}{}
		}
	}
	langSet["en"] = struct{}{}
	if p.userLang != "" {
		langSet[p.userLang] = struct{}{}
	}

	var localLangs []string
	for l := range langSet {
		localLangs = append(localLangs, l)
	}

	// 5. Process, Filter, and Hydrate
	processed, rescued, err := p.processAndHydrate(ctx, rawArticles, centerLat, centerLon, localLangs, medians)
	if err != nil {
		return nil, 0, err
	}

	if len(processed) > 0 {
		err = p.enrichAndSave(ctx, processed, localLangs, "en")
	}

	return processed, rescued, err
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

	hydrated, err := p.hydrateCandidates(ctx, candidates, allowedLangs)
	if err != nil {
		return nil, 0, err
	}

	return hydrated, rescuedCount, nil
}
