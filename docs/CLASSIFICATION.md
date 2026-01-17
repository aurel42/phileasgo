# Classification & Rescue Logic

This document explains how Phileas determines whether a Wikidata entity becomes a Point of Interest (POI) or is discarded.

## 1. Classification Flow (`Classifier.Classify`)

The Classifier determines the nature of an entity (e.g., "Airport", "City", "Mountain") based on its Wikidata instance (`P31`) and the class hierarchy (`P279`).

### The Lookup Process
1.  **Fast Config Check**: Is the QID itself explicitly mapped in `categories.yaml`?
    *   *Yes* -> Returns the Category.
2.  **DB Cache Check**: Do we have a cached classification for this QID?
    *   *Yes* -> Return it.
        *   If `__IGNORED__`: Returns `Ignored: true`.
        *   If `""` (Empty): Returns `nil` (Unknown). **(Historical Note: This was the source of the persistent bug. Old empty cache entries prevented re-evaluation.)**
    *   *No* -> Proceed to Step 3.
3.  **Graph Traversal**:
    *   Fetch `P31` (Instance of).
    *   For each instance, fetch its parents (`P279`).
    *   Traverse up the tree (BFS, max depth 4).
    *   **Matching**:
        *   If a parent matches a category in `categories.yaml`, we lock it in.
        *   If a parent matches the `ignored_categories` list, we mark it as `__IGNORED__`.
    *   **Saving**: The result is saved to `wikidata_hierarchy`.

## 2. The Filter Pipeline (`Service.ProcessTileData`)

When new data arrives from Wikidata (SPARQL), it goes through a strict pipeline:

1.  **EXISTING CHECK**: If `wikidata_id` exists in `poi` table -> **DROP**.
2.  **SEEN CHECK**: If QID is in `seen_entities` -> **DROP** (unless forced).
3.  **CLASSIFICATION**:
    *   Run `ClassifyBatch`.
    *   If result is `Ignored: true` -> **DROP** (and mark as seen).
    *   If result is `Category: "City"` (etc.) -> **KEEP**.
    *   If result is `nil` (Unknown) -> **KEEP** (Potentially for Rescue).

## 3. The "Rescue" Logic (`DimClassifier.ShouldRescue`)

Entities that survive the filter but have **no category** (Unknown) are candidates for "Rescue" if they are physically significant (Large Area, Long, or High).

### Condition
Code: `pkg/classifier/classifier.go` -> `ShouldRescue`

An entity is rescued if:
1.  **Not Ignored**: Its direct instances are NOT in `categories.yaml`'s ignore list.
    *   *Critique*: This check is shallow (config only). However, if the database cache is clean, the upstream `Classification` step would have already caught and filtered any hierarchical ignores.
2.  **Dimensions**:
    *   Height > 100m, OR
    *   Length > 500m, OR
    *   Area > 0.5 km²

### Outcome
If rescued, the entity is assigned a fallback category:
*   `"Area"`
*   `"Height"`
*   `"Length"`
*   `"Landmark"` (Default)

## Root Cause of the "Open Wound"
The persistence of unwanted POIs (like Administrative Regions) was due to **Cache Poisoning** combined with the flow:
1.  In older versions, ignored items were cached as `""` (Unknown) instead of `__IGNORED__`.
2.  `ClassifyBatch` returned `nil` (Unknown) for these items.
3.  The Ignore Filter (Step 3) let them pass.
4.  The Rescue Logic (Step 4) saw a large entity (e.g., a region) with "Unknown" category.
5.  It checked the shallow ignore list (Step 4.1), found no direct match (since the ignoring happens deep in the hierarchy), and **Rescued** it.

**The Fix**:
*   **v0.2.99**: Validated that new ignores are saved as `__IGNORED__`.
*   **Operational**: Dropping the `wikidata_hierarchy` table removes the `""` entries, forcing re-classification which will correctly resolve to `__IGNORED__`.
