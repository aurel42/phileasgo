# Classification & Rescue Logic

This document explains how PhileasGo determines whether a Wikidata entity becomes a Point of Interest (POI) or is discarded.

## 1. Classification Flow

The Classifier determines an entity's category (e.g. "airport", "city", "mountain") based on its Wikidata `P31` (instance-of) claims and the `P279` (subclass-of) class hierarchy.

Source: `pkg/classifier/classifier.go`

### Single Entity: `Classify(qid)`

1. **Fast Lookup** (`getLookupMatch`): checks the QID against the static lookup map *and* the regional categories map (both O(1)). If found, returns the category immediately.
2. **Fetch Instances**: calls the Wikidata API for `P31` claims on the QID.
3. **Classify Instances**: iterates each P31 instance through `classifyHierarchyNode`. The **Match > Ignore** invariant applies: if any instance resolves to a positive category, that result is returned immediately, even if other instances resolve to `__IGNORED__`. An ignored result is kept as a fallback only if no match is found.

### Batch: `ClassifyBatch(entities)`

Same logic as `Classify`, but P31 instances come from pre-fetched metadata (SPARQL results or `seen_entities` cache) instead of live API calls. Each entity is processed independently; the Match > Ignore invariant applies per entity.

### Hierarchy Node Resolution: `classifyHierarchyNode(qid)`

Resolves a single taxonomy node (a class QID, not an article):

1. **Config Check** (`getLookupMatch`): is this QID mapped in `categories.yaml` or in regional categories? If yes, return result.
2. **DB Cache Check** (`GetClassification`): look up the `wikidata_hierarchy` table:
   - `__IGNORED__` → return `Ignored: true`
   - `__DEADEND__` → return `nil` (this node was fully traversed before and led nowhere)
   - Non-empty string → return that category
   - Empty string or not found → continue to slow path
3. **Slow Path** (`slowPathHierarchy` → `searchHierarchy`): BFS traversal of `P279` parents.

### BFS Slow Path: `searchHierarchy`

Layer-by-layer BFS traversal up the `P279` (subclass-of) graph, max depth **4**.

Each layer goes through:

1. **`scanLayerCache`**: for each node in the current queue, check the DB for a cached classification or cached parents. Nodes not found in DB are added to a `toFetch` list. Cached matches/ignores are tracked per-layer.
2. **`fetchAndCacheLayer`**: batch-fetch `P279` parents for all `toFetch` nodes from the Wikidata API. Each fetched node's parents are scanned for matches (priority) then ignores, and the result is saved to `wikidata_hierarchy`. Nodes with no parents are marked `__DEADEND__`.
3. **Layer Priority Check**: if any match was found in the layer (from cache or fetch), finalize the match immediately.
4. **`buildNextLayer`**: scan parents of all current-layer nodes. Match scan runs first across all parents; if found, return immediately. Then ignored scan runs. Unvisited parents that are neither matched nor ignored are added to the next queue.
5. **Ignore Resolution**: if the layer contained an ignored result but no match, `propagateIgnored` marks **all** traversed nodes (the entire BFS path) as `__IGNORED__` to short-circuit future traversals.

If the BFS exhausts all layers (depth 4) with no match or ignore, the starting node is marked `__DEADEND__`.

### Match > Ignore Priority

This is a core invariant enforced at every level:

- In `Classify` / `ClassifyBatch`: all P31 instances are checked; a positive match on *any* instance wins over ignored on another.
- In `slowPathHierarchy`: all direct P279 parents are scanned for matches first, ignores second.
- In `buildNextLayer`: match scan runs across the entire layer before the ignore scan.
- In `fetchAndCacheLayer`: match scan runs before ignore scan per node.

This prevents false negatives where an entity has one P31 instance leading to an ignored branch and another leading to a valid category.

### Regional Categories

Runtime-injected QID→Category mappings, used for location-specific classifications (e.g. "Japanese Castle" near Japan).

- **`AddRegionalCategories(map)`**: appends entries to the `regionalCategories` lookup (thread-safe via `sync.RWMutex`).
- **`ResetRegionalCategories()`**: clears all regional entries (called on teleport).
- **Lookup order** in `getLookupMatch`: static lookup first, then regional categories. Both are O(1) map lookups.

Regional categories are checked at every point where `getLookupMatch` is called: fast lookup, hierarchy node resolution, and parent scanning during BFS.

### Sentinel Values

The classifier uses two sentinel strings stored in `wikidata_hierarchy.category`:

| Sentinel | Meaning |
|---|---|
| `__IGNORED__` | This node (and its descendants) resolve to an ignored category. Future traversals return `Ignored: true` immediately. |
| `__DEADEND__` | This node was fully traversed (all P279 paths exhausted up to depth 4) with no match or ignore. Future traversals return `nil` immediately. |

