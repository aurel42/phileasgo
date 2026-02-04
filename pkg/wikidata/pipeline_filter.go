package wikidata

import (
	"context"

	"phileasgo/pkg/logging"
	"phileasgo/pkg/rescue"
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

func (p *Pipeline) classifyArticlesOnly(ctx context.Context, rawArticles []Article) {
	candidates := p.collectUnclassifiedQIDs(rawArticles)
	if len(candidates) == 0 {
		return
	}

	p.classifyInChunks(ctx, rawArticles, candidates)
}

func (p *Pipeline) filterIgnoredArticles(rawArticles []Article) []Article {
	filtered := make([]Article, 0, len(rawArticles))
	for i := range rawArticles {
		if !rawArticles[i].Ignored {
			filtered = append(filtered, rawArticles[i])
		}
	}
	return filtered
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

func (p *Pipeline) postProcessArticles(rawArticles []Article, lat, lon float64, medians rescue.MedianStats) (processed []Article, rescuedCount int, err error) {
	// Separate candidates for rescue (those with no category) from those already classified
	var candidates []Article
	processed = make([]Article, 0, len(rawArticles))

	for i := range rawArticles {
		a := &rawArticles[i]
		if a.Ignored {
			continue
		}

		if a.Category != "" {
			// Already classified or ignored by category
			minLinks := p.getSitelinksMin(a.Category)
			if a.Sitelinks >= minLinks {
				processed = append(processed, *a)
			} else {
				logging.Trace(p.logger, "Pipeline: Dropped low sitelinks", "qid", a.QID, "sitelinks", a.Sitelinks, "min", minLinks)
			}
		} else {
			// Candidate for rescue
			candidates = append(candidates, *a)
		}
	}

	// Rescue Logic
	if len(candidates) > 0 {
		rescuedCount = p.rescueFromBatch(candidates, lat, lon, medians, &processed)
	}

	return processed, rescuedCount, nil
}

func (p *Pipeline) rescueFromBatch(candidates []Article, lat, lon float64, medians rescue.MedianStats, processed *[]Article) int {
	rescueCandidates := make([]rescue.Article, len(candidates))
	for i := range candidates {
		rescueCandidates[i] = rescue.Article{
			ID:     candidates[i].QID,
			Height: candidates[i].Height,
			Length: candidates[i].Length,
			Area:   candidates[i].Area,
		}
	}

	// Determine local tile max
	localMax := rescue.AnalyzeTile(lat, lon, rescueCandidates)

	// Determine rescued
	cfg := p.cfgProv.AppConfig().Wikidata.Rescue.PromoteByDimension
	rescued := rescue.Batch(rescueCandidates, localMax, medians, cfg.MinHeight, cfg.MinLength, cfg.MinArea)

	// Concise Logging
	p.logRescueStats(lat, lon, localMax, medians, rescueCandidates, rescued)

	// Apply back
	count := 0
	for _, ra := range rescued {
		for i := range candidates {
			if candidates[i].QID != ra.ID {
				continue
			}
			candidates[i].Category = ra.Category
			candidates[i].DimensionMultiplier = ra.DimensionMultiplier
			logging.Trace(p.logger, "Rescued article by dimension", "qid", ra.ID, "category", ra.Category, "multiplier", ra.DimensionMultiplier)
			*processed = append(*processed, candidates[i])
			count++
			break
		}
	}
	return count
}

func (p *Pipeline) logRescueStats(lat, lon float64, localMax rescue.TileStats, medians rescue.MedianStats, candidates, rescued []rescue.Article) {
	if localMax.MaxHeight <= 0 && localMax.MaxLength <= 0 && localMax.MaxArea <= 0 {
		return
	}

	var maxHQID, maxLQID, maxAQID string
	for _, c := range candidates {
		if c.Height != nil && *c.Height == localMax.MaxHeight && maxHQID == "" {
			maxHQID = c.ID
		}
		if c.Length != nil && *c.Length == localMax.MaxLength && maxLQID == "" {
			maxLQID = c.ID
		}
		if c.Area != nil && *c.Area == localMax.MaxArea && maxAQID == "" {
			maxAQID = c.ID
		}
	}

	key := p.grid.TileAt(lat, lon).Key()
	logging.Trace(p.logger, "Dimension Rescue Tile Stats",
		"tile", key,
		"max_h", localMax.MaxHeight, "max_h_qid", maxHQID, "med_h", medians.MedianHeight,
		"max_l", localMax.MaxLength, "max_l_qid", maxLQID, "med_l", medians.MedianLength,
		"max_a", localMax.MaxArea, "max_a_qid", maxAQID, "med_a", medians.MedianArea,
		"rescued", len(rescued),
	)
}

func (p *Pipeline) getSitelinksMin(category string) int {
	if cfg, ok := p.classifier.GetConfig().Categories[category]; ok {
		return cfg.SitelinksMin
	}
	return 0
}
