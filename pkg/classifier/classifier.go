package classifier

import (
	"context"
	"fmt"
	"log/slog"

	"phileasgo/pkg/config"
	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/wikidata"
	"sync"
)

const (
	catIgnored = "__IGNORED__"
	catDeadEnd = "__DEADEND__"
)

// WikidataClient defines the interface for interacting with Wikidata
type WikidataClient interface {
	GetEntityClaims(ctx context.Context, id, property string) ([]string, string, error)
	GetEntityClaimsBatch(ctx context.Context, ids []string, property string) (claims map[string][]string, labels map[string]string, err error)
	GetEntitiesBatch(ctx context.Context, ids []string) (map[string]wikidata.EntityMetadata, error)
}

// Classifier handles "Smart" classification of Wikidata items
type Classifier struct {
	store              store.HierarchyStore
	client             WikidataClient // Interface for testability
	config             *config.CategoriesConfig
	lookup             config.CategoryLookup
	tracker            *tracker.Tracker
	regionalCategories config.CategoryLookup
	mu                 sync.RWMutex
}

// NewClassifier creates a new classifier
func NewClassifier(s store.HierarchyStore, c WikidataClient, cfg *config.CategoriesConfig, tr *tracker.Tracker) *Classifier {
	return &Classifier{
		store:   s,
		client:  c,
		config:  cfg,
		lookup:  cfg.BuildLookup(),
		tracker: tr,
	}
}

// Classify determines the category for a given QID (usually an Article instance).
// It does NOT cache the article QID itself in the hierarchy table, but it DOES
// cache all hierarchy nodes (classes) it traverses.
func (c *Classifier) Classify(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	// 1. Config Check (for known roots or direct matches)
	if catName, _, ok := c.getLookupMatch(qid); ok {
		return c.resultFor(catName), nil
	}

	// 2. Fetch Instances (P31) - This is the starting point for articles
	targets, _, err := c.client.GetEntityClaims(ctx, qid, "P31")
	if err != nil {
		return nil, err
	}

	// 3. Classify based on instances
	// 3. Classify based on instances - Prioritize MATCH over IGNORE
	var bestRes *model.ClassificationResult
	for _, inst := range targets {
		res, err := c.classifyHierarchyNode(ctx, inst)
		if err != nil {
			slog.Warn("failed to classify hierarchy node", "inst", inst, "err", err)
			continue
		}
		if res != nil {
			if !res.Ignored {
				// Found a real match! Return immediately
				return res, nil
			}
			// Keep track of ignore result, but keep looking for a real match
			if bestRes == nil {
				bestRes = res
			}
		}
	}

	if bestRes != nil {
		return bestRes, nil
	}

	// No match found for this article
	return nil, nil
}

// ExplanationResult provides details about classification
type ExplanationResult struct {
	Category     string
	Size         string
	Ignored      bool
	Reason       string
	MatchedQID   string
	SitelinksMin int
}

// Explain analyzes a QID and returns details on why it was classified (or not).
func (c *Classifier) Explain(ctx context.Context, qid string) (*ExplanationResult, error) {
	// 1. Instances (P31)
	targets, _, err := c.client.GetEntityClaims(ctx, qid, "P31")
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return &ExplanationResult{Reason: "No P31 instances found"}, nil
	}

	// 2. Check each - Prioritize MATCH over IGNORE
	var bestRes *model.ClassificationResult
	var bestInst string

	for _, inst := range targets {
		res, err := c.classifyHierarchyNode(ctx, inst)
		if err != nil {
			return nil, err
		}
		if res != nil {
			if !res.Ignored {
				// Found proper match
				sMin := 0
				if cat, ok := c.config.Categories[res.Category]; ok {
					sMin = cat.SitelinksMin
				}

				return &ExplanationResult{
					Category:     res.Category,
					Size:         res.Size,
					Reason:       fmt.Sprintf("Matched via instance %s", inst),
					MatchedQID:   inst,
					SitelinksMin: sMin,
				}, nil
			}
			// Track ignored result as fallback
			if bestRes == nil {
				bestRes = res
				bestInst = inst
			}
		}
	}

	if bestRes != nil {
		return &ExplanationResult{
			Ignored:    true,
			Reason:     fmt.Sprintf("Ignored via instance %s", bestInst),
			MatchedQID: bestInst,
		}, nil
	}

	return &ExplanationResult{Reason: fmt.Sprintf("Traversed %d instances, no category match found", len(targets))}, nil
}