An empty string means the node exists in the DB (structural cache with parents) but has not been classified yet.

## 2. The Filter Pipeline

Source: `pkg/wikidata/pipeline.go`, `pkg/wikidata/pipeline_filter.go`

### Entry Point: `ProcessTileData`

Raw SPARQL JSON → parsed articles → pipeline:

1. **Parse** SPARQL response into `[]Article` (streaming parser).
2. **Filter existing POIs** (`filterExistingPOIs`): drop articles whose QID already exists in the `poi` table.
3. **Filter seen entities** (`filterSeenArticles`): drop articles whose QID exists in `seen_entities` (skipped if `force` mode).
4. **`ProcessEntities`**: the core pipeline (see below).

### Core Pipeline: `ProcessEntities`

1. **Classification** (`classifyArticlesOnly`):
   - `collectUnclassifiedQIDs`: gather articles with no category yet.
   - `classifyInChunks`: build a metadata cache from two instance sources — `seen_entities` (DB, higher priority) and SPARQL P31 data — then run `ClassifyBatch`.
   - `runBatchClassification`: apply results — set `Category` for matches, set `Ignored: true` for ignored, and mark ignored entities as seen.
2. **Filter ignored** (`filterIgnoredArticles`): remove articles flagged `Ignored`.
3. **Post-process** (`postProcessArticles`): applies sitelinks gating and rescue:
   - **Sitelinks gating**: classified articles whose `Sitelinks` count is below the category's `sitelinks_min` threshold (from `categories.yaml`) are stripped of their category and demoted to rescue candidates.
   - Articles that pass sitelinks gating proceed directly.
   - Unclassified articles and sitelinks-demoted articles become rescue candidates.
4. **Rescue** (`rescueFromBatch`): see Section 3.
5. **Merge** (`MergeArticles`): deduplicate nearby POIs of the same category group using `merge_distance` from config.
6. **Hydrate**: fetch Wikipedia extracts, thumbnails, and multilingual names.
7. **Enrich & Save**: compute additional metadata and persist to DB.

## 3. Rescue Logic

Source: `pkg/rescue/service.go`

Entities that survive filtering but have no category (or were demoted by sitelinks gating) are candidates for "rescue" — promotion based on physical significance.

### How It Works: `rescue.Batch`

The rescue system uses a **neighborhood-median-based** approach rather than fixed thresholds:

1. **`AnalyzeTile`**: compute the local maximum height, length, and area from all candidate articles in the current tile.
2. **`CalculateMedian`**: compute the median of neighboring tiles' maxima (gathered from surrounding tiles within `radius_km`, default 20 km).
3. **Threshold per dimension**: `max(2 × median, configurable_minimum)`.
4. **Rescue check**: if the tile's local max for a dimension exceeds the threshold, the article holding that maximum is rescued.

### Configurable Minimums

From `configs/phileas.yaml` → `wikidata.rescue.promote_by_dimension`:

| Dimension | Config Key | Default |
|---|---|---|
| Height | `min_height` | 30 m |
| Length | `min_length` | 500 m |
| Area | `min_area` | 10,000 m² |

These minimums act as noise floors: even if the neighborhood median is very low (or zero), an article must exceed the minimum to be rescued.

### Rescue Output

- Rescued articles are assigned a **fallback category**: `"height"`, `"length"`, or `"area"` (lowercase, matching the dimension).
- Each rescued article receives a **`DimensionMultiplier`**: `local_max / median` (or 1.0 if median is zero). This multiplier quantifies how exceptional the article is relative to its neighborhood.
- Only one article is rescued per dimension per tile (the one holding the local maximum).
- An article is only rescued once even if it qualifies for multiple dimensions.

## 4. Caching Architecture

Classification uses a 4-layer cache to minimize redundant API calls and DB queries:

| Layer | Storage | Contents | Lifetime |
|---|---|---|---|
| **L1 — Static Lookup** | In-memory map | QID → Category from `categories.yaml` | Built once at startup, immutable |
| **L2 — Regional Categories** | In-memory map | QID → Category injected at runtime | Added via `AddRegionalCategories`, cleared on teleport via `ResetRegionalCategories` |
| **L3 — Classification Cache** | DB (`wikidata_hierarchy.category`) | Resolved category, `__IGNORED__`, or `__DEADEND__` | Persistent across sessions |
| **L4 — Structural Cache** | DB (`wikidata_hierarchy.parents`) | P279 parent QIDs for a node | Persistent; avoids re-fetching hierarchy from Wikidata API |

Lookup order: L1 → L2 → L3 → (L4 for structural data) → Wikidata API (slow path).
