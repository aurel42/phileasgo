# PhileasGo: System Architecture & Data Flows (v0.2.47)

This document provides a technical source of truth for the core logic of PhileasGo as of version **v0.2.47**.

---

## 1. Wikidata Tile Pipeline
Converts flight telemetry into Points of Interest.

### Trigger: The Tick
- **Frequency**: Every 1 second (`service.go`).
- **Telemetry**: Fetched from SimConnect: `Latitude`, `Longitude`, `Heading`, `AltitudeAGL`, `IsOnGround`.

### Flow Breakdown (Step-by-Step)
1. **Request Verification**: `Service` asks `Scheduler` if it's time to fetch tiles for the current (Lat, Lon, Heading).
2. **Grid Resolution**: `Scheduler` queries `Grid (H3)` to get the center cell and all neighboring cells within the Field of View (FOV).
3. **Tile Selection**: `Scheduler` returns a list of candidate H3 Tile Indexes to the `Service`.
4. **Tile Processing Loop** (for each Tile):
   - **Cache Lookup**: `Service` checks the `SQLite Cache` for the TileID.
   - **Radius Check**: `Service` calculates the tile radius (center to vertex).
   - **Network Fallback**: If a cache miss occurs, `Service` sends a POST SPARQL query to the `Wikidata API`, receives JSON data, and saves it to the cache.
   - **ProcessTileData**:
     - Filter out POIs that already exist or have been "Seen".
     - Batch Classify items through the inheritance hierarchy.
     - Deduplicate items based on Wikipedia articles.
     - Hydrate remaining items with Wikipedia Titles and Article Lengths.
     - Final Deduplication based on refined metadata.
     - Save unique POIs to the Database.

---

## 2. Classification & Rescue
How the system determines if a Wikidata item is worthy of being a POI.

### Phase 1: Direct Matching (The Fast Path)
Before any network calls, the system checks the `categories.yaml` configuration for direct hits:
1. **Static Lookup**: If the QID matches a known specific landmark or a "Root Category" QID (e.g., Q62447 for Aerodrome).
2. **Dynamic Interests (AI Extensions)**: Regional subclasses (P279) suggested by Gemini based on location, mapping specialized QIDs to either existing or new "Dynamic" categories.

### Phase 2: Taxonomy Traversal (The BFS Engine)
If no direct match exists, the system starts a **layered Breadth-First Search (BFS)** starting from the item's **P31 (Instance Of)** claims.

1. **Depth Limit**: The traversal stops at **4 layers** deep to prevent performance degradation.
2. **Structural Caching**: Every node (class) encountered is saved in the `wikidata_hierarchy` table.
    - **Full Node**: Contains Category, Parents (P279), and Name.
    - **Structural Node**: Saved with an empty Category (`""`) to indicate it's a pass-through node that has been visited but doesn't map directly to a category.
    - **Ignored Node**: Marked specifically to block sub-trees of the taxonomy.
3. **Traversal Rules**:
    - **Batching**: Missing nodes in a layer are fetched as a single batch from the Wikidata API.
    - **Priority**: Within each layer, a `Match` for a category is prioritized over an `Ignore` signal.
    - **Persistence**: Results are committed to the SQLite DB to ensure that second visits to the same category (e.g., different types of "Castle") are instant.

### Phase 3: Landmark Rescue (Dimension Tracking)
Items that fail to classify into a known category are eligible for **Rescue** if they are geometrically significant.

1. **Eligibility**: An article is **never** rescued if any of its direct P31 instances are explicitly in the `IgnoredCategories` config.
2. **The Dimensions**: The system tracks `Height` (P2048), `Length` (P2043), and `Area` (P2046). These are **straight-fetched** from SPARQL as raw floats; no secondary calculation (like Length * Width) or unit conversion is performed by the server.
3. **Median Window**: The `DimensionTracker` maintains a sliding window of the **Max Dimensions** from the last **10 tiles**.
4. **Rescue Thresholds**:
    - **Local Hero**: If an item's dimension is the **Maximum** in the current tile, it is rescued as a "Landmark".
    - **Global Giant**: If an item exceeds the **Median of the Maxima** from the 10-tile window, it is rescued.