// ClassifyBatch determines categories for a batch of articles (by QID) using pre-fetched metadata.
func (c *Classifier) ClassifyBatch(ctx context.Context, entities map[string]wikidata.EntityMetadata) map[string]*model.ClassificationResult {
	results := make(map[string]*model.ClassificationResult)

	for qid, meta := range entities {
		// 1. Config Check
		if catName, _, ok := c.getLookupMatch(qid); ok {
			results[qid] = c.resultFor(catName)
			continue
		}

		// 2. Classify based on pre-fetched P31 instances
		// Prioritize MATCH over IGNORE
		var bestRes *model.ClassificationResult
		for _, inst := range meta.Claims["P31"] {
			res, err := c.classifyHierarchyNode(ctx, inst)
			if err != nil {
				continue
			}
			if res != nil {
				if !res.Ignored {
					// Found a real match! Set immediately and break
					bestRes = res
					break
				}
				// Keep track of ignore result, but keep looking for a real match
				if bestRes == nil {
					bestRes = res
				}
			}
		}

		if bestRes != nil {
			results[qid] = bestRes
		} else {
			results[qid] = nil
		}
	}

	return results
}

// classifyHierarchyNode determines the category for a taxonomy QID (a class).
// Results for these nodes ARE cached in the wikidata_hierarchy table.
func (c *Classifier) classifyHierarchyNode(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	// 1. Config Check (for known roots like "Q62447" -> "Aerodrome")
	if catName, _, ok := c.getLookupMatch(qid); ok {
		return c.resultFor(catName), nil
	}

	// 2. Fast Path: DB Cache
	storedCat, found, err := c.store.GetClassification(ctx, qid)
	if err == nil && found {
		switch storedCat {
		case catIgnored:
			if !c.HasRegionalCategories() {
				// Cached as explicitly ignored, and no regional overrides active
				return &model.ClassificationResult{Ignored: true}, nil
			}
			// Regional categories active: must re-evaluate this ignored node
			slog.Debug("Bypassing __IGNORED__ sentinel due to active regional categories", "qid", qid)
		case catDeadEnd:
			if !c.HasRegionalCategories() {
				// Cached as a Dead End, no regional overrides active
				return nil, nil
			}
			// Regional categories active: must re-evaluate this dead end
			slog.Debug("Bypassing __DEADEND__ sentinel due to active regional categories", "qid", qid)
		default:
			if storedCat != "" {
				return c.resultFor(storedCat), nil
			}
		}
		// Continue if completely empty (legacy or intermediate) or if sentinel was bypassed
	}

	// 3. Slow Path: Graph Traversal (Subclass Of P279)
	return c.slowPathHierarchy(ctx, qid)
}

func (c *Classifier) slowPathHierarchy(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	// 1. Check structural cache (Hierarchy table)
	var subclasses []string
	var label string

	hNode, err := c.store.GetHierarchy(ctx, qid)
	if err == nil && hNode != nil {
		subclasses = hNode.Parents
		label = hNode.Name
		if hNode.Category != "" {
			switch hNode.Category {
			case catIgnored:
				if !c.HasRegionalCategories() {
					return &model.ClassificationResult{Ignored: true}, nil
				}
				slog.Debug("slowPathHierarchy: Bypassing __IGNORED__ sentinel due to active regional categories", "qid", qid)
			case catDeadEnd:
				if !c.HasRegionalCategories() {
					return nil, nil
				}
				slog.Debug("slowPathHierarchy: Bypassing __DEADEND__ sentinel due to active regional categories", "qid", qid)
			default:
				return c.resultFor(hNode.Category), nil
			}
		}
	} else {
		// 2. Fetch from API if not in structural cache
		var errFetch error
		subclasses, label, errFetch = c.client.GetEntityClaims(ctx, qid, "P279")
		if errFetch != nil {
			return nil, errFetch
		}
	}

	// Immediate check for category match or ignored
	// Prioritize MATCH over IGNORE for direct parents

	// 1. Check ALL parents for a match first
	for _, sub := range subclasses {
		if catName, isRegional, ok := c.getLookupMatch(sub); ok {
			logging.TraceDefault("Match found as direct subclass", "qid", qid, "matched", sub, "category", catName)
			return c.finalizeMatch(ctx, qid, catName, subclasses, label, isRegional)
		}
	}

	// 2. Only if no match, check for ignored
	for _, sub := range subclasses {
		if _, ok := c.config.IgnoredCategories[sub]; ok {
			logging.TraceDefault("Ignored category found as direct subclass", "qid", qid, "ignored", sub)
			return c.finalizeIgnored(ctx, qid, subclasses, label)
		}
	}

	return c.searchHierarchy(ctx, qid, subclasses, label)
}

