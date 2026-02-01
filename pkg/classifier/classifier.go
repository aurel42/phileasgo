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
	store         store.HierarchyStore
	client        WikidataClient // Interface for testability
	config        *config.CategoriesConfig
	lookup        config.CategoryLookup
	tracker       *tracker.Tracker
	dynamicLookup config.CategoryLookup
	mu            sync.RWMutex
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
	if catName, ok := c.getLookupMatch(qid); ok {
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
	Category string
	Size     string
	Ignored  bool
	Reason   string
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
				return &ExplanationResult{
					Category: res.Category,
					Size:     res.Size,
					Reason:   fmt.Sprintf("Matched via instance %s", inst),
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
			Ignored: true,
			Reason:  fmt.Sprintf("Ignored via instance %s", bestInst),
		}, nil
	}

	return &ExplanationResult{Reason: fmt.Sprintf("Traversed %d instances, no category match found", len(targets))}, nil
}

// ClassifyBatch determines categories for a batch of articles (by QID) using pre-fetched metadata.
func (c *Classifier) ClassifyBatch(ctx context.Context, entities map[string]wikidata.EntityMetadata) map[string]*model.ClassificationResult {
	results := make(map[string]*model.ClassificationResult)

	for qid, meta := range entities {
		// 1. Config Check
		if catName, ok := c.getLookupMatch(qid); ok {
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
	if catName, ok := c.getLookupMatch(qid); ok {
		return c.resultFor(catName), nil
	}

	// 2. Fast Path: DB Cache
	storedCat, found, err := c.store.GetClassification(ctx, qid)
	if err == nil && found {
		if storedCat == catIgnored {
			// Cached as explicitly ignored
			return &model.ClassificationResult{Ignored: true}, nil
		}
		if storedCat == catDeadEnd {
			// Cached as a Dead End
			return nil, nil
		}
		if storedCat != "" {
			return c.resultFor(storedCat), nil
		}

		// Continue if completely empty (legacy or intermediate)
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
			if hNode.Category == catIgnored {
				return &model.ClassificationResult{Ignored: true}, nil
			}
			if hNode.Category == catDeadEnd {
				return nil, nil
			}
			return c.resultFor(hNode.Category), nil
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
	// Immediate check for category match or ignored
	// Prioritize MATCH over IGNORE for direct parents

	// 1. Check ALL parents for a match first
	for _, sub := range subclasses {
		if catName, ok := c.getLookupMatch(sub); ok {
			return c.finalizeMatch(ctx, qid, catName, subclasses, label)
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
		toFetch := make([]string, 0, len(queue))
		parentsFromCache := make(map[string][]string)

		// 1. Filter: Determine what to fetch vs use from cache
		for _, id := range queue {
			match, parents, foundInDB := c.checkCacheOrDB(ctx, id)
			if foundInDB {
				if match != "" {
					if match == catIgnored {
						// Propagate ignored to all traversed nodes
						c.propagateIgnored(ctx, allTraversed)
						return c.finalizeIgnored(ctx, qid, subclasses, label)
					}
					if match == catDeadEnd {
						// Dead end reached in branch, move to next queue item
						continue
					}
					return c.finalizeMatch(ctx, qid, match, subclasses, label)
				}
				parentsFromCache[id] = parents
			} else {
				toFetch = append(toFetch, id)
			}
		}

		// 2. Fetch: Batch request for missing nodes
		if len(toFetch) > 0 {
			fetchedParents := c.fetchAndCacheLayer(ctx, toFetch)
			for id, parents := range fetchedParents {
				parentsFromCache[id] = parents
			}
		}

		// 3. Build Next Layer & Check Hits
		nextQueue, matchedCat, ignoredCat := c.buildNextLayer(queue, parentsFromCache, visited)
		if matchedCat != "" {
			return c.finalizeMatch(ctx, qid, matchedCat, subclasses, label)
		}
		if ignoredCat != "" {
			// Propagate ignored to all traversed nodes in the BFS path
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
// Returns nextQueue, matchedCategory (if found), and ignoredCategory (if found).
func (c *Classifier) buildNextLayer(queue []string, parentsFromCache map[string][]string, visited map[string]bool) (nextQueue []string, matchedCat, ignoredCat string) {
	nextQueue = make([]string, 0)
	var foundIgnored string

	// 1. Scan ALL parents for a match first
	for _, id := range queue {
		parents := parentsFromCache[id]
		for _, p := range parents {
			// Check for matching category
			if catName, ok := c.getLookupMatch(p); ok {
				return nil, catName, ""
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
		return nil, "", foundIgnored
	}

	return nextQueue, "", ""
}

// checkCacheOrDB checks if a node is already classified or cached in DB.
// Returns matchedCategory if found as a class, parents if cached, and foundInDB bool.
func (c *Classifier) checkCacheOrDB(ctx context.Context, id string) (category string, parents []string, found bool) {
	hNode, err := c.store.GetHierarchy(ctx, id)
	if err == nil && hNode != nil {
		if hNode.Category != "" {
			return hNode.Category, nil, true
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
		for _, p := range parents {
			if cat, ok := c.getLookupMatch(p); ok {
				category = cat
				break
			}
			// FIX: Check if parent is ignored to prevent "cache poisoning" (saved as "")
			if _, ok := c.config.IgnoredCategories[p]; ok {
				category = "__IGNORED__"
				// Don't break immediately? If another parent is a MATCH, we prefer MATCH.
				// But we established earlier in the loop order that MATCH > IGNORE.
				// The lookup match above covers MATCH.
				// So if we find IGNORE here, we can set it, but we should continue checking for MATCH?
				// Actually, `getLookupMatch` check above handles the MATCH case first for the current parent `p`.
				// If p is ignored, we set category. But what if a later parent `p2` is a MATCH?
				// The logic in slowPathHierarchy checks ALL parents for match first.
				// Here we just want to save *some* valid state.
				// If we find an ignored parent, we should probably record it, UNLESS we find a match later?
				// Let's iterate all parents for MATCH first, then for IGNORE.
			}
		}

		// 1. Scan for Match first (Priority)
		for _, p := range parents {
			if cat, ok := c.getLookupMatch(p); ok {
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

		// Save to DB ONLY if we found a definite result (Match or Ignore)
		// Intermediate nodes remain uncached to prevent "poisoning" the hierarchy.
		if category != "" {
			if err := c.store.SaveClassification(ctx, id, category, parents, lbl); err != nil {
				slog.Warn("Failed to save hierarchy node", "id", id, "error", err)
			}
		}
		results[id] = parents
	}
	return results
}

func (c *Classifier) finalizeMatch(ctx context.Context, qid, catName string, parents []string, label string) (*model.ClassificationResult, error) {
	// Update DB with match
	if err := c.store.SaveClassification(ctx, qid, catName, parents, label); err != nil {
		return nil, fmt.Errorf("failed to save classification: %w", err)
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

// SetDynamicInterests updates the dynamic QID interests.
func (c *Classifier) SetDynamicInterests(interests map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dynamicLookup = interests
}

func (c *Classifier) getLookupMatch(qid string) (string, bool) {
	// 1. Static Lookup
	if cat, ok := c.lookup[qid]; ok {
		return cat, true
	}

	// 2. Dynamic Lookup
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.dynamicLookup != nil {
		if cat, ok := c.dynamicLookup[qid]; ok {
			return cat, true
		}
	}

	return "", false
}