5. **Scoring Impact**:
    - **Record/Giant Boost**: Rescued landmarks (or extremely large classified POIs) receive a `DimensionMultiplier`:
        - `x2.0`: If it's a Tile Record **OR** exceeds the Global Median.
        - `x4.0`: If it is **BOTH** a Tile Record and a Global Giant.

---

## 3. Hydration & Language Selection
How we determine the POI's Name and Wikipedia link.

### Logic (v0.2.47)
1. **Multi-Point Language Detection**: The system samples the **Country** at the **Tile Center** AND all **6 Corner Vertices** of the H3 hex.
2. **Mapper Lookup**: It resolves this set of countries to **ALL their Official Languages** (ISO codes) via a cached Wikidata mapping. This builds a deduplicated, prioritized list of regional languages.
3. **Length Fetching**: Article character counts are fetched from Wikipedia for:
    - **All Regional Languages** detected in the tile (e.g., `de`, `fr` if on a border).
    - **English** article (`en`).
    - **User Language** article (from config).
4. **Selection Logic** (`determineBestArticle`):
    - The system compares the character counts of all candidates.
    - **Tie-Breaker**: If lengths are similar, the system prefers languages in the order they were detected (Center > Vertices).
    - **Winner**: The longest article among the candidates becomes the primary source.
    - **Fallback**: If no lengths match, weights are: User > English > Local > Wikidata.

---

## 4. Spatial Deduplication (The Merger)
How the system prevents clustering and "POI soup" by merging nearby items.

### Two-Stage Merging
PhileasGo performs deduplication at two different points in the ingest flow for efficiency:

1. **Stage 1: Pre-Hydration (`MergeArticles`)**:
    - **Where**: `processAndHydrate` (before fetching Wikipedia titles/lengths).
    - **Purpose**: Optimization. By merging items early using **Wikidata Sitelinks** as a proxy for quality, the system avoids thousands of redundant API calls to Wikipedia for POIs that would ultimately be merged anyway.
2. **Stage 2: Post-Hydration (`MergePOIs`)**:
    - **Where**: `enrichAndSave` (after fetching full metadata).
    - **Purpose**: Final Polish. Uses actual **Article Lengths** to ensure the highest-quality POI remains as the "survivor" and "gobbles" the scores of merged neighbors.

### Merge Rules
- **Distance**: The required distance between POIs depends on their **Category** (e.g., small distance for statues, large distance for cities).
- **Quality**: The POI with the higher **Article Length** (or Sitelink count in Stage 1) survives and inherits the proximity of its neighbors.
- **Category Sizes**: Defined in `categories.yaml` (S, M, L, XL).

---

## 5. LanguageMapper & Country Detection
The component responsible for resolving geographic coordinates to human languages.

### Operation
- **Cold Start**: On service start, it attempts to load the mapping from the persistence store (`sys_lang_map_v4`).
- **Wikidata Sync**: if the cache is empty, it executes a broad SPARQL query across all countries to fetch:
    - **ISO Country Code** (P297).
    - **Official Languages** (P37) and their **Wikimedia Codes** (P424).
- **Mapping**: It maintains a `map[string][]LanguageInfo`, supporting countries with multiple official languages (e.g., Switzerland, Canada).
- **Refresh**: The map is intended to be refreshed monthly (`refreshInterval = 30 days`).

### Runtime Resolution
1. **Reverse Geocode**: `geo.Service` finds the nearest city (from `cities1000.txt`) to get the country code.
2. **Mapper Lookup**: `LanguageMapper.GetLanguages(cc)` returns all official language codes.
3. **Fallback**: If a country is not in the map, it defaults to **English** (`en`).

---

## 6. Narration Selection & LOS
The automated loop that triggers narration.

### Logic: `NarrationJob` (`scheduler.go`)
1. **Cooldown**: A randomized timer (`CooldownMin` to `CooldownMax`). Starts counting only **after** the previous narration has finished playing.
2. **Candidate Selection**: Hits `poiMgr.GetCandidates()` to get all active POIs (sorted by score).
3. **Line-of-Sight (LOS)**:
    - If `Terrain.LineOfSight` is enabled, the job iterates through candidates starting from the highest score.
    - It performs a 3D ray-check between aircraft and POI using **ETOPO1** elevation data.
    - **Tolerance**: 50m vertical offset (grazing-ray buffer).
    - **Selection**: The first POI with valid LOS and `Score > MinScoreThreshold` (0.01) is selected.