// searchHierarchy performs the BFS traversal for hierarchy discovery.
func (c *Classifier) searchHierarchy(ctx context.Context, qid string, subclasses []string, label string) (*model.ClassificationResult, error) {
	// BFS Queue (Layered)
	visited := make(map[string]bool)
	visited[qid] = true
	for _, s := range subclasses {
		visited[s] = true
	}

	// Track all traversed nodes so we can propagate __IGNORED__ to entire path
	var allTraversed []string
	allTraversed = append(allTraversed, subclasses...)

	queue := subclasses
	maxDepth := 4
	currentDepth := 1

	for len(queue) > 0 && currentDepth <= maxDepth {
		// 1. Filter & Layer Scan: Check cache for matches/ignores
		toFetch, parentsFromCache, layerMatch, layerIgnore, layerMatchRegional := c.scanLayerCache(ctx, queue)

		// 2. Fetch: Batch request for missing nodes
		if len(toFetch) > 0 {
			fetchedParents := c.fetchAndCacheLayer(ctx, toFetch)
			for id, parents := range fetchedParents {
				parentsFromCache[id] = parents
				m, ig, isRegional := c.checkFetchedForMatches(id, parents)
				if layerMatch == "" {
					layerMatch = m
					layerMatchRegional = isRegional
				}
				if layerIgnore == "" {
					layerIgnore = ig
				}
			}
		}

		// 3. Layer Priority Check
		if layerMatch != "" {
			return c.finalizeMatch(ctx, qid, layerMatch, subclasses, label, layerMatchRegional)
		}

		// 4. Build Next Layer & Final Layer Check
		nextQueue, matchedCat, ignoredCat, matchedRegional := c.buildNextLayer(queue, parentsFromCache, visited)
		if matchedCat != "" {
			return c.finalizeMatch(ctx, qid, matchedCat, subclasses, label, matchedRegional)
		}

		// If we found an ignore in this layer (either cached or direct parent match),
		// and no match was found in the whole layer scan, then we are ignored.
		if layerIgnore != "" || ignoredCat != "" {
			c.propagateIgnored(ctx, allTraversed)
			return c.finalizeIgnored(ctx, qid, subclasses, label)
		}

		// Track newly discovered nodes for potential ignored propagation
		allTraversed = append(allTraversed, nextQueue...)

		queue = nextQueue
		currentDepth++
	}

	// Result: Miss if we exit loop - mark as Dead End
	_ = c.store.SaveClassification(ctx, qid, catDeadEnd, subclasses, label)
	return nil, nil
}

