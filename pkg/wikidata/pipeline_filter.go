package wikidata

import (
	"context"

	"phileasgo/pkg/logging"
)

func (p *Pipeline) filterExistingPOIs(ctx context.Context, rawArticles []Article, qids []string) []Article {
	if len(rawArticles) == 0 {
		return rawArticles
	}
	poisBatch, err := p.store.GetPOIsBatch(ctx, qids)
	if err != nil {
		p.logger.Warn("POI batch lookup failed", "error", err)
		return rawArticles
	}

	filtered := make([]Article, 0, len(rawArticles))
	tracked := make(map[string]bool) // Dedup: avoid calling TrackPOI multiple times for same QID
	for i := range rawArticles {
		qid := rawArticles[i].QID
		if poi, ok := poisBatch[qid]; !ok {
			filtered = append(filtered, rawArticles[i])
		} else if !tracked[qid] {
			tracked[qid] = true
			if err := p.poi.TrackPOI(ctx, poi); err != nil {
				p.logger.Warn("Failed to track existing POI", "qid", poi.WikidataID, "error", err)
			}
		}
	}
	return filtered
}

func (p *Pipeline) filterSeenArticles(ctx context.Context, rawArticles []Article) []Article {
	if len(rawArticles) == 0 {
		return rawArticles
	}

	qids := make([]string, len(rawArticles))
	for i := range rawArticles {
		qids[i] = rawArticles[i].QID
	}

	seen, err := p.store.GetSeenEntitiesBatch(ctx, qids)
	if err != nil {
		p.logger.Warn("Failed to fetch seen entities", "error", err)
		return rawArticles
	}

	filtered := make([]Article, 0, len(rawArticles))
	for i := range rawArticles {
		if _, exists := seen[rawArticles[i].QID]; !exists {
			filtered = append(filtered, rawArticles[i])
		}
	}

	if len(rawArticles) != len(filtered) {
		logging.Trace(p.logger, "Filtered seen articles", "count", len(rawArticles)-len(filtered))
	}

	return filtered
}

func (p *Pipeline) classifyAndFilterArticles(ctx context.Context, rawArticles []Article) []Article {
	candidates := p.collectUnclassifiedQIDs(rawArticles)
	if len(candidates) == 0 {
		return rawArticles
	}

	ignoredQIDs := p.classifyInChunks(ctx, rawArticles, candidates)
	if len(ignoredQIDs) > 0 {
		logging.Trace(p.logger, "Classification ignored articles", "count", len(ignoredQIDs))
	}

	return p.filterByQIDs(rawArticles, ignoredQIDs)
}

func (p *Pipeline) collectUnclassifiedQIDs(articles []Article) []string {
	result := make([]string, 0)
	for i := range articles {
		if articles[i].Category == "" {
			result = append(result, articles[i].QID)
		}
	}
	return result
}

func (p *Pipeline) classifyInChunks(ctx context.Context, rawArticles []Article, candidates []string) []string {
	seenMap, err := p.store.GetSeenEntitiesBatch(ctx, candidates)
	if err != nil {
		p.logger.Warn("Failed to GetSeenEntitiesBatch", "error", err)
		seenMap = make(map[string][]string)
	}

	metaCache := make(map[string]EntityMetadata)
	candidateInstances := make(map[string][]string)
	for i := range rawArticles {
		art := &rawArticles[i]
		if len(art.Instances) > 0 {
			candidateInstances[art.QID] = art.Instances
		}
	}

	for _, qid := range candidates {
		var insts []string
		if seen, ok := seenMap[qid]; ok && len(seen) > 0 {
			insts = seen
		} else if sparqlInsts, ok := candidateInstances[qid]; ok {
			insts = sparqlInsts
		}

		if len(insts) > 0 {
			metaCache[qid] = EntityMetadata{Claims: map[string][]string{"P31": insts}}
		}
	}

	return p.runBatchClassification(ctx, rawArticles, metaCache)
}

func (p *Pipeline) runBatchClassification(ctx context.Context, rawArticles []Article, metaCache map[string]EntityMetadata) []string {
	ignoredQIDs := make([]string, 0)
	toMark := make(map[string][]string)

	batchRes := p.classifier.ClassifyBatch(ctx, metaCache)

	for qid, res := range batchRes {
		if res == nil {
			continue
		}
		if res.Ignored {
			p.setIgnoredByQID(rawArticles, qid)
			ignoredQIDs = append(ignoredQIDs, qid)
			if m, ok := metaCache[qid]; ok {
				if insts, ok := m.Claims["P31"]; ok {
					toMark[qid] = insts
				}
			}
		} else {
			p.setCategoryByQID(rawArticles, qid, res.Category)
		}
	}

	if len(toMark) > 0 {
		if err := p.store.MarkEntitiesSeen(ctx, toMark); err != nil {
			p.logger.Warn("Failed to mark ignored entities as seen", "error", err)
		}
	}

	return ignoredQIDs
}

func (p *Pipeline) setCategoryByQID(articles []Article, qid, category string) {
	for j := range articles {
		if articles[j].QID == qid {
			articles[j].Category = category
			return
		}
	}
}

func (p *Pipeline) setIgnoredByQID(articles []Article, qid string) {
	for j := range articles {
		if articles[j].QID == qid {
			articles[j].Ignored = true
			return
		}
	}
}

func (p *Pipeline) filterByQIDs(articles []Article, excludeQIDs []string) []Article {
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

func (p *Pipeline) postProcessArticles(rawArticles []Article) (processed []Article, rescuedCount int, err error) {
	dc, isDim := p.classifier.(DimClassifier)
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
		isPOI, rescued := p.checkPOIStatus(a, dc)

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

func (p *Pipeline) checkPOIStatus(a *Article, dc DimClassifier) (isPOI, rescued bool) {
	if a.Ignored {
		return false, false
	}

	if a.Category != "" {
		minLinks := p.getSitelinksMin(dc, a.Category)
		if a.Sitelinks >= minLinks {
			isPOI = true
		}
	}

	if dc != nil {
		h, l, area := getArticleDimensions(a)
		if dc.ShouldRescue(h, l, area, a.Instances) {
			isPOI = true
			if a.Category == "" {
				p.assignRescueCategory(a, h, l, area)
				rescued = true
			} else {
				p.logger.Debug("Article kept as Dimension Candidate", "qid", a.QID, "category", a.Category)
			}
		}
		a.DimensionMultiplier = dc.GetMultiplier(h, l, area)
		if a.DimensionMultiplier > 1.0 {
			logging.Trace(p.logger, "Dimension Multiplier applied", "qid", a.QID, "mult", a.DimensionMultiplier)
		}
	}

	return isPOI, rescued
}

func (p *Pipeline) assignRescueCategory(a *Article, h, l, area float64) {
	switch {
	case area > 0:
		a.Category = "Area"
		p.logger.Debug("Rescued article by Area", "title", a.LocalTitles, "qid", a.QID)
	case h > 0:
		a.Category = "Height"
		p.logger.Debug("Rescued article by Height", "title", a.LocalTitles, "qid", a.QID)
	case l > 0:
		a.Category = "Length"
		logging.Trace(p.logger, "Rescued article by Length", "title", a.LocalTitles, "qid", a.QID)
	default:
		a.Category = "Landmark"
	}
}

func (p *Pipeline) getSitelinksMin(dc DimClassifier, category string) int {
	if dc == nil {
		return 0
	}
	if cfg, ok := dc.GetConfig().Categories[category]; ok {
		return cfg.SitelinksMin
	}
	return 0
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