4. **Essay Fallback**:
    - If no POIs have LOS, or the aircraft is above 2000ft AGL, the system may trigger a "Regional Essay" on a general topic (Geography, Aviation, History).

---

## 7. Dynamic Categories & AI Extensions
How PhileasGo adapts its taxonomy to the current region.

### The Problem
A static list of categories (Castles, Airports, etc.) cannot capture the cultural or geological richness of every region on Earth (e.g., "Moai" in Easter Island or "Shinto Shrines" in Japan).

### The Solution: `DynamicConfigJob`
Every **30 minutes** or **50nm** of travel, the system triggers a background task:
1. **Context Batching**: The system sends the current Country, Region, and a list of all **Static Categories** to Gemini.
2. **AI Suggestion**: Gemini suggests 3-5 Wikidata **subclasses** (Classes, not Instances) that are iconic to that specific region.
3. **Taxonomy Mapping**:
    - Gemini attempts to map each suggestion to a **Static Category** (e.g., a "Moorish Castle" -> "Castle").
    - If no static category fits, Gemini provides a **Specific Category** name (e.g., "Buddhist Temple"). This becomes a **Dynamic Category**.
4. **Validation**: The suggested QIDs are validated via a SPARQL metadata check to ensure they are valid classes.
5. **Injection**: Validated QIDs are injected into the `Classifier` as **Dynamic Interests**.

### Impact on Flows
- **Classification**: Phase 1 (Direct Matching) now hits the `Dynamic Interests` map first.
- **Scoring**: Since Dynamic Categories aren't in `categories.yaml`, they inherit **Default Weights** (1.0) and **Medium Size** ("M").
- **Narration**: The narrator uses the `specific_category` name in her descriptions, providing a much higher level of localized "Tour Guide" expertise.
- **Persistence**: These interests are **In-Memory only**. They expire when the flight moves to a new region or the server restarts, ensuring the "Interest Window" remains relevant to the current geography.

---

---

## 8. POI Narration Workflow (The AI Path)
Technical orchestration from discovery to playback.

### The Entry Points
1. **Manual Selection**:
   - **Step 1**: User clicks a POI marker on the Map or a POI in the Sidebar list. This opens the `POIInfoPanel` and displays metadata/thumbnails.
   - **Step 2**: User clicks the **Play (▶)** button in the `POIInfoPanel`. This sends a `POST /api/narrator/play` request to `NarratorHandler.HandlePlay`, which triggers `AIService.PlayPOI(manual=true)`.
2. **Automated Selection**: The `NarrationJob` background loop periodically identifies high-scoring visible candidates and triggers `AIService.PlayPOI(manual=false)`.

### Orchestration Flow (`AIService.narratePOI`)
*This workflow executes in a dedicated goroutine to ensure the main simulation and telemetry loops remain responsive.*

1. **Concurrency & State Control**: `AIService` immediately acquires a mutex lock (`s.mu`) and sets `active=true` and `generating=true`. This "state lock" is critical; it prevents overlapping narrations if multiple selection triggers (e.g., manual and auto) occur simultaneously.
2. **Immediate Tactical Feedback**: Before entering the "slow" AI generation phase, the `BeaconService` spawns a marker balloon at the POI ground coordinates. This provides the user with an instant visual confirmation of the selection.
3. **Environmental Context Gathering**: The service assembles `NarrationPromptData`, which acts as the "world-view" for the LLM. It includes current and predicted telemetry, POI metadata (name, category, distance), and relevant Wikipedia extracts from the local cache.
4. **Script Generation**: The `LLM Provider` (Gemini) generates the narrative script using the `narrator/script.tmpl` template. This prompt governs the persona's tone, language, and measurement units.
5. **Session Memory Integration**: As soon as the script is generated, it is passed to the `tripSummary` update process. This asynchronously merges the new story into the rolling "Short-Term Memory," ensuring that subsequent narrations have a cohesive sense of the journey's history.
6. **Audio Synthesis**: The script is sent to the active `TTS Provider`. The synthesis process (e.g., Azure or Edge-TTS) produces an MP3/WAV file. In the case of a "Fatal" error (like an API rate limit), the system handles the fallback retries here.
7. **Latency Calibration**: The system measures the total time from Selection start to Synthesis completion. This **observed latency** is crucial; it is used to update the `sim.SetPredictionWindow`, ensuring that the spatial cues (like "at your 10 o'clock") generated in the next script will be accurate for when the pilot actually hears the audio.
8. **Audio Playback**: The `AudioService` begins playing the synthesized file. The `generating` flag is released, but the `active` flag remains held to protect the listener's focus.
9. **Finalization & Release**: A background Ticker polls `Audio.IsBusy()`. Once the audio finishes, the `active` flag is set to `false`, the current POI is cleared from the UI state, and the system becomes available for the next narration loop.

