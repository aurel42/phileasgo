# PhileasGo: System Architecture & Data Flows (v0.2.64)

This document provides a technical source of truth for the core logic of PhileasGo as of version **v0.2.64**.

---

## Wikidata Tile Pipeline
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

## Classification & Rescue
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

### Phase 4: Icon Assignment ("Heal on Load")
Since some categories (like "Rescued" internal categories) do not exist in the public `categories.yaml` config, and legacy data may have missing icons, the system acts as a final gatekeeper during **Ingestion/Loading**:

1.  **Gatekeeper**: `POIManager.upsertInternal` calls `ensureIcon(m)` on every POI before it enters the active tracking list.
2.  **Lookup Logic**:
    -   **Config Check**: Attempts to look up the category in `categories.yaml` (case-insensitive normalization).
    -   **Internal Fallback**: If the config check fails, it checks for hardcoded "Internal Categories" (e.g., `Length` -> `arrow`, `Area` -> `circle-stroked`).
3.  **Result**: This ensures that all POIs served to the API have a valid `Icon` property, healing any legacy or config-missing issues automatically on server start.

---

## Hydration & Language Selection
How we determine the POI's Name and Wikipedia link.

### Logic (v0.2.58)
1. **Upfront Language Resolution**: Before fetching any text, the system computes the authorized language set for the tile:
    - **English** (`en`)
    - **User Language** (from config)
    - **Regional Languages**: Derived from the Country at the **Tile Center** AND all **6 Corner Vertices** of the H3 hex (deduplicated via `LanguageMapper`).
2. **Strict Source Filtering**: The authorized set is converted to a Wikidata `sitefilter` (e.g., `enwiki|dewiki|frwiki`) and passed to the `wbgetentities` API.
    - **Efficiency**: The API returns sitelinks *only* for the requested languages, preventing "wiki pollution" (e.g., Russian articles for German POIs).
    - **Zero Waste**: Unwanted languages are never fetched or stored.
3. **Length Fetching**: Article character counts are fetched from Wikipedia only for the survivors of the filter (the intersection of Available Sitelinks and Authorized Languages).
4. **Selection Logic** (`determineBestArticle`):
    - The system compares the character counts of all candidates.
    - **Tie-Breaker**: If lengths are similar, the system prefers languages in the order they were detected (Center > Vertices).
    - **Winner**: The longest article among the candidates becomes the primary source.
    - **Nameless Filter**: If the candidate ends up with **no valid names** (User, English, or Local) after hydration/filtering, it is strictly **dropped**. This prevents "Unknown" entities (often caused by source-filtering removing all sitelinks) from entering the system.
    - **Deterministic Fallback**: If no length data is available, the system selects the first available title based on priority: User > English > Local (sorted). Random iteration is strictly forbidden.

---

## Spatial Deduplication (The Merger)
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

## LanguageMapper & Country Detection
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

## Scheduler Architecture & Narration Selection
The central heartbeat that drives all periodic tasks and triggers narration.

