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
        *   If `""` (Empty): Returns `nil` (Unknown). These are POIs that have neither a positive nor a negative match in our configured categories. We will later decide whether to keep or discard them based on their dimensions.
    *   *No* -> Proceed to Step 3.
3.  **Graph Traversal**:
    *   Fetch `P31` (Instance of).
    *   For each instance, fetch its parents (`P279`).
    *   Traverse up the tree (BFS, max depth 4).
    *   **Matching**:
        *   If a parent matches a category in `categories.yaml`, we assign the category to the POI. We save the category for all parent QIDs in this branch in the `wikidata_hierarchy` table.
        *   If a parent matches the `ignored_categories` list, we mark it and its branch as `__IGNORED__` in the `wikidata_hierarchy` table.
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
1.  **Not Ignored**: Up to this point, we haven't explicitly ignored the entity. This means it's not in `categories.yaml`'s ignore list and not in the `wikidata_hierarchy` table as `__IGNORED__`.
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