### Key Mechanisms

#### 1. Predictive Navigation Logic (`calculateNavInstruction`)
To ensure directional cues (e.g., "At your 3 o'clock") remain accurate when the pilot actually *hears* them, the system performs a self-correcting transformation:
- **Dynamic Window**: The system measures the actual **Selection-to-Audio** latency for every narration. It maintains a rolling average of these durations and uses it to set the `sim.SetPredictionWindow`.
- **Latency-Aware Source**: Instead of using current coordinates, the prompt engine uses **Predicted Telemetry** calculated by projecting the aircraft's current vector ahead by the **observed average latency**. This ensures directional cues are synchronized with the moment of playback.
- **The 4.5km Boundary**:
    - **Proximity Mode (< 4.5km)**: The system suppresses distance metrics and cardinal directions. It uses pure relative sectors ("On your left", "Straight ahead") to keep the narration immersive.
    - **Distant Mode (>= 4.5km)**: The system includes precise distances ("about 10 miles away").
- **Ground vs. Air**:
    - **Airborne**: Uses the **Clock Face** (e.g., "at your 2 o'clock") for distant targets.
    - **On Ground**: Uses **Cardinal Directions** (e.g., "to the North-East") unless in extreme proximity, where it remains silent to avoid logical errors while taxiing.

#### 2. Relative Dominance & Skew Strategy (`DetermineSkewStrategy`)
Narration length is not purely random; it is governed by **Competition Density**:
- **Lone Wolf**: If a POI has no "Rivals" (other POIs with >50% of its score) nearby, it uses **StrategyMaxSkew**. The system generates a pool of 3 random word counts and picks the **highest**, allowing for a deep, leisurely narration.
- **High Competition**: If Rivals exist, it uses **StrategyMinSkew** to pick the **lowest** of 3 random counts. This forces brevity, preventing the narrator from talking through the next discovery.
- **Dynamic Window**: The "Rival" check is performed every time a script is generated, ensuring the pacing adapts to the landscape.

#### 3. Spatial & Chronological Context
- **Short-Term Memory (v0.2.50)**: The system maintains a rolling **Trip Summary** in session memory.
    - **Session Evolution**: After each narration, a background task uses the `summary` model profile to merge the previous summary with the latest script.
    - **Chronological Density**: The summary is strictly chronological, consolidation ensures it remains rich with detail (specific names, dates, facts) but concise (max 300 words).
    - **Narrative Continuity**: The prompt engine injects this summary into every new script/essay prompt, instructing the LLM to use "what we saw earlier" to bridge stories and avoid repetition.
- **Flight Stage Persona**: The `FlightStage` variable (Taxi, Takeoff, Cruise, Descent, Landing) is injected into the prompt, allowing the narration tone to shift (e.g., more concise during high-workload takeoff).
- **Wikipedia Persistence**: Article extracts are cached in the local SQLite `articles` table, keyed by Wikidata QID, to bypass Wikipedia API rate limits during repeated flights over the same area.

#### 4. Visual Feedback & Safety
- **Marker Spawning**: The marker balloon (Beacon) is spawned *before* LLM generation to provide instant interaction feedback.
- **Deduplication**: `LastPlayed` is persisted to the DB only *after* successful TTS synthesis, ensuring a POI isn't "consumed" if the API calls fail.
- **TTS Fallback**: If Azure TTS fails with a 429/5xx, the system switches to `edge-tts` for the remainder of the session.

---

---

## 9. Marker Beacons (Visual Guidance)
The system used to visually highlight POIs in the 3D world.