// propagateIgnored marks all nodes in the BFS path as __IGNORED__ to prevent
func (c *Classifier) scanLayerCache(ctx context.Context, queue []string) (toFetch []string, parentsFromCache map[string][]string, layerMatch, layerIgnore string, layerMatchRegional bool) {
	toFetch = make([]string, 0, len(queue))
	parentsFromCache = make(map[string][]string)

	hasRegional := c.HasRegionalCategories()

	for _, id := range queue {
		match, parents, foundInDB := c.checkCacheOrDB(ctx, id)
		if !foundInDB {
			toFetch = append(toFetch, id)
			continue
		}

		if match != "" {
			switch match {
			case catIgnored:
				if !hasRegional {
					layerIgnore = id
				}
			case catDeadEnd:
				// Bypass sentinel if regional overrides are active (handled inherently by parents array)
			default:
				if layerMatch == "" {
					layerMatch = match
					layerMatchRegional = false // If it came from DB, it's not a fresh regional match. (Wait, if it was regional, we wouldn't have saved it to DB! So DB matches are always static.)
				}
			}
		}

		// If sentinels were bypassed due to regional config, `match` might now be ""
		// But we STILL don't want to re-fetch it from Wiki. We use its cached parents.
		parentsFromCache[id] = parents
	}
	return
}

func (c *Classifier) checkFetchedForMatches(id string, parents []string) (match, ignore string, isRegional bool) {
	for _, p := range parents {
		if cat, reg, ok := c.getLookupMatch(p); ok {
			match = cat
			isRegional = reg
			return
		}
		if _, ok := c.config.IgnoredCategories[p]; ok {
			ignore = id
		}
	}
	return
}

// future traversals from having to re-discover the same ignored chain.
func (c *Classifier) propagateIgnored(ctx context.Context, nodes []string) {
	for _, node := range nodes {
		// Update node to __IGNORED__ (upsert - will update if exists)
		if err := c.store.SaveClassification(ctx, node, catIgnored, nil, ""); err != nil {
			slog.Warn("Failed to propagate ignored to node", "node", node, "error", err)
		}
	}
}

// buildNextLayer processes current queue and parents to produce next queue layer.
// Returns nextQueue, matchedCategory (if found), ignoredCategory (if found), and matchedRegional.
func (c *Classifier) buildNextLayer(queue []string, parentsFromCache map[string][]string, visited map[string]bool) (nextQueue []string, matchedCat, ignoredCat string, matchedRegional bool) {
	nextQueue = make([]string, 0)
	var foundIgnored string

	// 1. Scan ALL parents for a match first
	for _, id := range queue {
		parents := parentsFromCache[id]
		for _, p := range parents {
			// Check for matching category
			if catName, isRegional, ok := c.getLookupMatch(p); ok {
				return nil, catName, "", isRegional
			}
		}
	}

	// 2. Scan for ignored or descend
	for _, id := range queue {
		parents := parentsFromCache[id]
		for _, p := range parents {
			// Check for ignored category
			if _, ok := c.config.IgnoredCategories[p]; ok {

				foundIgnored = p
				// Don't return yet, keep scanning for matches in this layer!
				// Actually we already scanned for matches above.
				// But we need to be careful not to return ignored if a sibling parent matches?
				// The match scan above covers the whole layer.
				continue
			}

			if !visited[p] {
				visited[p] = true
				nextQueue = append(nextQueue, p)
			}
		}
	}

	if foundIgnored != "" {
		return nil, "", foundIgnored, false
	}

	return nextQueue, "", "", false
}

// checkCacheOrDB checks if a node is already classified or cached in DB.
// Returns matchedCategory if found as a class, parents if cached, and foundInDB bool.
func (c *Classifier) checkCacheOrDB(ctx context.Context, id string) (category string, parents []string, found bool) {
	hNode, err := c.store.GetHierarchy(ctx, id)
	if err == nil && hNode != nil {
		if hNode.Category != "" {
			return hNode.Category, hNode.Parents, true
		}
		return "", hNode.Parents, true
	}
	return "", nil, false
}

