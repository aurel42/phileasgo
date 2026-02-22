# Regional Categories & Config Redesign Plan

This document details the investigation into the current implementation of "Dynamic Config," its challenges, and a comprehensive, multi-phase redesign incorporating recent feedback.

## Investigation Findings: How "Dynamic Config" Currently Works

The "Dynamic Config" feature is currently implemented as a background scheduler job (`DynamicConfigJob` in `pkg/core/dynamic_config_job.go`).

1. **Triggers**: It runs silently in the background, firing on the first run and subsequently only when the aircraft has moved more than 50 nautical miles **AND** 30 minutes have passed since the last execution.
2. **Context Gathering**: Upon triggering, it gathers the aircraft's current location (Latitude, Longitude, Country, Region) and a list of existing static categories.
3. **LLM Invocation**: It sends this data to the `gemini-2.5-flash` model using the `dynamic_config` profile and the `context/wikidata.tmpl` prompt. The prompt asks the AI to identify 3-5 specific Wikidata subclasses (P279) of structures or landmarks that are unique or iconic to that specific region (e.g., "Shinto Shrine" in Japan, "Pyramid" in Egypt).
4. **Validation**: The AI returns a JSON list of class names. Because LLM-provided QIDs are untrustworthy, the `wikidata.Validator` searches Wikidata by name to resolve them back into validated QIDs.
5. **Application**: The validated QIDs (and their mapped parent categories) are pushed to the `Classifier` via `SetDynamicInterests`. This allows the application to recognize and score these highly-specific local POIs exactly as if they were part of the static `categories.yaml`.
6. **No Reprocessing**: Once the new interests are added to the classifier, existing tracked POIs in the area are NOT re-evaluated against the new rules (`// Reprocessing disabled per user request`).

> [!NOTE]
> Despite its name, "Dynamic Config" does not actually modify application configuration (like volume, simulation settings, or narration frequency). It strictly dynamically updates the **Wikidata classification rules**.

---

## What is Currently Broken / Challenging

1. **Invisible to the User**: The process is completely opaque. The user is never informed that regional categories are being searched or applied. If the AI adds a fascinating local feature, the user only finds out if they happen to stumble upon a relevant POI.
2. **Confusing & Unreliable Prompting**: The LLM frequently responds with specific POIs (e.g., "Eiffel Tower" instead of "Cast-iron Lattice Tower"), uninteresting generic categories, or categories we already track. The system then fails silently because names don't match or the validator returns weak matches. 
3. **Trigger Lock-in**: Hardcoded triggers (>50nm AND >30min) are rigid. If the LLM generates a poor response or fails, the user is stuck with standard configuration for a long time. There is no way to manually force a refresh.
4. **Misleading Nomenclature**: Calling the feature "Dynamic Config" is confusing since it exists entirely separate from the actual application configuration system (`pkg/config/provider.go`). It is effectively a "Contextual Interests" or "Regional Categories" system.
5. **Loss of Ignored POIs**: Currently, if the classifier doesn't know about a local category (say, "Shinto Shrine"), Wikidata POIs of that type are downloaded and aggressively discarded. Even if the LLM adds the category seconds later, those downloaded POIs are not reprocessed and are permanently lost for that session.
6. **No Spatial Persistence**: Every flight requires pinging the LLM again. If a user ferries across the same region, or if they fly over Tokyo for a week, they shouldn't need a Gemini call to figure out that Shrines are important in Japan again.

---

## Opportunities & Proposed Solutions

- **Rename the concepts**: Split the feature into **Regional Categories** (finding local subclasses of structures to enrich the map) and **Regional Config** (future expansion to alter frequency, text length, or style based on the environment).
- **Spatial Grid Persistence**: Store discovered categories in a spatial DB table (e.g., 1x1 degree bounding boxes). When entering a "tile," load categories from it and its immediate neighbors to pre-seed the classifier without an LLM call.
- **Lightweight Reprocessing**: We do **not** need to re-download tiles to pick up ignored POIs! Wikidata tiles download *everything*, and we filter in memory. We can iterate over the in-memory cache of `manager.trackedPOIs` and simply call `classifier.Classify(poi.QID)` again for any POI currently marked as `Ignored`. This is instantaneous.
- **Rigorous Prompt Testing Unit**: Before wiring the new prompt into the live loop, write a dedicated Go test file (`prompt_test.go`) that iterates a series of known test cases (e.g., Tokyo, Cairo, Paris, London) and asserts that the LLM only returns **generic** expected classes (like "shrine", "pyramid", "château") and NOT specific instances.

---

## Multi-Phase Redesign Plan

### Phase 1: Prompt Engineering & Testing Framework
- **Create an LLM Test Harness**: A new test file (e.g., `pkg/core/regional_prompt_test.go`) that runs `context/wikidata.tmpl` against real `gemini-2.5-flash` using a set of fixed coordinates (Japan, Egypt, France).
- **Refine `wikidata.tmpl`**: Explicitly forbid returning specific individual instances (e.g., say "DO NOT name individual monuments like 'Eiffel Tower', ONLY name the generic class like 'Lattice Tower'"). Force the LLM to output QIDs directly alongside names (to assist validation) and emphasize uniqueness. Iterate until tests consistently pass.

### Phase 2: Terminology Pivot & Core Refactoring
- Rename `DynamicConfigJob` to `RegionalContextJob`.
- Update the job to clearly differentiate between fetching **Regional Categories** and (eventually) **Regional Config**.
- Refactor `classifier.SetDynamicInterests` to `classifier.SetRegionalCategories`.

### Phase 3: Spatial Persistence (The Cache Grid)
- Extend the SQLite database with a new table: `regional_categories` (lat_grid INT, lon_grid INT, categories JSON).
- Modify the `RegionalContextJob` to define a "tile" (e.g., `math.Floor(lat)`).
- When the job triggers (or on spawn):
  1. Check the local DB for the current 1x1 degree tile and its 8 neighbors. 
  2. If categories exist in the local cache, inject them into the `Classifier` immediately.
  3. If the tile is "empty," queue an LLM query, and save the result back into the `regional_categories` table.

### Phase 4: Lightweight Reprocessing
- Add a new method to `poi.Manager`: `ReprocessIgnoredPOIs()`.
- After applying new Regional Categories from the LLM, loop over `pm.trackedPOIs`.
- If a POI is marked as `Ignored: true`, re-run `Classify`. If it now matches an expanded regional category, un-ignore it and assign its new category. 

### Phase 5: Expose to the EFB
- Add a new API endpoint `GET /api/context` to return currently active regional categories.
- Update the React frontend Dashboard to show a "Phileas' Focus" or "Local Interests" panel with chips representing active categories (e.g., `[Shinto Shrines] [Castles]`).
- Add a "Refresh Local Interests" button bypassing the time lock.

### Phase 6: True "Regional Config" (Future)
- Once the category discovery is stable, expand the prompt to suggest application configuration tweaks (e.g., `{"narrator_style": "Douglas Adams", "narration_frequency": 4}`) based on the region.