### Lifecycle
1. **Spawn**: Triggered by `AIService.PlayPOI`. Beacons spawn *immediately* (before LLM/TTS) to provide tactical feedback.
2. **Smooth Updates**: The `Beacon.Service` runs an independent SimConnect connection at `PERIOD_VISUAL_FRAME` (~20Hz to 60Hz), bypassing the main telemetry heartbeat for jerky-free movement.
3. **Formation**:
    - **Target**: A "Hot Air Balloon" is spawned at the POI ground coordinates.
    - **Guide**: If enabled, additional balloons spawn in a formation between the aircraft and the target at a configured distance (`FormationDistanceKm`).
    - **Dissolve**: As the aircraft approaches the target, the guiding formation balloons despawn (at 1.5x formation distance) to clear the pilot's view for the final orbit.

### 3D Safety Logic (The "Altitude Floor")
- **Eye-Level Sync**: Beacons dynamically match the aircraft's **MSL Altitude** while in flight to ensure they remain at the pilot's eye level.
- **The Floor**: When the aircraft descends below the `AltitudeFloorFt` threshold (e.g., 2000ft AGL), the beacons **lock their altitude**.
- **Impact Prevention**: This "Lock" prevents the balloons from following the plane all the way to the ground, maintaining a realistic "aerial marker" appearance and preventing clipping into terrain or buildings.

---

## 10. The Prompt Engine (Context Orchestration)
How technical context is translated into the Tour Guide persona.

### Data Aggregation (`buildPromptData`)
The prompt sent to the LLM is a complex JSON object containing:
- **Telemetry**: Current and Predicted (1-min) Lat/Lon, Heading, Altitude, and Ground Speed.
- **Regional Profile**: Detected Country, nearest City, and official languages.
- **Flight Stage**: Custom tags for `Taxi`, `Takeoff`, `Cruise`, `Descent`, or `Landing`.
- **Wikipedia Extract**: Full article text from the persistent SQLite cache.

### Linguistic Control
- **Units Instruction**: The system explicitly tells the LLM whether to use Metric (km/m) or Imperial (miles/ft) based on user configuration.
- **TTS SSML Templates**: Different LLM templates are used depending on the TTS provider (Azure vs. Edge-TTS) to ensure correct SSML tag usage for pauses and emphasis.
- **Cross-Reference Memory**: The prompt includes the names of POIs narrated in the last 60 minutes (`fetchRecentContext`), allowing the LLM to generate more cohesive, "memory-aware" tour scripts.

---

## 11. The Prompt Template System (Orchestration)

The system uses a robust Go `text/template` based engine (`pkg/llm/prompts`) to construct the complex instructions sent to Gemini. This system ensures narrations remain fresh and variable through randomized logic and modular macros.

### Custom Logic Functions
The Prompt Manager registers several custom functions accessible within any `.tmpl` file:

- **`{{maybe <percent> <text>}}`**: Randomly includes text based on a percentage (0-100). Used to vary persona traits and speech patterns between narrations.
- **`{{pick "A|||B|||C"}}`**: Randomly selects one option from a pipe-separated string. Ensures concluding phrases remain diverse.
- **`{{interests <list>}}`**: Shuffles and thins the user's interest list (dropping ~2 topics) to force the AI to rotate its focus.
- **`{{category <name> <data>}}`**: Dynamically executes a sub-template from `configs/prompts/category/`. Allows for expert knowledge injection for specific POI types (e.g., "Aerodromes", "Cities").

### Core Macros (`common/macros.tmpl`)
Macros standardize the agent's persona and context regardless of the specific narration type.

| Macro | Purpose | Content |
| :--- | :--- | :--- |
| `{{template "persona" .}}` | Branding | Establishes a competent, fascinated tour guide persona. |
| `{{template "style" .}}` | Tone Control | Enforces natural speech patterns (contractions, fillers) and conversational pace. |
| `{{template "flight_data" .}}` | Telemetry | Injects current MSL, AGL, Groundspeed, Heading, and Predicted Position. |

### Template Hierarchy
Templates are organized in `configs/prompts/` by their functional role:
- **`narrator/script.tmpl`**: Primary POI narration instructions.
- **`narrator/essay.tmpl`**: Broad regional essay instructions.
- **`units/`**: Localization of measurement terms.
- **`tts/`**: Provider-specific SSML and formatting tweaks.

---

## 13. Regional Essay Workflow
Broad narrative tours triggered to provide context when specific POIs are sparse, such as during high-altitude cruise.