// fetchAndCacheLayer performs batch fetching and saving for a set of IDs.
func (c *Classifier) fetchAndCacheLayer(ctx context.Context, ids []string) map[string][]string {
	results := make(map[string][]string)
	batchClaims, batchLabels, err := c.client.GetEntityClaimsBatch(ctx, ids, "P279")
	if err != nil {
		slog.Error("Failed to batch fetch hierarchy", "error", err)
		return results
	}

	for id, parents := range batchClaims {
		lbl := batchLabels[id]

		// Resolve category immediately from parents to prevent cache poisoning
		category := ""
		if len(parents) == 0 {
			category = catDeadEnd
		}

		// 1. Scan for Match first (Priority)
		// Note: We deliberately do NOT check regional matches here!
		// fetchAndCacheLayer saves directly to the global database.
		// If we evaluate regional matches here, we pollute the global DB cache.
		for _, p := range parents {
			if cat, ok := c.lookup[p]; ok { // Use c.lookup directly to bypass regional
				category = cat
				break
			}
		}

		// 2. If no match, scan for Ignore
		if category == "" {
			for _, p := range parents {
				if _, ok := c.config.IgnoredCategories[p]; ok {
					category = catIgnored
					break
				}
			}
		}

		// Save to DB if we found a definite result (Match/Ignore) OR just to store the label
		if category != "" || lbl != "" {
			if err := c.store.SaveClassification(ctx, id, category, parents, lbl); err != nil {
				slog.Warn("Failed to save hierarchy node", "id", id, "error", err)
			}
		}
		results[id] = parents
	}
	return results
}

func (c *Classifier) finalizeMatch(ctx context.Context, qid, catName string, parents []string, label string, isRegional bool) (*model.ClassificationResult, error) {
	// Update DB with match, UNLESS it's a regional match to prevent global cache pollution.
	if !isRegional {
		if err := c.store.SaveClassification(ctx, qid, catName, parents, label); err != nil {
			return nil, fmt.Errorf("failed to save classification: %w", err)
		}
	} else {
		slog.Debug("Bypassed saving regional classification to DB", "qid", qid, "category", catName)
	}
	return c.resultFor(catName), nil
}

func (c *Classifier) resultFor(catName string) *model.ClassificationResult {
	cat, ok := c.config.Categories[catName]
	size := "M" // Default
	if ok {
		size = cat.Size
	}
	return &model.ClassificationResult{
		Category: catName,
		Size:     size,
	}
}

// finalizeIgnored saves ignored sentinel to DB and returns ignored result.
func (c *Classifier) finalizeIgnored(ctx context.Context, qid string, parents []string, label string) (*model.ClassificationResult, error) {
	// Save with regular sentinel to mark as ignored in cache
	if err := c.store.SaveClassification(ctx, qid, catIgnored, parents, label); err != nil {
		return nil, fmt.Errorf("failed to save ignored classification: %w", err)
	}
	return &model.ClassificationResult{Ignored: true}, nil
}

// No longer using individual fetch wrappers here as we call client directly or via batch metadata.

// FinalizeDimensions is a no-op kept for transition/interface compatibility.
func (c *Classifier) FinalizeDimensions() {}

// GetConfig returns the categories configuration.
func (c *Classifier) GetConfig() *config.CategoriesConfig {
	return c.config
}

// GetMultiplier is no longer used, returns 1.0.
func (c *Classifier) GetMultiplier(h, l, a float64) float64 {
	return 1.0
}

// AddRegionalCategories appends new regional categories to the existing lookup.
func (c *Classifier) AddRegionalCategories(categories map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.regionalCategories == nil {
		c.regionalCategories = make(config.CategoryLookup)
	}
	for qid, cat := range categories {
		c.regionalCategories[qid] = cat
	}
}

// ResetRegionalCategories clears all active regional categories.
func (c *Classifier) ResetRegionalCategories() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.regionalCategories = make(config.CategoryLookup)
}

// HasRegionalCategories returns true if any regional categories are active.
func (c *Classifier) HasRegionalCategories() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.regionalCategories) > 0
}

// GetRegionalCategories returns a copy of the currently active regional categories.
func (c *Classifier) GetRegionalCategories() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make(map[string]string)
	for k, v := range c.regionalCategories {
		res[k] = v
	}
	return res
}

func (c *Classifier) getLookupMatch(qid string) (category string, isRegional, ok bool) {
	// 1. Static Lookup
	if cat, ok := c.lookup[qid]; ok {
		return cat, false, true
	}

	// 2. Regional Categories
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.regionalCategories != nil {
		if cat, ok := c.regionalCategories[qid]; ok {
			return cat, true, true
		}
	}

	return "", false, false
}