### The Tick Loop (`pkg/core/scheduler.go`)
The `Scheduler` runs a **100ms ticker** (configurable via `ticker.telemetry_loop`) that:
1. Broadcasts sim state to the API (UI updates)
2. Fetches telemetry from SimConnect
3. Broadcasts telemetry to the `TelemetrySink` (API)
4. Checks for teleport (see [Session Reset (Teleport Detection)](#session-reset-teleport-detection))
5. Evaluates all registered `Job` instances

### Job Types

| Job Type | Trigger | Example Usage |
|----------|---------|---------------|
| `NarrationJob` | Complex logic (see below) | Auto-narration of POIs |
| `DistanceJob` | Distance traveled > threshold | Wikidata tile fetching (5km) |
| `TimeJob` | Time elapsed > threshold | POI eviction check (5min) |

All jobs implement the `Job` interface:
```go
type Job interface {
    Name() string
    ShouldFire(t *sim.Telemetry) bool
    Run(ctx context.Context, t *sim.Telemetry)
}
```

### NarrationJob Logic (`narration_job.go`)
The `NarrationJob` is the most complex job, with multiple pre-conditions:

#### Pre-Conditions (`checkPreConditions`)
- **AutoNarrate**: Must be enabled in config
- **Location Consistency**: Scorer's last position must be within 10km of current position (prevents stale scores after teleport)
- **Sim State**: Must be `StateActive` (not paused, in menus, or loading)
- **Ground Proximity (v0.2.64)**: Replaced legacy distance checks with centralized category filtering. If on ground, the system only considers POIs in the `Aerodrome` category.

#### Narrator Activity (`checkNarratorReady`)
The narrator has three activity states:

| State | Meaning | ShouldFire? |
|-------|---------|-------------|
| `IsPlaying()` | Audio is playing | ❌ Block, set `wasBusy=true` |
| `IsPaused()` | User paused playback | ❌ Block |
| `IsActive()` | Generating script/TTS | ❌ Block |
| None | Idle | ✅ Proceed |

When `wasBusy` transitions from true→false (playback finished), the cooldown timer **resets to now**.

#### Cooldown (`checkCooldown`)
- **Formula**: Random value between `CooldownMin` and `CooldownMax` (config)
- **Clock Start**: Timer starts when **playback finishes**, not when generation starts
- **Skip**: User can bypass via `narrator.ShouldSkipCooldown()`

#### POI Selection (`hasViablePOI`)
Returns true if `GetBestCandidate().Score >= MinScoreThreshold` (default: 0.5)

> [!NOTE]
> **Update (v0.2.62)**: The selection logic now uses a centralized `isPlayable(poi)` check which relies solely on the `LastPlayed` timestamp and the `RepeatTTL` configured in the narrator settings. All "Zombie Rules" (like RecentlyPlayed or 5-minute visual persistence) have been removed in favor of this single source of truth for freshness.

#### Essay Eligibility (`checkEssayEligible`)
If no viable POI, essays are considered with these rules:

| Rule | Condition |
|------|-----------|
| **Enabled** | `narrator.essay.enabled = true` |
| **Essay Cooldown** | `time.Since(lastEssayTime) >= essay.cooldown` (default: 10min) |
| **Silence Rule** | `time.Since(lastNarration) >= 2 × CooldownMax` |
| **Altitude** | `AltitudeAGL >= 2000ft` |

#### Line-of-Sight (LOS) Selection
- **Enabled via**: `terrain.line_of_sight: true`
- **Data**: ETOPO1 elevation grid (1 arc-minute resolution)
- **Tolerance**: 50m vertical buffer (grazing-ray forgiveness)
- **Iteration**: Candidates checked in score order; first with LOS wins

### Lone Wolf Detection (`pkg/narrator/skew.go`)
Determines narration length based on POI competition:

**Threshold**: `math.Max(heroScore * 0.2, 0.5)`

| Rivals Found | Strategy | Effect |
|--------------|----------|--------|
| > 1 | `min_skew` | Short narration (pick smallest of 3 random lengths) |
| ≤ 1 | `max_skew` | Long narration (pick largest of 3 random lengths) |

---

## Session Reset (Teleport Detection)
How the system handles large position jumps caused by teleport, map changes, or flight loading.

### Detection: `Scheduler.tick()`
1. **Position Tracking**: The `Scheduler` tracks `lastTickPos` (the aircraft position from the previous tick).
2. **Distance Check**: On each tick, if the distance between `lastTickPos` and the current position exceeds `cfg.Sim.TeleportThreshold` (default: 80km), a teleport is detected.
3. **State Protection**: Detection only occurs when `sim.GetState() == Active` to avoid false positives during loading screens.

### Reset Protocol
When a teleport is detected, the `Scheduler` iterates through all registered `SessionResettable` components and calls `ResetSession(ctx)`:

| Component | Reset Behavior |
|-----------|----------------|
| `narrator.AIService` | Clears `tripSummary`, `narratedCount`, and replay state. The "Short-Term Memory" is wiped for a fresh narrative start. |
| `poi.Manager` | Clears the `trackedPOIs` in-memory cache and resets `lastScoredLat/Lon`. The `seen` filter (DB-backed `LastPlayed`) is **NOT** cleared. |
| `DynamicConfigJob` | Resets `lastMajorPos`, `lastRunTime`, and `firstRun`. This forces a fresh AI context poll for the new region. |

### Registration
Components are registered in `main.go` via `scheduler.AddResettable(component)` during initialization.

---

## POI Scoring & Visibility
How the system ranks POIs for narration selection based on distance, size, and content quality.

> [!NOTE]
> **Update (v0.2.62)**: Scoring and Filtering are now decoupled. The `Scorer` provides a **Pure Quality Score** based on geographic and content factors. Temporal filtering (for both display and narration) is handled by the `POIManager` and `NarrationJob`.

### The Visibility Table (`configs/visibility.yaml`)
Defines **maximum visible distances** (in nautical miles) for each size category at different altitudes. The table is interpolated at runtime.

| Altitude (ft) | S (Small) | M (Medium) | L (Large) | XL (Extra Large) |
|---------------|-----------|------------|-----------|------------------|
| 0             | 0.5       | 1.0        | 3.0       | 4.0              |
| 1000          | 2.0       | 5.0        | 5.0       | 10.0             |
| 3500          | 1.5       | 7.5        | 14.5      | 22.0             |
| 7000          | 0.8       | 10.0       | 23.0      | 35.0             |
| 10000         | 0.0       | 2.0        | 26.0      | 44.0             |

**Design Rationale**: Small POIs (monuments, lighthouses) become invisible at high altitudes, while large geographic features (mountains, cities) remain visible from cruise altitude.

### Scoring Formula (`pkg/scorer/scorer.go`, `pkg/visibility/calculator.go`)
The final score is computed as a product of multipliers:

```
FinalScore = VisibilityScore × SizePenalty × DimensionMultiplier × ContentScore × VarietyMultiplier
```

#### Visibility Score (Distance Decay)
```go
visScore = 1 - (distanceNM / maxDistanceNM)
```
- **Linear Decay**: Score decreases linearly from 1.0 (at POI location) to 0.0 (at max distance).
-   **Size-Dependent**: The `maxDistanceNM` is looked up from the visibility table based on POI size and aircraft altitude.
-   **Effective AGL / Valley Boost**:
    -   Logic: `FinalVisibilityScore = Max(Score(RealAGL), Score(EffectiveAGL))`
    -   Effective AGL = `AircraftAltMSL - LowestValleyFloorMSL` (within dynamic radius).
    -   **Dynamic Radius**: The scan radius is determined by the maximum visible distance for an **XL** POI at the aircraft's current altitude (e.g., higher altitude = wider valley scan).
    -   Scanned using efficient `terrain.GetLowestElevation`.
-   **Invisible Cutoff**: If `distance > maxDistance`, the POI is marked invisible (`Score = 0`).

#### Size Penalty (Bias Correction)
Applied after visibility score to reduce the advantage of distant large POIs:

| Size | Penalty |
|------|---------|
| S    | 1.0 (none) |
| M    | 1.0 (none) |
| L    | 0.85 |
| XL   | 0.70 |

**Purpose**: Without this penalty, XL POIs at 10nm would outscore nearby S/M POIs due to their slower distance decay rate.

#### Dimension Multiplier (Landmark Bonus)
Applied to geometrically significant POIs identified by the Rescue system:
- **Tile Record OR Global Giant**: `x2.0`
- **Both**: `x4.0`

#### Content Score
Product of several quality factors:

| Factor | Formula | Notes |
|--------|---------|-------|
| **Article Length** | `sqrt(chars / 500)` | Capped at minimum 1.0 |
| **Sitelinks** | `1 + sqrt(max(0, sitelinks - 1))` | Cities/Towns capped at 4 sitelinks |
| **Category Weight** | From `categories.yaml` | e.g., Monument=1.3, Railway=0.4 |
| **MSFS POI** | `x4.0` | Photogrammetry landmarks |

#### Variety Multiplier
Discourages repetitive category selection:
- **Novelty Boost**: `x1.3` if category not in recent history.
- **Variety Penalty**: `x0.1` to `x0.5` if same category was recently narrated (sliding scale).
- **Group Penalty**: Additional penalty if category belongs to same group (e.g., "Settlements") as the last narration.

### Bearing Multipliers (Airborne Only)
POIs in the pilot's field of view receive scoring bonuses. Relative bearing is normalized to [-180°, 180°] then converted to [0°, 360°] for sector lookup:

| Sector (rb360) | Multiplier | Notes |
|----------------|------------|-------|
| 0° - 90° (Right Front) | x1.0 | Baseline |
| 90° - 225° (Rear) | x0.0 | Invisible behind aircraft |
| 225° - 270° (Left Rear) | x0.5 | Limited visibility |
| 270° - 300° (Left Side) | x1.5 | Good side window view |
| 300° - 330° (Left Front) | x2.0 | Best visibility from cockpit |
| 330° - 360° (Forward Left) | x1.5 | Forward visibility |

> [!NOTE]
> Sector boundaries use `<` operator (exclusive upper bound). For example, exactly 270° falls into the Left Side sector, not Left Rear.

### Blind Spot Detection
POIs directly beneath the aircraft (under the nose) are penalized:

**Algorithm** (`pkg/visibility/calculator.go`):
```go
blindRadius := 1.0 * math.Min((altAGL - 50.0) / 4950.0, 1.0)
if altAGL < 500 {
    blindRadius = 0.1
}
// Blind spot if: distNM < blindRadius AND abs(relBearing) < 90°
```

| Altitude (AGL) | Blind Radius (nm) |
|----------------|-------------------|
| < 500ft | 0.1 |
| 500ft | 0.09 |
| 2500ft | 0.49 |
| 5000ft | 1.0 (max) |
| 10000ft+ | 1.0 (capped) |

**Penalty**: `x0.1` if POI is within blind radius and within ±90° of heading.

### Example Score Calculation
*Castle (M, weight=1.2) at 2nm, aircraft at 3500ft, heading 090°, POI bearing 300° (left front)*

1. **Max Distance** (M @ 3500ft): 7.5nm
2. **Visibility Score**: `1 - 2/7.5 = 0.73`
3. **Size Penalty** (M): `1.0`
4. **Bearing Multiplier** (left front): `x2.0`
5. **Category Weight**: `x1.2`
6. **Novelty Boost**: `x1.3`

**Final Score**: `0.73 × 1.0 × 2.0 × 1.2 × 1.3 = 2.28`

---

## POI List Filtering
To provide a consistent and responsive experience, PhileasGo centralizes all POI filtering in `POIManager`. The manager exposes two distinct access methods tailored to different consumers (UI vs. Backend), ensuring that strict rules (like cooldowns) don't hide visibility in the map.

### Frontend: `GetPOIsForUI`
Designed for the visual map and list.
- **Goal**: Providing situational awareness and history.
- **Input**: `filterMode` (fixed/adaptive), `targetCount`, `minScore`.
- **Output**: A unified list of POIs including:
    -   **Played Items**: `LastPlayed != zero`. These are **always** included (rendered as blue markers) regardless of visibility or score, ensuring travel history is preserved.
    -   **Visible Candidates**: Unplayed items that meet the visibility and score criteria.
-   **Logic**:
    -   **Ground Agnostic**: Does **NOT** apply the Aerodrome-only filter. This ensures that even when taxiing, the user can see nearby landmarks on the map.
    -   **Thresholding**: Calculates an `EffectiveThreshold` based on the active `Filter Mode`.
        -   **Fixed Mode**: Threshold = `Narrator.MinPOIScore` (default: 0.5).
        -   **Adaptive Mode**: Dynamically calculated to include at least `TargetPOICount` visible POIs. (If ties exist at the cutoff score, all tied POIs are included).

### Backend: `GetNarrationCandidates`
Designed for the `NarrationJob` and `GetBestCandidate`.
- **Goal**: Finding the next valid target to speak about.
- **Output**: A strictly filtered list of viable targets.
- **Logic** (Strict Filtering):
    -   **Strictly Fresh (`isPlayable`)**: Filters out **ANY** POI that is currently on cooldown (where `time.Since(LastPlayed) < RepeatTTL`). Does not return "Played" items.
    -   **Strictly Visible**: `IsVisible` must be true (determined by Visibility Table/LOS).
    -   **Ground Aware**: If `IsOnGround` is true, strictly restricts results to the `Aerodrome` category to prevent "shadowing" of airports by nearby city centers.
    -   **Score Gated**: If a score threshold is provided, candidates below it are dropped.

### `GetBestCandidate`
- **Logic**: A wrapper around `GetNarrationCandidates` with `limit=1`.
- **Behavior**: Returns the absolute best *playable* and *visible* POI. By delegating to the unified backend filter, it strictly prevents the narrator from stalling on a high-scoring but recently-played POI.

---

---

## Dynamic Categories & AI Extensions
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

## POI Narration Workflow (The AI Path)
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

#### Predictive Navigation Logic (`calculateNavInstruction`)
To ensure directional cues (e.g., "At your 3 o'clock") remain accurate when the pilot actually *hears* them, the system performs a self-correcting transformation:
- **Dynamic Window**: The system measures the actual **Selection-to-Audio** latency for every narration. It maintains a rolling average of these durations and uses it to set the `sim.SetPredictionWindow`.
- **Latency-Aware Source**: Instead of using current coordinates, the prompt engine uses **Predicted Telemetry** calculated by projecting the aircraft's current vector ahead by the **observed average latency**. This ensures directional cues are synchronized with the moment of playback.
- **The 4.5km Boundary**:
    - **Proximity Mode (< 4.5km)**: The system suppresses distance metrics and cardinal directions. It uses pure relative sectors ("On your left", "Straight ahead") to keep the narration immersive.
    - **Distant Mode (>= 4.5km)**: The system includes precise distances ("about 10 miles away").
- **Ground vs. Air**:
    - **Airborne**: Uses the **Clock Face** (e.g., "at your 2 o'clock") for distant targets.
    - **On Ground**: Uses **Cardinal Directions** (e.g., "to the North-East") unless in extreme proximity, where it remains silent to avoid logical errors while taxiing.

#### Relative Dominance & Skew Strategy (`DetermineSkewStrategy`)
Narration length is not purely random; it is governed by **Competition Density**:
- **Lone Wolf**: If a POI has no "Rivals" (other POIs with >50% of its score) nearby, it uses **StrategyMaxSkew**. The system generates a pool of 3 random word counts and picks the **highest**, allowing for a deep, leisurely narration.
- **High Competition**: If Rivals exist, it uses **StrategyMinSkew** to pick the **lowest** of 3 random counts. This forces brevity, preventing the narrator from talking through the next discovery.
- **Dynamic Window**: The "Rival" check is performed every time a script is generated, ensuring the pacing adapts to the landscape.

#### Spatial & Chronological Context
- **Short-Term Memory (v0.2.50)**: The system maintains a rolling **Trip Summary** in session memory.
    - **Session Evolution**: After each narration, a background task uses the `summary` model profile to merge the previous summary with the latest script.
    - **Chronological Density**: The summary is strictly chronological, consolidation ensures it remains rich with detail (specific names, dates, facts) but concise (max 300 words).
    - **Narrative Continuity**: The prompt engine injects this summary into every new script/essay prompt, instructing the LLM to use "what we saw earlier" to bridge stories and avoid repetition.
- **Flight Stage Persona**: The `FlightStage` variable (Taxi, Takeoff, Cruise, Descent, Landing) is injected into the prompt, allowing the narration tone to shift (e.g., more concise during high-workload takeoff).
- **Wikipedia Persistence**: Article extracts are cached in the local SQLite `articles` table, keyed by Wikidata QID, to bypass Wikipedia API rate limits during repeated flights over the same area.

#### Visual Feedback & Safety
- **Marker Spawning**: The marker balloon (Beacon) is spawned *before* LLM generation to provide instant interaction feedback.
- **Deduplication**: `LastPlayed` is persisted to the DB only *after* successful TTS synthesis, ensuring a POI isn't "consumed" if the API calls fail.
- **TTS Fallback**: If Azure TTS fails with a 429/5xx, the system switches to `edge-tts` for the remainder of the session.

---

---

## Marker Beacons (Visual Guidance)
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
- **The Floor**: When the aircraft descends below the `AltitudeFloor` threshold (default: 2000ft, configurable as any distance unit), the beacons **lock their altitude**.
- **Impact Prevention**: This "Lock" prevents the balloons from following the plane all the way to the ground, maintaining a realistic "aerial marker" appearance and preventing clipping into terrain or buildings.

---

## The Prompt Engine (Context Orchestration)
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

## The Prompt Template System (Orchestration)

The system uses a robust Go `text/template` based engine (`pkg/llm/prompts`) to construct the complex instructions sent to Gemini. This system ensures narrations remain fresh and variable through randomized logic and modular macros.

### Custom Logic Functions
The Prompt Manager registers several custom functions accessible within any `.tmpl` file:

- **`{{maybe <percent> <text>}}`**: Randomly includes text based on a percentage (0-100). Used to vary persona traits and speech patterns between narrations.
- **`{{pick "A|||B|||C"}}`**: Randomly selects one option from a pipe-separated string. Ensures concluding phrases remain diverse.
- **`{{interests <list>}}`**: Shuffles and thins the user's interest list (dropping ~2 topics) to force the AI to rotate its focus.
- **`{{category <name> <data>}}`**: Dynamically executes a sub-template from `configs/prompts/category/`. Allows for expert knowledge injection for specific POI types (e.g., "Aerodromes", "Cities").

### Common Templates (`common/`)

Shared templates standardize the agent's persona and context regardless of the specific narration type.

| Template | Purpose | Content |
| :--- | :--- | :--- |
| `{{template "Identity" .}}` | Branding | Establishes a competent, fascinated tour guide persona. |
| `{{template "Voice" .}}` | Tone Control | Enforces natural speech patterns (contractions, fillers) and conversational pace. |
| `{{template "Constraints" .}}` | Boundaries | Enforces what NOT to say (no clichés, no Spock-like precision). |
| `{{template "Situation" .}}` | Telemetry | Injects current MSL, AGL, Groundspeed, Heading, and Predicted Position. |

### Template Hierarchy
Templates are organized in `configs/prompts/` by their functional role:
- **`narrator/script.tmpl`**: Primary POI narration instructions.
- **`narrator/essay.tmpl`**: Broad regional essay instructions.
- **`units/`**: Localization of measurement terms.
- **`tts/`**: Provider-specific SSML and formatting tweaks.

---

## Regional Essay Workflow
Broad narrative tours triggered to provide context when specific POIs are sparse, such as during high-altitude cruise.

### Flow Logic (`AIService.PlayEssay`)
1. **Intelligent Trigger (Strict Gating)**: The `NarrationJob` enforces a strict hierarchy to ensure essays acts as "gap fillers":
    - **Priority**: Essays are only considered if **NO** viable POI candidates (Score > Threshold) are available.
    - **Silence Prerequisite**: The narrator must have been silent for at least **2x Max Cooldown** (e.g., 60s) to prevent overcrowding the timeline.
    - **Dedicated Cooldown**: Once an essay plays, a separate `Essay.Cooldown` (e.g., 10m) prevents another essay from triggering, ensuring variety between specific POI facts and broad regional context.
    - **Altitude Check**: The aircraft must be consistently above **2000ft AGL**.
2. **Topic Selection**: The `EssayHandler` uses a weighted selection algorithm to pick a relevant topic (Geography, Aviation History, or Regional Culture). It checks cooldowns and historical usage to ensure the tour doesn't become repetitive.
3. **Narrative Orchestration (`narrateEssay`)**:
   - **Visual Discipline**: Unlike POI flows, essays clear existing beacons. This signals to the user that the narrator is shifting from a "Point-and-Describe" mode to a broader "Historical Lecture" mode.
   - **Prompt Engineering**: The service renders `narrator/essay.tmpl`, providing the LLM with the broad regional context of the current flight path.
   - **Dynamic Metadata**: If the generated script includes a `TITLE:` prefix, the `AIService` parses this and updates the UI header in real-time, providing a visual anchor for the essay's theme.
   - **Continuous Memory**: The essay script is added to the `tripSummary`. This ensures that if the flight later encounters a related POI, the AI can reference the "earlier lecture."
   - **Finalization**: Similar to the POI flow, the essay maintains an `active` lock until the narration audio is complete.

---

## TTS Fallback & State Persistence
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

## Final Verification Checklist
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