### Flow Logic (`AIService.PlayEssay`)
1. **Intelligent Trigger**: The `NarrationJob` triggers a regional essay only if no high-scoring POIs have Line-of-Sight, or if the aircraft is sustained above 2000ft AGL where wide-area context is more valuable than individual landmarks.
2. **Topic Selection**: The `EssayHandler` uses a weighted selection algorithm to pick a relevant topic (Geography, Aviation History, or Regional Culture). It checks cooldowns and historical usage to ensure the tour doesn't become repetitive.
3. **Narrative Orchestration (`narrateEssay`)**:
   - **Visual Discipline**: Unlike POI flows, essays clear existing beacons. This signals to the user that the narrator is shifting from a "Point-and-Describe" mode to a broader "Historical Lecture" mode.
   - **Prompt Engineering**: The service renders `narrator/essay.tmpl`, providing the LLM with the broad regional context of the current flight path.
   - **Dynamic Metadata**: If the generated script includes a `TITLE:` prefix, the `AIService` parses this and updates the UI header in real-time, providing a visual anchor for the essay's theme.
   - **Continuous Memory**: The essay script is added to the `tripSummary`. This ensures that if the flight later encounters a related POI, the AI can reference the "earlier lecture."
   - **Finalization**: Similar to the POI flow, the essay maintains an `active` lock until the narration audio is complete.

---

## 14. TTS Fallback & State Persistence
Mechanisms to ensure that the audio experience remains stable even during network congestion or API failures.

### Failure Recovery (`handleTTSError`)
1. **Detection**: The system distinguishes between "Soft" errors (temporary glitches) and "Fatal" errors (Rate Limits, Account Exhaustion, or persistent 5xx errors).
2. **The Pivot**: If a fatal error is detected in the primary provider (e.g., Azure), `AIService.activateFallback()` is called. This switches the session's active provider to `Edge-TTS` (a free, highly reliable local/edge fallback).
3. **Non-Destructive Release**: When an error occurs, the POI is **released** (the `LastPlayed` timestamp is NOT updated). This allows the narration job to immediately re-try the same POI with the fallback provider, ensuring the user doesn't "lose" a high-quality discovery due to a network error.

### UI Synchronization (State Ticker)
Because narration generation and playback occur in background goroutines, the service maintains UI consistency through a polling mechanism:
- A **100ms Ticker** monitors the `Audio.IsBusy()` state. 
- The `active` flag (which drives the "PLAYING" indicator in the UI) is only released when the audio hardware reports the clip is finished.
- This ensures the UI accurately reflects the narrator's activity, even during long essays.

### Narrative Replay (`ReplayLast`)
Allows users to re-hear the previous stop's information without re-triggering the AI generation cost:
- The `AudioService` maintains the previous MP3 file in a temporary buffer.
- When `ReplayLast` is called, the service temporarily restores the `currentPOI` or `currentEssayTitle` to the UI.
- This creates a seamless "Instant Replay" experience where the visuals and metrics re-synchronize with the playback audio.

---

## 15. Final Verification Checklist
Files to audit against this document:
- [ ] **H3 Resolution**: `pkg/wikidata/grid.go` (`h3Resolution`).
- [ ] **Classification Path**: `pkg/classifier/classifier.go` (`Classify`).
- [ ] **Spatial Merging**: `pkg/wikidata/merger.go`.
- [ ] **Language Mapping**: `pkg/wikidata/mapper.go` (SPARQL query in `refresh`).
- [ ] **Article Winner**: `pkg/wikidata/service_enrich.go` (`determineBestArticle`).
- [ ] **LanguageMapper Persistence**: Document the storage and refresh mechanism for country-to-language mappings.
- [ ] **Selection Loop**: `pkg/core/scheduler.go` (`getVisibleCandidate`).
- [x] **Narration Workflow**: `pkg/narrator/service_ai_poi.go` (`narratePOI`).
- [x] **Essay Workflow**: `pkg/narrator/service_ai_essay.go` (`narrateEssay`).
- [x] **Beacon Logic**: `pkg/beacon/service.go` (`updateStep`).
- [x] **Prompt Logic**: `pkg/narrator/service_ai_data.go` (`buildPromptData`).
- [x] **Prompt Templates**: `configs/prompts/` (logic and macro consistency).
