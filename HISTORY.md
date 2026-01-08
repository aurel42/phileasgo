# Release History

## v0.2.33 (2026-01-08)
- **Regression Fix**: **Scheduler Runs with Invalid Telemetry**
    - Fixed issue where Wikidata and POI Scoring services would run their ticker loops when the simulator was disconnected or inactive, causing requests to "Null Island" (0,0 coordinates).
    - Added `GetState()` to `SimStateProvider` interface.
    - Both `pkg/wikidata/service.go` and `pkg/poi/manager.go` now guard on `sim.StateActive` before processing.
- **Feature**: **Thumbnail Map Image Filter**
    - Extended `isVectorGraphic()` to also filter out map images (`_map.` or `_map_` in filename).

## v0.2.32 (2026-01-08)
- **Fix**: **Thumbnail Appears Immediately When Panel Opens**
    - POIInfoPanel now receives `pois` array as prop, enabling real-time lookup of fresh POI data.
    - Removed broken queryClient approach (pois wasn't in React Query cache).

## v0.2.31 (2026-01-08)
- **Fix**: **Thumbnail Immediate Display**
    - POI Info Panel now watches for thumbnail updates in the POI cache, triggering re-render when data arrives.
- **Feature**: **Improved Vector Graphic Filtering**
    - Added `isVectorGraphic()` helper to detect `.svg`, `.svg.png`, and `.gif` patterns.
    - If `pageimages` returns a vector graphic, falls back to first non-vector content image.

## v0.2.30 (2026-01-08)
- **Feature**: **Thumbnail Fallback for Articles Without Page Image**
    - When Wikipedia `pageimages` API returns no thumbnail, now falls back to the first non-SVG/GIF content image.
    - Added `getFirstContentImage()` and `getImageURL()` helpers to `pkg/wikipedia/client.go`.
- **Feature**: **POI Info Panel Thumbnail Sync**
    - Panel now checks React Query cache for fresh POI data before making API calls for thumbnails.

## v0.2.29 (2026-01-08)
- **Fix**: **Azure TTS 400 Errors (Nested SSML Tags)**
    - Resolved issue where LLM-generated `<speak>` and `<voice>` tags caused Azure to reject the request (HTTP 400).
    - Added `repairSSML` stripping logic to remove nested wrapper tags before SSML construction.
    - Added prompt constraint (rule 4) in `azure.tmpl` to prevent LLM from outputting these tags.
- **Fix**: **Predicted Position for Stationary Aircraft**
    - Corrected telemetry logic to return current position (instead of 0,0) when aircraft speed is 0.
- **Prompt**: **Removed Hesitation Padding Rule** from Azure TTS template for cleaner output.

## v0.2.28 (2026-01-08)
- **Configuration**: **Defaults Overhaul**
    - Updated default values across LLM profiles, logging, Wikidata parameters, and narrator settings for production-ready deployments.
    - Migrated from deprecated `gemini-2.0-flash` to `gemini-2.5-flash-lite`.
- **Configuration**: **Improved Generated Config File**
    - Added documentation header, inline `# Options:` comments for enum fields, and locale examples.
- **Fix**: **Wikipedia/Wikidata API Stats Tracking**
    - Resolved bug where API call statistics showed 0 in the UI due to a provider key mismatch between cache and API tracking.

## v0.2.27 (2026-01-08)
- **Refactor**: **Best Local Language Selection**
    - Removed arbitrary "Rescue" logic. Instead, the system now fetches Wikipedia titles for *all* relevant local languages (e.g. at borders).
    - It retrieves article lengths for all candidates and selects the language with the **longest** Wikipedia article as the definitive "Local Name" for the POI.
    - Improved strictness: Articles are only ingested if they exist in English, the User's language, or one of the detected local languages.
    - **Fix**: Reduced cyclomatic complexity in article selection logic.

## v0.2.26 (2026-01-08)
- **Feature**: **Multi-Language Sampling for Borders**
    - Implemented a more robust language detection strategy that samples not just the center of a tile, but all 6 corners.
    - If a tile overlaps a border (e.g., Germany/Poland), the system now queries Wikidata for articles in *all* detected languages (e.g., `en`, `de`, `pl`).
    - This ensures correct POI names are found even for exclaves or border regions where the center point might miss the relevant local language.
    - Reverted the "Any Wikipedia" filter in favor of this strict but expanded language set to maintain data quality.

## v0.2.25 (2026-01-08)
- **Fix**: **Blacklist Low-Quality Wikis**
    - Explicitly excluded `rowiki` (Romanian), `cewiki` (Cebuano), and `warwiki` (Waray) from name rescue sources.
    - Prevents mass-imported bot stubs (often minimal geographic entries) from being used as POI names when high-quality English or Local articles are missing.
    - Ensures POI names are derived from human-curated content.

## v0.2.24 (2026-01-08)
- **Feature**: **"Any Wikipedia Article" Pre-Filter**
    - Optimized SPARQL query to use `FILTER(?sitelinks > 0)` instead of strict language matching.
    - Dropped 90% of raw Wikidata items early, while allowing items with *any* valid sitelink to proceed to classification.
- **Fix**: **Strict Rescue Name Filtering**
    - Implemented strict filtering in the "Rescue" phase to reject names starting with `Category:`, `File:`, `Template:`, etc.
    - Explicitly excluded `commonswiki`, `wikidatawiki`, and `wikiquote` from being used as name sources.
    - Prevents "Category:Naturdenkmal..." and similar administrative titles from appearing as POI names.

## v0.2.23 (2026-01-08)
- **Fix**: **Wikipedia API 414 Error (URI Too Long)**
    - Switched `GetArticleLengths` requests from GET to POST.

## v0.2.22 (2026-01-08)
- **Refactor**: **Integer Precision for Geodata Cache**
    - Changed `cache_geodata` schema to store radius in integer meters (`radius_m`) instead of floating-point kilometers.
    - Eliminates floating-point drift and simplifies frontend rendering logic.
- **Fix**: **Strict Name Selection**
    - Enforced strict "Wikipedia Article Title" requirement for POIs.
    - Removed fallback to Wikidata Labels, preventing raw Wikitext or internal identifiers from appearing as POI names.
    - Improved "Rescue" logic to strictly prioritize Sitelinks (Wikipedia URLs) over Labels.
    - Verified edge cases (Exclaves, Belgium) with new table-driven tests.
- **Testing**: **H3 Coverage**
    - Added comprehensive table-driven tests for H3 Grid and Scheduler logic.

## v0.2.21 (2026-01-08)
- **Architecture**: **Migration to Uber H3**
    - Completely replaced the custom axial hexagonal grid implementation with **Uber H3** (Hierarchical Hexagonal Geospatial Indexing System).
    - Standardized on **Resolution 5** (~8.5km edge length) for Wikidata tile caching.
    - Updated `HexTile` implementation to use standard H3 indices (string) instead of custom Row/Col coordinates.
    - **Breaking Change**: Cache keys migrated from `wd_hex_` to `wd_h3_`. Old cache entries will be ignored/pruned.
- **Feature**: **Dynamic Tile Geometry**
    - The system now computes the exact circumradius of each H3 tile (plus a 50m buffer) for SPARQL queries.
    - Added `cache_geodata` table to store specific query radius meta-data for visualization.
    - Updated `CacheLayer` to visualize the actual true-geometry radius of cached tiles on the map.
- **Fix**: **Narrator Stability**
    - Resolved race condition where playing a missing or evicted POI could cause a nil pointer dereference in the workflow.
- **Build**: **H3 CGO Requirement**
    - Added requirement for a C Compiler (MinGW/GCC) to build the project due to the new H3 library dependency (CGO).

## v0.2.20
- **Refactor**: **Narrator Service Architecture**
    - Decomposed the monolithic `service_ai.go` into focused files: `service_ai_workflow.go` (orchestration), `service_ai_data.go` (data fetching), and `service_ai_logic.go` (navigation calculations).
    - Improves maintainability, testability, and adheres to idiomatic Go practices.
- **Feature**: **Simplified Navigation Logic (4.5km Threshold)**
    - Introduced a universal 4.5 km threshold for navigation instructions.
    - **Distance < 4.5 km**: Distance is omitted. Ground: silence. Airborne: relative direction ("Straight ahead", "On your left").
    - **Distance >= 4.5 km**: Distance is stated. Ground: cardinal direction ("To the North"). Airborne: clock position ("At your 12 o'clock").
- **Refactor**: **Naming Conventions**
    - Renamed `WikiProvider` → `WikipediaProvider` and `MockWiki` → `MockWikipedia` to comply with project rules (no "Wiki" abbreviation).
- **Testing**: Created `service_ai_logic_test.go` and `service_ai_data_test.go` with comprehensive table-driven tests.

## v0.2.19
- **Fix**: **Startup Resilience**
    - Decoupled critical startup checks (Validator, Language Mapper) from the main application context using independent 60s timeouts.
    - Prevents "context deadline exceeded" errors during initialization on slower environments or when using strict deadlines.
- **Fix**: **Language Name Resolution**
    - Resolved issue where `{{.Language_name}}` resolved to "Unknown" in prompts due to a SPARQL query variable mismatch (`?langLabel` vs `?officialLangLabel`).
    - Invalidated language map cache (`v3`) to ensure fresh data fetch.
- **Fix**: **Template Context ("No Value")**
    - Resolved issue where TTS templates (e.g., `azure.tmpl`) rendered as "no value" for variable macros.
    - Refactored `AIService` to pass the full `NarrationPromptData` context to the TTS instruction renderer, ensuring all variables are available.
- **Improvement**: **Request Client Robustness**
    - Enhanced `request.Client` to explicitly log when jobs are dropped from the queue due to context expiry.
    - Improved backoff logic to retry on network errors and 429/5xx status codes while respecting cancellation.

## v0.2.18
- **Config**: **TTS Log Path**
    - The path for the TTS debug log is now configurable via `phileas.yaml` (default: `logs/tts.log`).
    - This allows users to redirect or organize logs as needed without code changes.

## v0.2.17
- **Feature**: **Improved Wikidata Ingestion**
    - Implemented a "Rescue" strategy for POIs that lack an English Wikipedia article.
    - The system now aggressively searches for *any* available label or sitelink (prioritizing Local > User > English > Any) to provide a valid name and URL.
    - This ensures that local landmarks (which often have no English article) are correctly ingested and displayed with their native name instead of a raw specific QID (e.g. `Q123043127`), significantly improving coverage in non-English speaking regions.
- **Refactor**: **Wikidata Service Architecture**
    - Split the monolithic `pkg/wikidata/service.go` into focused components (`service_rescue.go`, `service_enrich.go`) to reduce complexity.
    - Converted core processing methods into pure functions, significantly improving testability and maintainability.
- **Testing**: **Comprehensive Coverage Campaign**
    - Achieved **>80% Test Coverage** for `pkg/wikidata` core logic using table-driven tests.
    - Added comprehensive test suites: `service_rescue_test.go`, `service_enrich_test.go`, `client_test.go`, `mapper_test.go`, `scheduler_test.go`, and `validator_test.go`.
    - Implemented robust `httptest` mocking for Wikidata API client to simulate complex search and claim retrieval scenarios.
- **Tech Debt**: **Dependency Decoupling**
    - Refactored `LanguageMapper` to depend on the minimal `cache.Cacher` interface instead of the full `store.Store`, simplifying the dependency graph and enabling isolated testing.

## v0.2.16
- **Fix**: **TTS API Tracking (Fish Audio)**
    - Wired up the `tracker.Tracker` to the Fish Audio provider.
    - Providing correct `APISuccess` and `APIFailures` statistics to the Info Panel.
    - Injected tracker dependency via `narrator.NewTTSProvider` factory.

## v0.2.15
- **Refactor**: **Narrator Architecture & Testing**
    - Refactored `NarratorHandler` (`internal/api/narrator.go`) to use Interface Segregation Principle with local `AudioController` and `NarratorController` interfaces, removing dependencies on package-wide monolithic services.
    - Updated `narrator_test.go` to use simplified mocks, drastically improving test readability and maintainability.
    - Achieved **82% Test Coverage** for `pkg/narrator` by adding comprehensive table-driven tests for:
        - `factory.go`: Provider instantiation logic.
        - `service_utils.go`: Telemetry analysis helper (flight stage determination).
        - `service.go`: Full `StubService` lifecycle.
        - `service_ai_logic.go`: Complex AI logic including navigation instruction generation (Ground/Airborne/Clock Position/Relative), unit conversion (Imperial/Metric), and latency tracking.
- **Log**: **Narrator State Tracking**
    - Implemented state tracking in `NarratorHandler` (via `HandleStatus`) to log state changes (e.g., Idle -> Playing) at INFO level.
    - Verified logging behavior with unit tests.
- **Feature**: **Reset Nearby Narrations**
    - Added a "DANGER ZONE" button to the Info Panel to reset narration history for POIs within 100km.
    - Useful for content creators or testing to replay a specific flight segment without manually clearing the database.
    - Implemented with a specialized spatial update query for efficiency.
- **Fix**: **"Preparing" State**
    - Resolved a regression where the "Preparing" status was never shown in the UI because the `generating` flag was not being set in the backend.
- **Tweak**: **Fish Audio Prompts**
    - Updated prompt template to explicitly forbid markdown emphasis (asterisks) which are not supported by the engine.

## v0.2.14
- **UX**: **Responsive Playback Status**
    - Implemented optimistic UI updates for the "Play" button.
    - When clicked, the UI now instantly shows a "Loading..." state instead of waiting for the next server poll, providing immediate feedback that the request has been received.
    - Fixed a bug where the "Preparing" status was occasionally skipped in the UI.

## v0.2.13
- **UX**: **Click-Through Aircraft Icon**
    - The aircraft icon on the map is now "transparent" to mouse clicks (`interactive={false}`).

## v0.2.12
- **Feature**: **Fish Audio Model Selection**
    - Added support for specifying the Fish Audio model ID (e.g., `s1`) in configuration.
    - Updated API client to pass `model` parameter in synthesis requests.
    - Adapted fish.audio prompt template to speak in the voice and style of the Supreme Leader (will probably get boring fast, but it's fun to try)
- **Debug**: **TTS Header Logging**
    - Enhanced `tts.log` to include full request headers for deeper debugging of API interactions.
- **Maintenance**: **Log Rotation**
    - Changed log management strategy: logs are no longer truncated on startup. Instead, existing logs are rotated to `.old` (overwriting any previous `.old` file) to preserve the previous session's data for debugging.
- **Feature**: **Dynamic TTS UI Label**
    - The Info Panel now displays the active TTS provider name (e.g., "AZURE SPEECH") instead of a hardcoded label.
- **Feature**: **Configurable Mock Simulator**
    - Users can now configure the Mock Simulator's starting position (Lat/Lon/Alt/Heading) and phase durations in `phileas.yaml`.
- **Improvement**: **Refined Logging**
    - Downgraded noisy logs ("Setting prediction window", "Updated latency stats") to DEBUG.
    - Added "Narrator: Narration stats" log showing requested word count, generated word count, and audio duration.
    - Added "relative_dominance" strategy to the "Narrating POI" log to track dynamic length decisions.

## v0.2.11
- **Maintenance**: **Project Structure**
    - Moved debugging and Proof-of-Concept scripts (`debug_simconnect`, `latency_check`, `mocksim`, `simtest`) to `cmd/experiments` to clean up the root `cmd` directory.

## v0.2.10
- **Fix**: **Advanced Azure TTS Pronunciation**
    - Updated SSML strategy to prevent word truncation (e.g. "Seepyramide" -> "Se") by injecting punctuation inside `<lang>` tags if missing.
    - Updated Prompt Template to explicitly authorize the use of SSML `<phoneme>` tags (IPA) by the LLM for complex cases.
    - Removed previous `<break>` injection workaround which was ineffective for mid-word truncation.
- **Fix**: **Gemini Logging**
    - Improved `Gemini` logs to warn (rather than confusingly info-log) when Grounding/Search metadata is present but empty.
- **Maintenance**: **Log Rotation**
    - Added `logs/tts.log` to the list of log files truncated on application startup.

## v0.2.9
- **Feature**: **Dynamic TTS UI & Telemetry**
    - The Info Panel now dynamically updates to show the stats of the *active* TTS engine (e.g., "AZURE SPEECH", "FISH AUDIO") instead of hardcoded labels.
    - Implemented API-level tracking for Azure Speech success/failure rates.
- **Fix**: **SSML Robustness (Azure Speech)**
    - Implemented pre-emptive "Repair Logic" to strip hallucinated XML attributes (like `xml:ID`) from Gemini output *before* SSML validation.
    - This allows valid language tags (`<lang>`) to be preserved and handled correctly by Azure, preventing the engine from reading raw XML tags aloud due to fallback escaping.
- **Fix**: **Logging Noise**
    - Reduced verbosity of network request logs by excluding query parameters (specifically massive SPARQL queries), logging only Host and Path.
    - Fixed Gemini log to output the actual search query used (`WebSearchQueries`) instead of the CSS-laden `RenderedContent`.
- **Fix**: **Playback UI Status**
    - Resolved regression where the "Preparing" status was invisible because the title row was hidden when empty. Added "Loading..." fallback title.
### UNIT-TESTS
- Added table-driven tests for `ConfigurationHandler` to verify dynamic TTS config exposure.
- Added comprehensive SSML repair and validation tests in `pkg/tts/azure/provider_test.go`.

## v0.2.8 (Hotfix)
- **Fix**: Resolved panic in Gemini client logger caused by nil `SearchEntryPoint` in `GroundingMetadata`.
- **Refactor**: Extracted logging logic to `client_helper.go` and added comprehensive table-driven tests.

## v0.2.7
- **Fix**: Resolved issue where SimConnect "Cinematic" camera state (16) was treated as unknown, causing the UI to report "Disconnected". It is now correctly mapped to **Active** state.
- **Log**: Added `GroundingMetadata` verification logging to Gemini client to confirm search tool usage.

## v0.2.6
- **Feature**: **Audio Playback UI Enhancements**
    - **UI**: Shows the Title of the narration (POI Name or Essay Topic).
    - **UI**: Aligned telemetry items to the top of their cards for better consistency.
    - **UI**: Removed the "Visibility" telemetry card as data was unreliable.
    - **Progress**: Displays a circular progress indicator and total duration (M:SS) after the title.
    - **Backend**: Update Audio Manager to track and expose real-time playback position and duration via the API.
    - **Backend**: Removed `InCloud` and `VisualRange` telemetry tracking.
    - **Fix**: Resolved "..." delay in backend version display in the UI.
    - **Fix**: Resolved UI regressions (stale duration during preparing, "00" artifact, status flicker).
    - **Fix**: Resolved all linting errors in frontend and backend code.
### UNIT-TESTS
- Added `Position()` and `Duration()` methods to `pkg/audio` and verified via `manager_test.go`.
- Updated mocks in `pkg/narrator` to support the expanded interface.

## v0.2.5
- **Feature**: **Dynamic Narration Length (Relative Dominance)**
    - Implemented a "Relative Dominance" strategy for narration length.
    - **Logic**:
        - **High Competition**: If >1 high-quality rivals exist, skew narration shorter (to cover more POIs).
        - **Low Competition**: If 0-1 rivals (Lone Wolf), skew narration longer for better detail.
        - **Balanced**: Otherwise, standard random length.
    - **Optimization**: Updated `POIProvider.CountScoredAbove` to accept a `limit` parameter, stopping early (at 2) to save CPU cycles in dense areas.
### UNIT-TESTS
- Added `pkg/narrator/length_test.go` to verify statistical skew behavior.
- Added `pkg/narrator/sampling_test.go` to verify basic bounds and step logic.

### BUG FIXES
- **UI**: Fixed an issue where played POIs were hidden from the map if they were played more than 1 hour ago. They now remain visible indefinitely (blue marker).

## v0.2.4
- **Feature**: **POI Eviction & Re-hydration**
    - Implemented a memory management strategy to keep the application lightweight during long flights.
    - **Eviction**: Automatically removes POIs that are **> 90km away** (configurable via `max_dist_km + 10km`) AND located **behind** the aircraft.
    - **Re-hydration**: Implemented smart cache eviction for distant tiles. If the aircraft performs a U-Turn and re-enters a previously visited area, the system now "forgets" that it recently checked those tiles, allowing the scheduler to re-fetch them (hits DB cache) and restore the POIs to the map.
    - **Job**: Added a periodic `EvictionJob` (every 30s) to orchestrate this cleanup.
- **Refactor**: **Thread Safety**
    - Added `sync.RWMutex` to `wikidata.Service` to protect the in-memory tile cache (`recentTiles`) which is now accessed concurrently by the Scheduler and the Eviction Job.

## v0.2.3
- **Fix**: **Startup Crash (SimConnect DLL Loading)**
    - Resolved `failed to load SimConnect.dll` error by updating `sim_helper.go` to use an empty path, enabling the new auto-discovery logic (embedded DLL extraction).
- **Fix**: **Spurious Telemetry Validation**
    - Implemented `validateTelemetry` in SimConnect client to discard garbage data often sent during initialization or VR state changes.
    - Filters "Null Island" coordinates (0,0).
    - Filters spurious equatorial/polar mix coordinates (lat~0, |lon|~90).
    - Filters impossible state contradictions (On Ground + High Altitude).
    - Prevents "Navajo in Berlin" essay triggers caused by momentary telemetry glitches.
- **Fix**: **Azure TTS Prompt Engineering**
    - Updated Azure TTS system prompt to strictly forbid HTML/CSS styling (e.g., `font-family` spans) in the output, which were causing silence or errors in the TTS engine.
    - **Truncation Workaround**: Implemented automatic silence injection (`25ms` break) after every closing `</lang>` tag in the SSML payload. This forces the TTS engine to flush its audio buffer, preventing the last syllables of foreign words from being cut off during context switches.
- **Fix**: **Missing Airfields (e.g., Bautzen EDAB)**
    - Resolved issue where certain airfields were dropped because they hit tile fetch limits before sorting.
    - **Increased Tile Capacity**: Raised `max_articles` from 500 to 1000 per tile to handle dense data regions without dropping valid POIs via `wikibase:around` pre-filtering.
- **Fix**: **Nameless "Ghost" POIs & Dynamic Language Resolution**
    - Resolved issue where POIs in border regions (e.g., DE/CZ) were ingested without names due to language mismatches (looking for German articles for Czech entities).
    - Implemented a **Dynamic Language Mapper**: Phileas now learns the primary language for every country from Wikidata and prioritizes it over hardcoded assumptions.
    - Added **Label Service Fallback**: If no Wikipedia article exists in the target languages, the system now falls back to the Wikidata Label (`rdfs:label`) to ensure every POI has a name.
    - Enforced **Strict Validation**: POIs that remain nameless after all fallbacks are now explicitly dropped to prevent "Q-ID" only narrations.
- **Fix**: **Replay UI Synchronization**
    - Resolved issue where replaying a narration left the UI in an "Idle" state with no title or info card.
    - **Backend State Tracking**: The `AIService` now persists the `lastPOI` and `lastEssayTopic` state specifically for replay scenarios.
    - **State Restoration**: The `ReplayLast` method now proactively restores the narration context (Title, ID, Info Card data) to the frontend before triggering audio playback.
    - The UI now correctly displays the "Playing" status (Green Badge), Title, and full Info Card during a replay.

## v0.2.2
- **Enhancement**: **Embedded SimConnect.dll**
  - The application now embeds the SimConnect DLL directly into the executable at build time.
  - SDK is now only required for building from source.
- **Fix**: Corrected web build output path in `vite.config.ts` to prevent creation of duplicate `internal/ui/internal` directory.
- **Refactor**: Improved Makefile PowerShell compatibility using `scripts/copy_simconnect.ps1` for robust DLL copying.
- **Cleanup**: Removed unused `findMSFSInstallPath` function (lint fix) and updated `.gitignore` to exclude build-time artifacts.

## v0.2.1
- **Fix**: Resolved `deferInLoop` lint error in `pkg/sim/simconnect/dll.go` by closing registry keys immediately after use instead of deferring inside the loop.

## v0.2.0
- **Distribution**: Prepared project for public GitHub release.
    - **Makefile**: Removed debug targets, updated web path to `internal/ui/web`, fixed build order, added `vendor` target, output exe to project root.
    - **Install Script**: Created idempotent `install.ps1` that downloads GeoNames data, prompts for MSFS POIs, and generates config.
    - **SimConnect**: Added `FindDLL()` auto-discovery for SimConnect.dll (supports Steam and Microsoft Store MSFS installations).
    - **Config**: Added `--init-config` flag to generate default config file and exit.
    - **Documentation**: Created `README.md` with installation, configuration, and usage instructions.
    - **License**: Added MIT License.

## v0.1.188
- **Feature**: **Visibility Telemetry (Fixed)**
    - Instrumented `Ambient In Cloud` and `AMBIENT VISIBILITY` (fixed casing).
    - Added "VISIBILITY" card to the Info Panel displaying visibility (km) and "In Cloud" status.
    - Validated using `cmd/debug_simconnect`. `CLOUD COVERAGE DENSITY` was excluded due to Exception 7.

## v0.1.187
- **Revert**: **Visibility Telemetry**
    - Removed visibility telemetry integration due to SimConnect instability (Exception causing memory corruption).
    - Reverted changes to `pkg/sim` and Frontend.

## v0.1.186
- **Feature**: **Visibility Telemetry** (BROKEN/REVERTED)
    - **SimConnect**: Instrument `AMBIENT IN CLOUD`, `AMBIENT VISIBILITY`, and `CLOUD COVERAGE DENSITY`.
    - **Note**: This release caused server instability and was immediately reverted.

## v0.1.185
- **Adaptive Navigation Prediction**: Implemented a feedback loop that adjusts the aircraft position prediction based on the observed latency between POI selection and audio playback start.
    - Updated `sim.Client` interface to support dynamic prediction windows.
    - Implemented rolling window latency tracking in `AIService` (excluding essays).
    - Ensures navigation instructions (e.g., "at your 2 o'clock") refer to the aircraft's position when the user hears the instruction.
- **Flight Context Injection**: Injected raw flight telemetry (Altitude, Heading, Speed, Predicted Position) into the narrator prompt's "NO REFERENCE TO THE FLIGHT" section.
    - Added `flight_data` macro to `macros.tmpl`.
    - Gives the LLM ground truth to prevent hallucinations about flight parameters.
- **Refactor**: Reduced cyclomatic complexity in SimConnect client message handling.
    - Extracted helper methods (`handleOpen`, `handleQuit`, `handleAssignedObject`, `handleSimObjectData`) from `handleMessage`.
    - Resolved `golangci-lint` cyclomatic complexity warning (>15).
- **Unit Tests**:
    - Added `pkg/narrator/latency_test.go` to verify rolling window logic.
    - Updated `pkg/narrator/mocks_dev_test.go` and other tests to support the new `sim.Client` interface.

## v0.1.183
- **Refactor**: **Dynamic Narration Length**
    - Renamed propery `max_words` to `narration_length_min` / `narration_length_max` in configuration.
    - Implemented dynamic sampling logic: requests a random word count between Min/Max with a step size of 10 (e.g., 400, 410, 420...).
    - Allows for more varied essay lengths compared to the previous hardcoded 500-word limit.
### UNIT-TESTS
- Added `pkg/narrator/sampling_test.go`:
    - Verified random value falls within configured range.
    - Verified step integrity (value is always a multiple of 10).
    - Verified edge case handling (min > max, zero values).

## v0.1.182
- **Fix**: **SimConnect Reconnection**
    - Implemented subscription to `SimStop` system event to explicitly detect simulator quit.
    - Prevents "Zombie" connection state where client remains connected but receives no data after Sim restart.

## v0.1.181
- **Debug**: **Dedicated TTS Logging for All Providers**
    - Added dedicated logging to `logs/tts.log` for Azure, EdgeTTS, Fish Audio, and SAPI.
    - Captures final prompt (SSML or text) sent to the engine and status.
    - Format: `[TIMESTAMP] STATUS: <code> | PROMPT: <ssml>`.
    - This helps debug validation failures and fallback escapes that may break pronunciation tags.

## v0.1.180
- **Cleanup**: **Remove Azure TTS Logging**
    - Removed `[AZURE_TTS_SSML]` debug logging from `gemini.log` as the issue is resolved.
    - Reverted changes to logging package that exposed tracking file paths.

## v0.1.179
- **Debug**: **Move Azure TTS Logging to gemini.log**
    - Redirected Azure TTS SSML output from stdout/server.log to `gemini.log`.
    - Use `[AZURE_TTS_SSML]` prefix in `gemini.log` to distinguish from LLM prompts.
    - This allows viewing TTS input alongside LLM generation for easier debugging of pronunciation tags.

## v0.1.178
- **Fix**: **Azure TTS Multilingual Support**
    - Removed `mstts:express-as` style wrapper from the SSML payload.
    - Research indicates that applying a primary-language-specific style (like "narration-friendly") can override or break the behavior of nested `xml:lang` tags, causing multilingual voices to ignore language switching.
    - This change should restore correct pronunciation for non-English names when using multilingual neural voices.

## v0.1.177
- **Debug**: **Azure TTS Logging**
    - Added comprehensive logging of the constructed SSML payload sent to Azure Speech.
    - Added logging of SSML validation errors (including the exact input) to help debug pronunciation tag issues.
    - Output is written to stdout/server log with prefix "Azure TTS SSML:".

## v0.1.176
- **Fix/Debug**: **Manual Play Button**
    - Increased `z-index` of the Play button in `POIInfoPanel` to prevent overlap by other elements (like Close button or wrapped text).
    - Added `console.log` to Play button click handler for easier frontend debugging.
    - Added debug log to `HandlePlay` backend endpoint to trace request arrival.
    - This release primarily instruments the application to diagnose why manual play requests were lost, with a speculative fix for z-index overlap.

## v0.1.175
- **Fix**: **Gemini Log Truncation** - Resolved regression where all log lines were being truncated at 80 characters.
    - Updated `truncateParagraphs` to only apply truncation and empty-line removal **inside** the `<start/end of Wikipedia article>` block.
    - Preserves prompt structure and system instructions while still compacting long article text.
### UNIT-TESTS
- Removed `TourGuideName` from prompt templates in previous steps (v0.1.174 cleanup).
- Added `TestTruncateParagraphs` with table-driven tests covering empty strings, no-op cases (outside block), and correct truncation (inside block).

## v0.1.174
- **Fix**: **Template Parsing Error** - Removed nested template syntax from `maybe`/`pick` function arguments that caused startup failure.
- **Cleanup**: **Removed `TourGuideName`** from all prompt templates (legacy remnant from two-person conversation design).
    - Updated `script.tmpl`, `essay.tmpl`, `macros.tmpl`, `readout.tmpl`.
    - Simplified narration prompts to single-narrator style.
- **Documentation**: Added warning to `README.md` about no nested templates in function arguments.
### UNIT-TESTS
- Added `TestProductionTemplates` to verify actual production templates parse correctly (catches syntax errors early).

## v0.1.173
- **Feature**: **Prompt Variety System** - Multiple mechanisms to produce varied narration scripts.
    - **Temperature Bell Curve**: LLM temperature now varies using a normal distribution around `temperature_base` ± `temperature_jitter` (default: 1.0 ± 0.2).
    - **`{{maybe N "content"}}`**: New template function that includes content with N% probability.
    - **`{{pick "A|||B|||C"}}`**: New template function that randomly selects one option from a `|||` separated list.
    - **`{{interests .Interests}}`**: Now excludes 2 random topics from the list (if ≥4) for variety.
    - **Word Count Variation**: Added `max_words_min` and `max_words_max` config (waiting for narrator hookup).
- **Documentation**: Created comprehensive `configs/prompts/README.md` with all template functions, data fields, and variety mechanisms.
- **Config**: Added `temperature_base`, `temperature_jitter`, `max_words_min`, `max_words_max` to `phileas.yaml`.
- **Cleanup**: Removed unused `essay.score_threshold` config field.
### UNIT-TESTS
- Added `TestMaybeFunc` (0%, 100%, and 50% probability edge cases).
- Added `TestPickFunc` (single option, multiple options, whitespace trimming).
- Updated `TestInterestsFunc` to verify 2-topic exclusion behavior.

## v0.1.172
- **Feature**: **Externalized Interests Configuration**.
    - Moved topics list from `script.tmpl` to `configs/interests.yaml`.
    - Topics are shuffled each prompt generation to improve variety.
    - Added `interests` template function in prompts manager.
- **Feature**: **SSML Validation for Azure TTS**.
    - Validates SSML before sending to Azure Speech API.
    - Falls back to escaped plain text if SSML is malformed.
- **Feature**: **Improved Gemini Model Validation**.
    - Uses `Models.Get` (1 request) for normal validation.
    - On failure, lists available models for recovery.
- **Improvement**: **Gemini Log Formatting**.
    - WP article paragraphs truncated at 80 chars (no word wrap).
    - Empty lines filtered from prompts.
- **Fix**: Added missing `xmlns:mstts` namespace in Azure SSML.
- **Fix**: Lint errors (`nestingReduce`, `SA9003`).
### UNIT-TESTS
- Added tests for `interestsFunc` (shuffle verification).
- All tests pass via `make test`.

## v0.1.171
- **Feature**: **Multi-Model Support**.
    - Implemented intent-based model selection (`essay`, `narration`, `dynamic_config`).
    - Added `profiles` section to `phileas.yaml` configuration.
- **Feature**: **Google Search Enabled**.
    - Upgraded LLM SDK to `google.golang.org/genai` (v1.40.0) from the deprecated `generative-ai-go`.
    - Enabled the **Google Search** tool for all Gemini requests to provide grounded responses.
- **Refactor**: Replaced legacy Gemini client with official Go SDK.
### UNIT-TESTS
- Validated via `go vet` and `go test ./pkg/llm/gemini/...`.
- Verified compilation and Google Search struct availability.

## v0.1.170
- **Fix**: Resolved `SQLITE_BUSY` (database locked) errors during high-concurrency writes.
    - Increased `busy_timeout` to 30s (via `PRAGMA`).
    - Enforced `db.SetMaxOpenConns(1)` to strictly serialize DB access application-side.
### UNIT-TESTS
- Validated via standard release protocol.

## v0.1.169
- **Maintenance**: Manual configuration update.
### UNIT-TESTS
- Validated via standard release protocol.

## v0.1.168
- **Maintenance**: Fixed `hugeParam` lint error in `NewTTSProvider` by passing configuration object by pointer.
- **CI**: Enforced stricter linting checks.
### UNIT-TESTS
- Validated via `golangci-lint run`.

## v0.1.167
- **Config**: Made telemetry source configurable via `phileas.yaml` (`sim.provider`).
    - Defaults to `simconnect` with fallback to `mock`, but can enforce `mock` directly.
    - Code now initializes simulation client based on static config rather than DB state.
### UNIT-TESTS
- Validated via `go vet` and manual inspection of config loading logic.

## v0.1.166
- **Feature**: Added **Azure Speech** support for TTS.
    - Implemented `azure-speech` engine with `narration-friendly` capability.
    - Updated prompt logic to request SSML language tags for correct foreign pronunciation (`<lang xml:lang="...">`).
    - Config: Added `azure_speech` (Key, Region, Voice).
- **Refactor**: Genericized `tts.Provider` initialization to easily plug in new engines.
### UNIT-TESTS
- Added `pkg/tts/azure` package.
- Manual verification of SSML construction via walk-through.

## v0.1.165
- **TTS Integration**: Implemented support for **Fish Audio** (Deep Learning TTS).
    - Can now use high-quality, emotive voices via the Fish Audio API.
    - Configuration: Added `tts.fish_audio` section and `tts.engine: fish-audio` support.
- **Narrator**: Refactored TTS Prompt Management.
    - Instructions are now injected dynamically based on the active TTS engine.
    - **Fish Audio**: Uses "Emotion Control" tags (e.g. `[excited]`, `[whisper]`) for expressive narration.
    - **Edge TTS**: Uses clean script format without speaker labels.
    - Removed legacy `male_voice_id`/`female_voice_id` settings in favor of engine-specific config.
- **Config**: Added `configs/prompts/tts/` for engine-specific templates.
### UNIT-TESTS
- Added `phileasgo/pkg/tts/fishaudio` package (provider implementation).
- Verified `pkg/narrator` logic with updated `service_ai.go`.
- Manual verification of template rendering logic via walkthrough.

## v0.1.164
- **UI/Nav Fixes**:
    - **Panel State**: Fixed issue where the POI Info Panel displayed mixed content (new text, old thumbnail) by forcing a full component refresh on POI change.
    - **Selection**: Resolved state conflict where manual selection loop logic was fighting with the narrator's auto-scroll, preventing users from inspecting other POIs during narration.
- **Classifier Fix**:
    - **Hierarchy Traversal**: Fixed a critical bug where intermediate nodes with empty categories in the database cache were treated as dead-ends. The classifier now correctly falls through to verify parents, ensuring deep hierarchies (like *Bridge* -> *Architectural Structure*) are traversed correctly.
    - **Batching**: Updated `fetchAndCacheLayer` to resolve categories immediately from parents during fetch, preventing "empty" nodes from poisoning the cache.
- **Investigation**: Confirmed "Census-designated place" items fall through to "Area" rescue by design.
### UNIT-TESTS
- Verified frontend changes manually (UI consistency and selection persistence).

## v0.1.163
- **Beacon**: Refined altitude behavior on low-level flight. Beacons now lock their altitude when aircraft is below 2000ft AGL, providing a stable visual reference instead of following terrain.
### UNIT-TESTS
- Added `TestUpdateLoop_AltitudeLock` to `pkg/beacon/service_test.go` to verify the new altitude locking logic.

## v0.1.162
- **Configuration**: Added missing `units` parameter to `phileas.yaml`.
  - Default: `hybrid`
  - Options: `imperial`, `metric`, `hybrid`
### UNIT-TESTS
- No new unit tests required as this is a configuration schema fix to match existing code.

## v0.1.161 (2026-01-02) - UX Tweak
### Improvements
*   **UI/UX**:
    *   Moved the narration title **below** the playback controls.
    *   This prevents the control buttons from jumping up/down when the title appears or disappears, resulting in a more stable and usable interface.
### UNIT-TESTS
*   Verified frontend build.

## v0.1.160 (2026-01-02) - Responsive UI
### Improvements
*   **UI/Responsive Sidebar**:
    *   Switched the sidebar width from a fixed `360px` to a responsive **30%** of the screen width (`30vw`).
    *   Added a minimum width of `360px` to ensure usability on smaller screens or narrow windows.
    *   This provides more breathing room for the info panel content on wider landscape displays while maintaining a solid baseline.
### UNIT-TESTS
*   Verified frontend build and responsiveness manually.

## v0.1.159 (2026-01-02)
- **Maintenance**: Resolved linting errors (unchecked error returns) in `pkg/poi` tests to ensure a clean `make test`.

## v0.1.158 (2026-01-02) - UI & Narration Titles
### Improvements
*   **Narrator**:
    *   Exposed `CurrentTitle()` in the narrator backend.
    *   For Essays: Title is initially "Essay about <Topic>", then updates to the actual parsed title (e.g., "THE HISTORY OF AVIATION") once the script is generated.
*   **UI/Playback**:
    *   Updated `PlaybackControls` to display the narration title above the controls.
    *   Switched to a vertical layout (Title Row + Controls Row) to accommodate the title without cramping the UI.
*   **UI/Fixes**:
    *   **Volume Slider**: Fixed issue where the volume slider overflowed its container in narrow layouts by properly constraining its flex width.
### UNIT-TESTS
*   Updated `pkg/core` tests to support new interface methods.
*   Verified backend logic for title parsing.

## v0.1.157 (2026-01-02) - UI Resilience & Config
### Improvements
*   **UI/Resilience**:
    *   Implemented a "Latched" Connection Error state. The UI now persists the "Connection Error" screen (with a subtle "Retrying..." indicator) during reconnection attempts instead of flickering back to "Connecting...".
    *   This eliminates the "Strobe Light" effect when the backend is down.
*   **Configuration**:
    *   Made the maximum Wikidata fetch distance configurable via `wikidata.area.max_dist_km` (default 100km).
*   **UI/Polish**:
    *   Enforced a minimum width for the volume slider to prevent it from shrinking too much in landscape layouts.
### UNIT-TESTS
*   Verified frontend build for UI logic changes.
*   Verified Wikidata scheduler logic via existing tests (configurable distance).

## v0.1.156 (2026-01-02) - Narrator Responsiveness
### Improvements
*   **Narrator**:
    *   **Unit Support**: Implemented unit selection for the narrator (Imperial, Metric, Hybrid).
        *   Imperial: "2 miles" (default).
        *   Metric: "3 kilometers" (used when units are metric or hybrid).
        *   Updates prompt macro to respect user configuration.
    *   **Responsiveness**: Moved beacon creation to occur *before* the LLM generation step. This provides immediate visual feedback (target marker appears) as soon as the POI is selected, rather than waiting for script generation and TTS.
    *   **Resilience**: Added robust cleanup logic to ensure beacons are removed if the narration fails (LLM error, TTS error) to prevent "orphaned" markers.
### UNIT-TESTS
*   Added `TestAIService_NavUnits` to verify correct distance string formatting based on unit configuration.
*   Added `TestAIService_BeaconCleanup` to verify that beacons are cleared if downstream processes fail.

*   **Logging**:
    *   Downgraded "Reprocessed tile and rescued entities" message to DEBUG to reduce console spam.

## v0.1.155 (2026-01-02) - Visibility Table Update
### Improvements
*   **Config**: Updated `visibility.yaml` definitions:
    *   Extended altitude tiers up to 100,000 ft (previously max 10,000 ft).
    *   Smoothed visibility distance progression.
    *   Added high-altitude tiers (15k, 20k, 30k, 100k) with appropriate visibility caps.
### UNIT-TESTS
*   No new unit tests required (configuration data change only).

## v0.1.154 (2026-01-02) - UI Flight Stage
### Improvements
*   **UI/InfoPanel**:
    *   Replaced the binary "GROUND" / "AIR" status with the actual Flight Stage (e.g., "GROUND", "TAKEOFF/CLIMB", "CRUISE", "APPROACH/LANDING").
    *   Implemented `DetermineFlightStage` helper in `pkg/sim` to centralize this logic based on IsOnGround flag, Altitude AGL, and Vertical Speed.
    *   Exposed `FlightStage` in the `Telemetry` struct for frontend consumption.
### UNIT-TESTS
*   Verified logic via `pkg/sim/mocksim` updates. No new specific unit tests required as logic was moved/centralized from existing tested `service_utils.go`.

## v0.1.153 (2026-01-01) - Log Wrapping Improvements
### Improvements
*   **Logging**:
    *   Updated `gemini.log` to wrap the full prompt text (including Wikipedia article content) to 80 characters. This improves readability when debugging prompt construction and AI inputs.


## v0.1.152 (2026-01-01) - Ground & Flight Stage Fixes
### Improvements
*   **Narrator**:
    *   Corrected the "Flight Stage" passed to the narrator prompt. It now dynamically calculates the stage ("Ground", "Takeoff/Climb", "Approach/Landing", "Cruise") based on telemetry instead of being hardcoded to "Cruise".
*   **Visibility/Scorer**:
    *   Standardized `IsOnGround` logic throughout the application.
    *   Replaced explicit `AltitudeAGL < 50` checks with a unified `isOnGround` boolean passed from the simulator telemetry.
    *   Refactored `Calculator` and `Scorer` to rely on this flag, ensuring consistent ground visibility behavior across all modules.

### UNIT-TESTS
*   Updated `pkg/scorer/scorer_test.go` and `pkg/visibility/visibility_test.go` to include `IsOnGround` flag in test cases, ensuring ground logic is correctly validated.


## v0.1.151 (2026-01-01) - Log Field Improvements
### Improvements
*   **Wikidata**:
    *   Renamed log field `tiles_rescued` to `dynamic_pois_added` in `ReprocessNearTiles` completion log to accurately reflect the semantic meaning.
    *   Corrected the `rescuedCount` logic to sum the actual number of rescued POIs instead of counting the number of tiles that had rescues.
### UNIT-TESTS
*   No new unit tests required (logging and metric correction only).

## v0.1.150 (2026-01-01) - Info Panel Thumbnail Logic
### Improvements
*   **UI/InfoPanel**:
    *   Removed `max-height: 300px` limitation from the POI Info Panel thumbnail. It now maintains its natural aspect ratio at 60% of the container width, allowing taller images to display fully (as the main container now expands vertically to accommodate).
### UNIT-TESTS
*   No new backend unit tests required (frontend layout change only).

## v0.1.149 (2026-01-01) - Info Panel Layout Fixes
### Improvements
*   **UI/InfoPanel**:
    *   Updated the POI Info Panel layout to fill all remaining vertical space in the sidebar (`flex: 1`).
    *   Refactored the internal layout so the text column and score breakdown expand to fill the available height, ensuring the breakdown uses all empty space before scrolling.
    *   Removed rigid height constraints on the breakdown text directly.
### UNIT-TESTS
*   No new backend unit tests required (frontend layout change only).

## v0.1.148 (2026-01-01) - Ground Visibility Refactor
### Improvements
*   **Scorer**: Removed the special hardcoded scoring rules for Aerodromes on the ground to align with standard visibility logic.
*   **Visibility**:
    *   Unified ground and air visibility logic. Ground visibility is now purely distance-based, using the 0ft AGL entries in the visibility table.
    *   Updated `Calculator` to bypass blindspot and bearing penalties when on the ground (AGL <= 50ft).
*   **Testing**: Updated `pkg/scorer` unit tests to verify ground visibility scenarios explicitly.
### UNIT-TESTS
*   Updated `TestScorer_Calculate` in `pkg/scorer/scorer_test.go`:
    *   Removed `Ground_Suppression` test (logic removed).
    *   Added `Ground_(Standard_Vis)` to verify distance-based ground scoring.
    *   Updated `Ground_(Aerodrome)` to reflect standard visibility scoring instead of hardcoded boost.
    *   Updated test mock visibility table to support 0ft AGL distances.

## v0.1.147 (2026-01-01) - MSFS POI Boost & Badging
### Improvements
*   **Scoring**:
    *   Applied a **4.0x** score multiplier to POIs identified as MSFS POIs, giving them significant priority in narration selection.
    *   Updated `ScoringExplanation` to explicitly list "MSFS POI: x4.0" when applied.
*   **UI**:
    *   Added a gold **Star Badge** (★) to the POI marker on the map for MSFS POIs.
    *   Updated `POI` frontend model to include `is_msfs_poi` flag.
### UNIT-TESTS
*   Added `TestScorer_Calculate/MSFS_POI_Boost` to `pkg/scorer/scorer_test.go` to verify the 4.0x multiplier logic.
*   Verified frontend build with updated TypeScript interfaces.

## v0.1.146 (2026-01-01) - MSFS POI Integration
### New Features
*   **MSFS Integration**: Added ability to flag POIs that overlap with known MSFS POIs (e.g. airports, landmarks).
    *   Added `is_msfs_poi` flag to `POI` model and database.
    *   Implemented `CheckMSFSPOI` in store to detect overlaps based on category-specific merge distances.
    *   Updated `poi.Manager` to automatically check for overlaps during enrichment.
### Improvements
*   **Logic Refactoring**:
    *   Refactored `pkg/poi/manager.go` `Upsert` logic to reduce complexity.
    *   Refactored `pkg/wikidata/service.go` `classifyInChunks` to improve maintainability.
*   **Database**: Added `CheckMSFSPOI` to `Store` interface and `SQLiteStore` implementation.
*   **Migration**: Added `scripts/migrate_msfs_flags` to backfill MSFS flags for existing POIs.
### Bug Fixes
*   Fixed lint errors regarding cyclomatic complexity in core logic.
*   Fixed a `defer` issue in the migration script.
### UNIT-TESTS
*   Verified `CheckMSFSPOI` logic in `pkg/store/sqlite_test.go` implicitly via integration (and existing MSFS tests).
*   Mock stores updated across `pkg/narrator`, `pkg/classifier`, `pkg/poi` to support new interface method.
*   Verified `poi.Manager` correctly flags overlaps.

## v0.1.145 (2026-01-01)
- **Optimization**: Reduced redundant API calls to Wikidata.
    - Implemented `seen_entities` optimization to store instances of ignored entities.
    - This allows re-evaluating ignored entities locally during reprocessing without fetching metadata from the API.
    - **Schema**: Updated `seen_entities` to store `instances` JSON column.
    - **Logic**: Refactored `wikidata.Service` to verify local instances before network requests.

### UNIT-TESTS
- Updated mocks and verified `pkg/store`, `pkg/wikidata`, `pkg/classifier`, `pkg/poi`, `pkg/narrator` tests pass.

## v0.1.144 (2026-01-01)
- **Maintenance**: Resolved linting errors across the codebase.
    - **API**: Fixed unchecked `json.Encode` errors in `internal/api/pois.go` and refactored `HandleThumbnail` to reduce complexity (from 18 to <15).
    - **Scripts**: Fixed `log.Fatalf` misuse in `scripts/inspect_tile` to ensure proper DB cleanup via deferred calls.
    - **Beacon**: Resolved ineffectual assignment in `pkg/beacon/service.go` spawn logic.
    - **Imports**: Fixed missing `model` package imports in API handler.

### UNIT-TESTS
- Maintenance release, verified `make test` runs clean (0 exit code).

## v0.1.143 (2026-01-01)
- **Fix**: Resolved "Repeated Narration" bug when skipping.
    - Previously, pressing "Skip" could cause the same POI to be selected again immediately because the `LastPlayed` timestamp was only updated *after* narration finished, and the background scorer (running every 5s) was using stale data.
    - **Solution**: Moved the `LastPlayed` timestamp update to the **start** of the narration process. This ensures the POI is immediately marked as played, allowing the scorer to penalize it (score = 0) before the next selection occurs.
- **Maintenance**: Checked and verified full test suite.

### UNIT-TESTS
- Verified `pkg/narrator` tests pass with the logic changes.

## v0.1.142 (2025-12-31)
- **Audio**: Added persistent volume control.
    - Added a volume slider to the Info Panel configuration section.
    - Volume setting is saved to persistent state and restored on application startup.
    - Implemented real-time volume adjustment using `beep/effects`.

## v0.1.141 (2025-12-31)
- **Config**: fixed a bug where the `target_language` config was partly ignored.
    - Now correctly passing `UserLang` from the config to the Wikidata service.
    - Added normalization to strip regional codes (e.g., `en-US` -> `en`) to ensure compatibility with Wikidata's language codes.

## v0.1.140 (2025-12-31)
- **Wikidata**: Relaxed the language requirement for POIs.
    - Previously, items were *strictly* required to have an article in the local language (e.g., German if in Germany).
    - Now, items are accepted if they have an article in the **Local Language**, **User's Language** (English), OR **English** (fallback).
    - This ensures that prominent international landmarks (which might have an English Wikipedia entry but a missing/stub local one) are not filtered out.

## v0.1.139 (2025-12-31)
- **Config**: Lowered `Park` category `sitelinks_min` from 5 to 2. This restores visibility for significant regional parks (e.g., Lake Pleasant Regional Park) that were previously filtered out due to strictly enforcing international notability (usually requiring 5+ wikis).

## v0.1.138 (2025-12-31)
- **Wikidata**: Increased `max_articles` query limit from 100 to 500 to prevent prominent POIs (like Lake Pleasant) from being dropped in areas with many minor entities.

## v0.1.137 (2025-12-31)
*   Fix: EdgeTTS failure due to unescaped XML (added `pkg/tts/edgetts/ssml_test.go`).
*   Fix: Refined auto-panel suppression logic (only represses same POI ID).
*   UNIT-TESTS: Added new tests for SSML construction and verified existing tests pass.

## v0.1.136 (2025-12-31)
- **Feat**: Implemented "Safe Altitude" logic for Beacon formation.
    - **Low Altitude (< 1000 ft AGL)**: Target balloon spawns at `Aircraft Altitude (MSL) + 1000 ft` to ensure visibility. Formation balloons are suppressed.
    - **High Altitude (> 1000 ft AGL)**: Standard behavior (Target + Formation).
- **UI**: Improved POI Info Panel layout.
    - Switched to a responsive percentage-based layout (Image 60% / Text 40%).
    - Removed fixed pixel constraints to allow the thumbnail to maximize available space in the sidebar.

### UNIT-TESTS
- Added `TestSetTarget_LowAGL` and `TestSetTarget_HighAGL` to `pkg/beacon` to verify conditional spawning logic.

## v0.1.135 (2025-12-31)
- **UI**: POI Info Panel now properly shows text on the left and thumbnail on the right using flexbox layout.
- **Feature**: Panel stays closed after user dismisses it until they manually select a new POI from the map.

### UNIT-TESTS
- No new unit tests required; UI layout and state management changes only.

## v0.1.134 (2025-12-31)
- **UI**: Thumbnail now uses CSS float layout in POI Info Panel. Text (including Score Breakdown) flows around the image on the left, eliminating wasted vertical space and scroll bars.

### UNIT-TESTS
- No new unit tests required; purely cosmetic UI layout change.

## v0.1.133 (2025-12-31)
- **Fix**: Beacon formation despawn now uses the correct SimConnect connection handle. Previously, the formation beacons would stop updating but not actually despawn when reaching the 3km trigger distance.
- **UI**: Thumbnail moved to the right side of the POI Info Panel with responsive sizing to fill available space.
- **UI**: Increased thumbnail resolution from 300px to 800px for better VR readability.
- **Config**: Updated City category weight from 0.5 to 0.8.
- **Prompt**: Fixed label for recent POIs context in script template (was "Description", now "Recent POIs").

### UNIT-TESTS
- No new unit tests required; changes were to UI styling, configuration values, and a SimConnect handle fix that was verified manually with live testing.

## v0.1.132 (2025-12-31)
- **Feature**: POI Info Panel now auto-opens when the narrator starts playing a POI and auto-closes when playback ends (unless manually opened).
- **Feature**: POI Info Panel moved from map overlay to dashboard area, fully covering telemetry/stats when active.
- **Feature**: Wikipedia thumbnail is now fetched on-demand when a POI is selected and displayed in the Info Panel. Thumbnails are persisted to avoid repeated API calls.
- **Backend**: Added `GET /api/pois/{id}/thumbnail` endpoint and `ThumbnailURL` field to POI model.

## v0.1.131 (2025-12-31)
- **Feature**: Restricted regional essays to altitudes above 2000 ft AGL. This ensures a cleaner experience during the busy takeoff and landing phases, reserving broad regional context for the cruise segment.
- **Workflow**: Updated essay trigger logic in `NarrationJob`.

## v0.1.130 (2025-12-31)
- **Fix**: Corrected airborne detection logic in the POI scorer to use `AltitudeAGL` instead of `AltitudeMSL`. This ensures correct scoring multipliers (bearing, blind spot) are applied based on height above ground, fixing issues at high-altitude airports like Leadville (KLXV).
- **Config**: Updated `visibility.yaml` to provide non-zero visibility distances at 0ft altitude (S:0.5, M:1.0, L:3.0, XL:4.0 NM). This ensures the visibility layer is visible while on the ground.
- **Workflow**: Finalized v0.1.130 release.

### UNIT-TESTS
- Verified `pkg/scorer` tests pass with the new `AltitudeAGL` logic. The tests already covered ground states with 0ft altitude, which now correctly maps to the non-airborne path.

## v0.1.129 (2025-12-31)
- **Refactor**: Renamed `Altitude` to `AltitudeMSL` (Mean Sea Level) and `Speed` to `GroundSpeed` across the entire application (SimConnect, Backend, and Frontend) to prevent confusion between different measurement types.
- **Refactor**: Renamed `Altitude` fields in visibility configuration to `AltAGL` (Above Ground Level), making it explicit that visibility lookups are based on height above terrain.
- **HUD**: Updated the info panel to display explicit MSL/AGL altimeter values and "GS" for ground speed.
 
## v0.1.128 (2025-12-31)
- **Log**: Downgraded "Attempting SimConnect connection..." to DEBUG level to reduce console clutter when the simulator is not running.
 
 ## v0.1.127 (2025-12-31)
- **Improvement**: Refactored Beacon Service connection lifecycle. The service now handles SimConnect connections asynchronously with a 1-minute retry loop, eliminating startup warnings when the simulator is not running and providing automatic reconnection.
  
## v0.1.126 (2025-12-31)
- **Refactor**: Unified script length control to use `.MaxWords` across all prompt templates (POI narrations and Regional Essays).
- **Config**: Added `max_words` setting to `phileas.yaml` for global POI narration length control.
  
## v0.1.125 (2025-12-31)
- **Feature**: Implemented category-specific prompt macros. You can now define templates in `configs/prompts/category/<name>.tmpl` and include them in the narrator script using `{{category .Category .}}`.
- **Improvement**: Switched to fetching the **full article content** from Wikipedia instead of just the lead section (`exintro`), ensuring the AI has access to the complete set of facts for narration.
- **Maintenance**: Added unit tests for the prompt macro system.
 
## v0.1.124 (2025-12-31)
- **Fix**: Resolved issue where POIs disappeared after a restart due to the "Early Exit" optimization. Implemented a memory hydration step (`TrackPOI`) that correctly registers existing POIs in the active tracking list without redundant processing.

## v0.1.123 (2025-12-31)
- **Improvement**: Optimized Wikidata ingest pipeline by filtering out existing POIs early. This drastically reduces redundant classification and Wikipedia metadata requests, especially during tile reprocessing.

## v0.1.122 (2025-12-31)
- **Improvement**: Consolidated Wikidata tile reprocessing logs into a single summary line to reduce log spam.

## v0.1.121 (2025-12-31)
- **Feature**: POI Markers now show status via color: **Green** while playing/preparing, **Blue** for previously narrated.
- **Improvement**: Added a 3-second mandatory pause between narrations to prevent overlap and improve pacing.
- **Logic**: Dynamic Config Reload now requires both >30m AND >50nm traveled since last run.

## v0.1.120 (2025-12-31)
- **Feature**: POI Info Panel now displays `last_played` in a human-readable format (e.g., "5m ago", "2h ago").

## v0.1.119 (2025-12-31)
- **Improvement**: Category Group Variety Penalty now uses the mildest penalty (`variety_penalty_last`) instead of the strongest (`variety_penalty_first`).

## v0.1.118 (2025-12-31)
- **Fix**: Prevented `__IGNORED__` category from appearing in UI and logs.
- **Fix**: Handled stale legacy classification strings (`__IGNORED__`) in the `wikidata_hierarchy` table by treating them as explicitly ignored.
- **Improvement**: Updated "Landmark Rescue" logic to strictly respect the `Ignored` status of articles, preventing invisible administrative units from being surfaced as POIs.

## v0.1.117 (2025-12-31)
- **Fix**: Resolved beacon altitude regression. Balloons now spawn at the aircraft's current altitude using internal telemetry.
- **Refactor**: Simplified `BeaconService.SetTarget` interface by removing the unnecessary altitude parameter.

## v0.1.116 (2025-12-31)
- **Fix**: Resolved pause button regression. The pause button is now always enabled and toggles a global "User Paused" state that correctly halts the automatic narration scheduler even when idle.

## v0.1.115 (2025-12-31)
- **Fix**: Resolved duplicate narration bug caused by cache pointer replacement. The POI manager now performs in-place struct updates, ensuring `LastPlayed` timestamps are consistently reflected across all components (Scorer, Narrator, Manager).

## v0.1.114 (2025-12-31)
- **Feature**: Dynamic Interest Re-ingest. When the AI discovers new dynamic interests (e.g. "Steel Mill" near Pueblo), the system now forces a re-evaluation of cached map tiles ahead of the aircraft. This "rescues" entities that were previously classified as "boring" and discarded, ensuring they are now flagged as valid POIs.

## v0.1.113 (2025-12-31)
- **Feature**: Category Group Variety Penalty. Implemented configuration-driven category grouping (e.g. Natural = Mountain, Lake). If the last played POI shares a group with the current candidate, a penalty (`variety_penalty_first`) is applied, stacking with existing variety logic.

## v0.1.112 (2025-12-31)
- **Feature**: Implemented spatial merging during ingestion. Adjacent POIs (within `merge_distance`) are now grouped, keeping only the one with the longest Wikipedia article. This significantly reduces repetitive narrations (e.g. Mountain vs Complex).
- **Feature**: Persistent Map Markers. POIs narrated within the last hour now remain visible on the map regardless of the score threshold.
- **Refactor**: Locked in optimal Beacon Service settings (2ms sleep, 1s interval) for smoothness.

## v0.1.111 (2025-12-31)
- **Refactor**: Addressed linting issues (cyclomatic complexity in narrator, inefficient string ops, and code style in wikidata/config).
- **Maintenance**: General code cleanup.

## v0.1.110 (2025-12-31)
- **Refactor**: Optimized Beacon Service for smoother updates (increased frequency to 1 frame delay, reduced poll sleep).
- **Fix**: Corrected formation balloon despawn logic to trigger when target distance is < 3.0km.
- **Test**: Updated narrator tests to align with improved natural language direction logic.

## v0.1.109 (2025-12-31)
- **Feature**: Upgraded narrator direction logic to use natural language (e.g. "at your 2 o'clock", "just ahead") and predicted aircraft positions for better timing.
- **Feature**: Updated POI Score Threshold configuration to support a range of [-10, +10] allowing filtering of negative-score entities.
- **Fix**: Removed backend validation that prevented setting negative POI score thresholds.

## v0.1.108 (2025-12-31)
- **Fix**: Optimized startup validation to prevent log spam and API rate limiting issues.
- **Feat**: Implemented batching and persistent caching (MD5) for Wikidata API requests, significantly speeding up startup and reducing API load.

## v0.1.107 (2025-12-31)
- **Fix**: Fixed critical bug in category configuration loading where the `qids` field was ignored due to a struct tag mismatch (`qid` vs `qids`). This restores static category matching (e.g. for Cities).

## v0.1.106 (2025-12-31)
- **Fix**: Adjusted beacon update interval to exactly match the smooth reference implementation from 80days (every 2nd visual frame).
- **Refinement**: Verified correct SimConnect frame skipping logic (Interval=2).

## v0.1.105 (2025-12-31)
- **Refactor**: Made `DynamicConfigJob` fully asynchronous to prevent blocking the scheduler loop during LLM calls.
- **Feature**: Implemented distance (50km) and time (30min) based triggers for Dynamic Config to prevent excessive API usage.

## v0.1.104 (2025-12-31)
- **Fix**: Classifier logic now prioritizes valid categories over ignored ones (e.g. Reservoir is kept even if also Administrative).
- **Refinement**: Improved debug logging for filtered/ignored articles to aid diagnosis.

## v0.1.103 (2025-12-31)
- **Test**: Added regression test for essay coordinate shadowing bug in `BuildPrompt`.
- **Test**: Added unit tests for validator direct matching and QID extraction logic.
- **Test**: Added store persistence test for new `specific_category` POI field.
- **Coverage**: Narrator package coverage verified at ~79%.

## v0.1.102 (2025-12-31)
- **Feature**: Added `specific_category` to POI model and DB schema for more precise category labels.
- **Feature**: Dynamic config now parses `specific_category` from Gemini; uses it when category is "Generic".
- **Feature**: Frontend POI panel displays specific_category in brackets when different from category.
- **Feature**: Validator now accepts weak Wikidata search matches with warning, using result anyway.
- **Fix**: Essay coordinate regression - removed shadowing Lat/Lon fields.

## v0.1.101 (2025-12-31)
- **Fix**: Restored telemetry wiring to scheduler (frontend was showing "disconnected").
- **Fix**: Test config now uses `test_gemini.log` to avoid truncating production log.
- **Refactor**: Consolidated log truncation into `pkg/logging` (removed `log_helper.go`).
- **Feature**: Rotation-based essay selection to avoid topic repetition.

## v0.1.100 (2025-12-31)
- **Feature**: Refactored `Beacon.Service` to use an independent SimConnect frame-driven loop for smooth marker movement.
- **Feature**: Implemented explicit beacon suppression and clearing during "Regional Essay" narrations.
- **Refactor**: Decoupled `Beacon.Service` from the main `sim.Client` to avoid telemetry stream interference.
- **Fix**: Resolved potential race conditions in `AIService` by setting narration state synchronously.

## v0.1.99
- fix: suppressed regional essays while on the ground (using AGL < 50ft)
- fix: prevented back-to-back essays by doubling the subsequent cooldown interval
- fix: added random startup cooldown to prevent immediate narration on launch
- feat: robust ground detection combining SIM ON GROUND and AltitudeAGL

## v0.1.97 (2025-12-31)
- **UI**: Finalized system stats labels and layout alignment.

## v0.1.96 (2025-12-31)
- **UI**: Refined system stats labels (removed colons, added spaces, relabeled DISP/TRK/MEM).
- **UI**: Adjusted statistic cards to be top-aligned instead of vertically centered.

## v0.1.95 (2025-12-31)
- **Feature**: Added a "SHUTDOWN SERVER" button in the Configuration panel to allow remote application termination via the API.

## v0.1.94 (2025-12-31)
- **Feature**: Added historical peak memory tracking to the UI (`Current / Peak MB`) to provide better context during memory fluctuations.

## v0.1.93 (2025-12-31)
- **Fix**: Replaced mocked AGL calculation in the UI with a real `AltitudeAGL` field supplied by the simulator (SimConnect/Mock).

## v0.1.92 (2025-12-31)
- **Feature**: Suppressed "Regional Essays" while the aircraft is on the ground (PARKED, TAXIING, or HOLDING).
- **Refactor**: Extracted `POIProvider` interface in `pkg/core` to improve testability of `NarrationJob`.
- **Test**: Added comprehensive unit tests for `NarrationJob` ground suppression logic.

 ## v0.1.91 (2025-12-30)
 - **Feature**: Restored "Grey" status display for Wikidata in the API panel to indicate successful but empty POI results.
 - **Feature**: Enhanced POI Info Panel with "Last Played" timestamp and comprehensive score breakdown (Variety, Repeat, Ground, Dimension multipliers).
 - **Fix**: Improved location accuracy by passing fresh telemetry to the POI narration service, preventing `0,0` coordinates in prompts.
 - **Fix**: Resolved `PlayPOI` signature mismatch in `internal/api/narrator.go` that blocked builds.
 - **Fix**: Corrected ground suppression scoring calculation and updated `scorer_test.go`.
 - **Refactor**: Updated `Service` and `AIClient` interfaces to accept telemetry for enriched geocoding context.
 - **Refactor**: Added `GetLocation` to `GeoProvider` for consistent city/region resolution across the backend.
 - **Maintenance**: Added word-wrapping to `gemini.log` for better readability and enhanced logging for novelty boosts.
 
## v0.1.90 (2025-12-30)
- **Feature**: Added a persistent "POI Score Threshold" slider (0.0 - 1.0) in the Configuration panel to dynamically filter POIs on the map.
- **Refactor**: Extracted initialization logic in `main.go` to `setupNarrator` helper to reduce cyclomatic complexity.
- **Test**: Increased test coverage for `pkg/narrator` to >80% by adding unit tests for `EssayHandler` and integration tests for `AIService`.
- **Fix**: Resolved linting errors (gocyclo, SA9003) and verified strict lint checks pass.

## v0.1.89
- **Feature**: Implemented "Regional Essays" as a fallback narration mechanism.
    - **EssayHandler**: Manages essay topics (`configs/essays.yaml`) and ensures variety by tracking history.
    - **Narrator**: Integrated `PlayEssay` capability into `AIService`, using a new `essay.tmpl` LLM prompt.
    - **Scheduler**: Updated `NarrationJob` logic to trigger essays when no high-quality POIs are available, respecting a separate essay cooldown.
- **Config**: Added `essay` configuration section to `phileas.yaml` for tuning thresholds and cooldowns.

## v0.1.88
- **Refactor**: Removed `SimTime` from `Telemetry`, `sim.Client`, and frontend types. The application now purely uses system time (`time.Now()`) to avoid time skew issues with the simulator.

## v0.1.87
- **Fix**: Resolved `LastPlayed` timestamp reset issue in `UpsertPOI`, ensuring valid history is preserved when updating POIs.
- **Fix**: Made `UpsertPOI` robust against overwriting user metadata (`NameUser`, `TriggerQID`) with empty values from fresh fetches.
- **Fix**: Hardened Scorer to use System Time instead of Simulation Time for repeat penalty, fixing re-narration bugs due to time skew.

## v0.1.86
- **Fix**: Resolved backend panic in `beacon.Service.SetTarget` when using Mock Simulator (nil pointer dereference).
- **Refactor**: Reduced cyclomatic complexity in `main.go` by extracting logic to `log_helper.go` and `sim_helper.go`.

## v0.1.85
- **UI Improvements**: Removed redundant "System" label and improved layout of System stats in the Info Panel.
- **Log Management**: `logs/server.log`, `logs/requests.log`, and `logs/gemini.log` are now truncated at application startup based on configuration.
- **Bug Fixes**:
    - Reverted configuration changes to `categories.yaml` per user request.
    - Fixed `SimConnect Exception` spam (UNRECOGNIZED_ID) by automatically clearing AI beacons when returning to the main menu or disconnecting.

## v0.1.84 (2025-12-30)
### Features
- **Frontend**: Added "Click to Close" on map background to deselect POIs.
- **Frontend**: Added "System" stats card to Info Panel (Displayed/Tracked POIs, Backend Memory).
- **Frontend**: Improved POI name display (shows User > En > Local, with Local in brackets if different).
- **Frontend**: Added "Play" button to POI Info Panel to manually trigger narration.
- **Backend**: Updated `/api/stats` to include system memory and tracking counts.

## v0.1.83 (2025-12-30)
### Fixes
- **Narrator**: Fixed bug where POI marker vanished and playback status returned to "idle" during narration. `NarratePOI` now blocks until audio playback completes, keeping the state active.

## v0.1.82 (2025-12-30)
### Refactor
- **Audio**: Renamed `audio.AudioManager` to `audio.Manager` (and constructor `NewAudioManager` to `New`) to fix linter "stutter" warnings.
- **Audio**: Renamed `audio.Manager` interface to `audio.Service` to resolve naming conflict and better reflect its role as an abstract service definition.

## v0.1.81 (2025-12-30)
### Fixes
- **Refactor**: Completed renaming from `sim.SimState` to `sim.State` and related constants (`SimStateActive` → `StateActive`, `SimStateInactive` → `StateInactive`, `SimStateDisconnected` → `StateDisconnected`, `UpdateSimState` → `UpdateState`). Fixed 8 files with incomplete renaming that caused build failures.

## v0.1.80 (2025-12-30)
### Fixes
- **Classifier**: Fixed bug where articles in `ignored_categories` were being rescued by dimension logic. Added `Ignored` field to `ClassificationResult` to explicitly communicate drop intent.
- **Service**: Articles in "seen" cache are now filtered immediately. Ignored articles are added to "seen" cache and dropped before reaching dimension rescue.
- **Tests**: Added table-driven tests for ignored category classification (direct and cached paths).

## v0.1.79 (2025-12-30)
### Fixes
- **Scheduler**: Refactored `NarrationJob` to respect audio playback status ("Busy"). The cooldown timer now only restarts *after* audio finishes playing, preventing overlap and ensuring proper pacing.
- **Narrator**: Implemented robust POI name resolution (`NameUser` > `NameEn` > `Local` > `ID`) to prevent empty names in logs and prompts.
- **Stability**: Reverted experimental category filtering changes to maintain consistent behavior.

## v0.1.78 (2025-12-30)
### Features
- **LLM**: Implemented strict model validation at startup. If the configured Gemini model is incorrect, the application will list available models and exit, preventing runtime failures.

## v0.1.77 (2025-12-30)
### Fixes
- **UI**: Added missing `.flex-container` styles to `index.css`, fixing the portrait mode layout where telemetry cards were vertically stacked instead of wrapping side-by-side.
- **Build**: Fixed `make build-web` on Windows/PowerShell by ensuring correct path resolution. Decoupled `make test` from binary build to prevent file locking.
- **Resilience**: Increased HTTP client default timeout to 300s to accomodate slow LLM responses.
- **Config**: Added support for `GEMINI_API_KEY` environment variable to fix "client not configured" errors.

## v0.1.76 (2025-12-30)
### Fixes
- **Templates**: Fixed template rendering errors by resolving field mismatches (`Persona`, `Language`, `Country`, `Region`, `RecentContext`) in `NarrationPromptData`.
- **Testing**: Increased `narrator` package test coverage to >85% by refactoring `AIService` with interfaces and mocks.
- **Validation**: Added integration tests to validate struct fields against prompt templates.
- **Linting**: Fixed various linting issues in test files.

## v0.1.75 (2025-12-30)
### Features
- **AI Infrastructure**: Standardized `request.Client` usage for all external API calls (Wikidata, Wikipedia, Gemini). Added queueing, retries, and metrics tracking for LLM interactions.
- **Observability**: Added "API STATISTICS" to the InfoPanel, displaying real-time success/failure rates for Gemini and EdgeTTS.
- **Gemini Logging**: Implemented detailed request/response logging to `logs/gemini.log` for debugging and fine-tuning.
- **Scheduler**: Wired up `NarrationJob` in the core internal scheduler to evaluate POI narration triggers (Phase 4.3 preliminary).

### Fixes
- **Tests**: Fixed path resolution issues in `cmd/phileasgo` unit tests.
- **TTS**: Fixed file extension verification to avoid double-extensions (e.g., `file.mp3.mp3`).

## v0.1.74 (2025-12-30)
### Features
- **TTS Providers**: Implemented Windows SAPI5 (offline) and Microsoft Edge TTS (online) providers in pkg/tts.
- **Audio Resampling**: Updated AudioManager to support 48kHz resampling, enabling mixed audio formats (WAV/MP3) without speaker re-initialization.
- **Demo Script**: Added scripts/demo_tts.go for verifying audible TTS output across engines.
- **Prompt Sanitization**: Added utility to strip speaker labels from generated narration scripts.

## v0.1.73 (2025-12-30)
### Features
- **LLM Infrastructure**: Implemented a modular system for Large Language Model support.
- **Gemini Provider**: Integrated with Google Gemini models for text and JSON generation.
- **Prompt Management**: File-based prompt templates with support for macros and user configuration.

## v0.1.72 (2025-12-30)
### Refactor
- **Wikidata Service**: Refactored `postProcessArticles` to reduce complexity and improve readability.
- **Classifier**: Cleaned up `buildNextLayer` signature.

## v0.1.71 (2025-12-30)
### Fixes
- **Classification**: Fixed bug where `ignored_categories` were only checked against direct instances. Now checks the entire hierarchy up to depth 4.
- **Classification**: Ignored categories now completely disqualify an article from being rescued as a Landmark/Area/etc.

## v0.1.70 (2025-12-30)
### Fixes
- **Linting**: Fixed unchecked `json.Encoder.Encode` error returns in `internal/api/audio.go` and `internal/api/narrator.go`.

## v0.1.69 (2025-12-30)
### Features
- **Audio Playback Infrastructure**: Ported audio playback foundation from 80days Python PoC to Go.
  - **Backend**: `pkg/audio` with `AudioManager` using `gopxl/beep` for MP3/WAV playback, pause/resume, volume control.
  - **Backend**: `pkg/narrator` stub service (placeholder for future Gemini TTS integration).
  - **API**: Added `/api/audio/control`, `/api/audio/volume`, `/api/audio/status` endpoints.
  - **API**: Added `/api/narrator/play`, `/api/narrator/status` endpoints.
  - **Frontend**: `PlaybackControls` component with Replay, Pause/Resume, Stop, Skip, Volume slider.
  - **Frontend**: `useAudio` hook for state management with react-query.
  - **Styling**: Added dark-themed playback controls matching existing UI.

### Dependencies
- Added `gopxl/beep` v2.1.1 for audio playback (Windows, no CGO).
- Added `lucide-react` for frontend icons.

### Unit Tests
- `pkg/audio`: Tests for state management, volume clamping, user pause logic.
- `pkg/narrator`: Tests for stub service behavior.

## v0.1.68 (2025-12-30)
### Fixes
- **Smart Positioning**: Restored heading-based map offset. Aircraft is now positioned at bottom 25% of screen with more visibility ahead (regression from v0.1.18).

## v0.1.67 (2025-12-30)
### Features
- **Multi-Layer Visibility Overlay**: Map now shows three visibility zones (M=Red, L=Orange, XL=Yellow) with 50% reduced opacity for better readability.

### Fixes
- **Rear Visibility**: Objects behind the aircraft (90-225Â° bearing) are now fully invisible (0.0) instead of barely visible (0.1).

## v0.1.66 (2025-12-30)
### Features
- **POI Info Panel**: Replaced map tooltips with a persistent Info Panel overlay. Panel position dynamically adjusts based on aircraft heading.
- **Visibility Overlay**: Minimum opacity now 30% for visible areas, improving distinction from invisible zones.

### Configuration
- **Ignored Categories**: Added `Q51041800` (Religious administrative entity) to prevent religious dioceses from being dimension-rescued.

## v0.1.65 (2025-12-30)
### Polish
- **UI**: Updated map tooltips to use a dark theme, matching the application's overall aesthetic (Canvas-like panel background, light text, accented links).

## v0.1.64 (2025-12-30)
### Features
- **Map UI Enhancements**:
    - **Score-based Coloring**: POI markers now transition from Yellow (Score 1.0) to Red (Score 50.0).
    - **Refactored Markers**: Implemented `POIMarker` component with React Memoization to eliminate flickering.
    - **Stable Tooltips**: Disabled auto-pan to prevent UI layout shifts.

### Fixes
- **Category Lookup**: Fixed case-sensitivity bug in `getIcon` that prevented standard categories (e.g. "Castle") from displaying correct icons.

### Testing
- **Coverage**: Achieved 95% coverage in `pkg/scorer` and 71% in `pkg/visibility` with new table-driven tests.

## v0.1.63 (2025-12-30)
### Features
- **Dimension Scoring**: Implemented dimension-based score multipliers. Physically large POIs (local records or exceeding global median) now receive a visibility boost (2x or 4x).
- **Backend Classification**: Ported dimension tracking and rescue logic from Python options to the Go backend `pkg/classifier`.

### Fixes
- **Tests**: Fixed `pkg/wikidata` unit tests which were failing due to interface mismatches in the mock classifier.

## v0.1.62 (2025-12-30)
### Features
- **Scoring Engine**: Integrated the dynamic POI scoring engine (`pkg/scorer`) into the main application. POIs are now continuously scored based on visibility, distance, and content quality.

### Fixes
- **Configuration**: Fixed a bug where `categories.yaml` was being parsed as JSON instead of YAML.
- **Build & Test**: Resolved multiple linting errors and build failures in `make test`.

### Refactoring
- **Code Quality**: Major refactoring of `pkg/scorer` and `pkg/visibility` to reduce cyclomatic complexity and improve maintainability.
- **Optimization**: Optimized parameter passing (pointers) in scoring logic to avoid `hugeParam` lint warnings.

## v0.1.61 (2025-12-29)
### Fixes
- **Visibility Overlay**: Increased overlay opacity to make it clearly visible on the dark map theme.
- **Version Warning**: Fixed mismatch warning in the InfoPanel by aligning backend version string format with frontend expectations.

## v0.1.60 (2025-12-29)
### Refinement
- **Cache Strategy**: Removed the 5-minute expiration from the in-memory tile cache. Tiles are now processed exactly once per session, ensuring they are ingested correctly (from DB or Network) but never re-processed redundantly while the application is running.

## v0.1.59 (2025-12-29)
### Fixes
- **Regression**: Reverted the aggressive DB cache check that was preventing POIs from being ingested. The system now correctly loads cached tiles but avoids redundant network requests.
- **Logging**: Downgraded "Fetched and Saved" logs to `DEBUG` to eliminate console spam during map traversal.

## v0.1.58 (2025-12-29)
### Features
- **Visibility Overlay**: Implemented a dynamic "Visibility Map" overlay that visualizes what the pilot can see based on altitude, heading, and blind spots.
    - **Backend**: `pkg/visibility` calculator logic and `GET /api/map/visibility` endpoint.
    - **Frontend**: Canvas-based `VisibilityLayer` on the map with a toggle in the InfoPanel.
    - **Config**: Added `configs/visibility.yaml` for altitude-based visibility ranges.
- **Configuration Persistence**: Added persistence for `show_visibility_layer` and `show_cache_layer` settings via the API.

### Fixes
- **Tile Fetching Redundancy**: Fixed a critical bug where the `WikidataService` would redundanty re-process and re-save already-cached tiles every 5 minutes (when memory cache expired). It now checks the persistent database cache before fetching.
- **Linting**: Resolved code complexity issues in `calculator.go` and `main.go`.

## v0.1.57 (2025-12-29)
### Refined POI Resolution & Rescue
- **Categorization & Rescue Refactor**: Decoupled dimension-based rescue logic from sitelink filters. Categorized features (e.g., "Church") that fail their sitelink minimum are now **rescued** while preserving their category, rather than being dropped or genericized.
- **Improved Name Resolution**: Fixed the SPARQL query to correctly fetch the user's preferred language title (`title_user_val`) and updated `poi.Manager` to prioritize names as `User > En > Local` in logs and database storage.
- **Configurable Result Limits**: Introduced `wikidata.area.max_articles` in `phileas.yaml` (default 100) to control POI density and noise in crowded areas.
- **Sitelinks Transparency**: Added detailed debug logging in `postProcessArticles` identifying exactly why a POI was filtered (e.g., `links=2 min=5`).
- **Comprehensive Negative Cache**: Updated `seen_entities` logic to mark *all* rejected articles (including categorized items that fail sitelink filters) as seen, preventing redundant API calls on subsequent runs.
- **Fixes**: Resolved a structural lint error (`rangeValCopy`) in the article processing loop.

## v0.1.56
- feat: hook up Wikipedia stats display in the frontend `InfoPanel`
- refactor: group Wikipedia metrics under "wikipedia" provider in `request.Client`

## v0.1.55
- refactor: use `version.Version` for `User-Agent` string in `request.Client` to avoid manual updates

## v0.1.54
- refactor: move "Wikidata Request" logs to `request.Client` so they only fire on cache misses
- feat: add `DEBUG` level cache hit/miss logging for SPARQL tiles and entity metadata
- log: downgrade network request logs to `DEBUG` to reduce console noise during normal operation
- chore: update User-Agent to include correct version string

## v0.1.53
- refactor: split classifier BFS into `searchHierarchy` to reduce cyclomatic complexity
- feat: implement robust table-driven tests for classifier caching levels
- fix: optimize article marking loop in Wikidata service to avoid unnecessary copies (rangeValCopy)
- fix: resolve linting issues (hugeParam, syntax errors) in test suite

## v0.1.52 (2025-12-29)
### Performance
- **Persistent Entity Deduplication**: Added a lean "Negative Cache" (`seen_entities` table) to store QIDs of processed landmarks that weren't classified as POIs. This prevents redundant Wikidata metadata requests across server restarts and tile re-ingestion.
- **Improved Cache Integrity**: Verified intermediate hierarchy nodes are correctly persisted in `wikidata_hierarchy`.

## v0.1.51 (2025-12-29)
### Fixes
- **Area Icon (Final Fix)**: Centralized icon mapping for dimension categories ("Area", "Height", "Length") in the `poi.Manager` hydration logic. This ensures icons display correctly for both reloaded cached items and new items without needing database schema changes.

## v0.1.50 (2025-12-29)
### Performance & Reliability
- **Batch Hierarchy Traversal**: Refactored Classifier to fetch subclass data (`P279`) in batches of 50 via `wbgetentities`. This eliminates serial request patterns during hierarchy discovery, drastically reducing API round-trips.
- **Area Icon Fix**: Explicitly assigned `circle-stroked` icon to "Area" rescued articles, fixing a regression where the fallback logic failed to display the correct marker.
- **Linter Compliance**: Refactored classifier logic to reduce cyclomatic complexity and applied named returns for public batch methods.

## v0.1.49 (2025-12-29)
### Hotfix (v0.1.48 Regressions)
- **Classifier Logic Fix**: Identified and resolved a logic gap in `Classifier.slowPathHierarchy` where intermediate hierarchy nodes were fetched from the API without checking the database cache. This caused the "silent" request storm (~200 requests/batch).
- **Request Visibility**: Added `INFO` logging to `GetEntityClaims` to ensure all external data fetches are visible in server logs.

## v0.1.48 (2025-12-29)
### Debugging & Stabilization
- **Frontend / UX**:
    - **Reverted Icon Fallback**: Removed the generic fallback icon logic to ensure underlying data issues (missing icons) are visible and fixable.
    - **Aircraft Z-Index**: Fixed regression where aircraft icon was obscured by map markers by enforcing `zIndexOffset: 1000`.
- **Backend / Wikidata**:
    - **Granular Logging**: Added detailed INFO-level logging for *every* outgoing HTTP request to the Wikidata API/SPARQL endpoints to diagnose high request volumes.
    - **Spatial Deduplication**: Implemented `inflightTiles` mechanism in `wikidata.Service` to strictly prevent concurrent processing of the same tile/area.
    - **Rescue Logic**: Enhanced "Rescued" POI logic to assign specific categories and icons based on the rescue trigger:
        - **Length** â†’ Icon: `arrow`
        - **Height** â†’ Icon: `cemetery-jp`
        - **Area** â†’ Icon: `circle-stroked`
- **Backend / POI Manager**:
    - **Runtime Icon Hydration**: Updated `poi.Manager` to inject `CategoriesConfig` and hydrate the `Icon` field at runtime for POIs loaded from the database (where the icon field might be missing due to schema limitations).

## v0.1.47 (2025-12-29)
### Bug Fixes
- **Frontend**: Fixed broken POI marker icons by correctly appending the `.svg` extension to the icon path construction.

### Debugging
- **Classifier**: Added detailed debug logging to the Wikidata classifier to help diagnose excessive API requests during hierarchy traversal.

## v0.1.45 (2025-12-29)
### Bug Fixes
- **Frontend**: Fixed a regression where the aircraft icon had a white background. It is now correctly transparent.

## v0.1.44 (2025-12-29)
### Improvements
- **Backend/Deployment**: The application now embeds all icons directly into the binary via the frontend build process, removing the dependency on the external `data/icons` directory at runtime.

## v0.1.43 (2025-12-29)
### Bug Fixes
- **Frontend**: Restored the missing `AircraftMarker` component to display the aircraft icon on the map. This fixes a regression introduced in v0.1.42 where the aircraft icon disappeared.

## v0.1.42 (2025-12-29)
### Maintenance
- **Linting**: Resolved major linting issues across the codebase (`gocyclo`, `gocritic`, `errcheck`).
    - Reduced complexity in `pkg/wikidata/service.go`, `pkg/geo/geo.go`, and `cmd/phileasgo/main.go`.
    - Fixed variable shadowing in `pkg/wikipedia/client.go`.
    - Fixed unchecked errors in `pkg/poi/manager_test.go`.
- **Refactoring**:
    - **Interface Standardization**: Updated `pkg/cache.Cacher` interface and implementation to consistently use `context.Context` and specialized `GetCache`/`SetCache` methods, matching `pkg/store.Store`.
    - **Performance**: Optimized loops in `pkg/wikidata/service.go` to use pointers and avoid struct copying.
- **Frontend Fixes**:
    - Cleaned up unused imports in `Map.tsx`.
    - Restored `units` and `showCacheLayer` props in `Map` component for proper configuration support.

## v0.1.41 (2025-12-29)
### Features
- **POI Visualization**: Exposed tracked POIs via API (`GET /api/pois/tracked`) and visualized them on the frontend map.
- **Icons**: Added `Icon` field to POI model, populated from configuration, and served category icons from `data/icons`.
- **Refactor**: Ported `Wikidata` ingestion pipeline to support multi-provider architecture with a centralized `POI Manager`.
- **Strict Model**: Implemented strict POI data model with consistent naming (En/Local/User) and Wikipedia URL selection logic.

### Refactoring
- **Architecture**: Decoupled `pkg/wikidata` from direct storage access; it now uses `pkg/poi`.
- **API**: Standardized `internal/api` structure with dedicated handlers.

### Fixes
- **Frontend**: Fixed `useTelemetry` type safety and `Map` component syntax issues.

## v0.1.40 (2025-12-29)
### Performance
- **Batch Classification**: Optimized the article processing pipeline to use `wbgetentities` batching (50 entities per request).
- **POI Deduplication**: Implemented a pre-classification check against the `poi` table. Existing POIs are now loaded instantly, skipping the Wikidata API entirely.
- **Improved Storage Layer**: Added `GetPOIsBatch` to the storage layer for efficient bulk lookups.

## v0.1.39 (2025-12-29)
### Refinement
- **Clean Hierarchy Taxonomy**: Refactored the `Classifier` to exclude article instances from the `wikidata_hierarchy` table. Only taxonomy/class nodes are now cached, keeping the database schema focused on structural data.
- **On-the-fly Classification**: Articles are now classified by re-evaluating their instances against the cached hierarchy, maintaining performance without polluting the database.

## v0.1.38 (2025-12-29)
### Refinement
- **Consolidated Tracking**: Tracking is now centralized in `request.Client`, grouping all `*.wikidata.org` subdomains under a single `"wikidata"` identifier for cleaner stats.
- **Conditional Caching**: Unified caching logic into `request.Client`â€”caching now only occurs when an explicit `cacheKey` is provided by the caller.
- **Cleanup**: Stripped out manual tracking and local caching from `wikidata.Service` and `classifier.Classifier`.

## v0.1.37 (2025-12-29)
### Refinement
- **Centralized Headers**: Moved the default `User-Agent` to `request.Client` to ensure all outgoing requests identify correctly.
- **Header Flexibility**: Added `GetWithHeaders` to support custom headers while retaining resilient request features.
- **Wikidata**: Cleaned up the Wikidata client to use centralized identification.

## v0.1.36 (2025-12-29)
### Resilience
- **Wikidata Client**: Refactored to use the central resilient `request.Client`.
- **429 Handling**: Added automatic exponential backoff for Wikidata SPARQL and API requests.
- **Queuing**: Wikidata requests are now queued per-provider to respect rate limits.
- **Cache**: Integrated Wikidata client with transparent SQLite-based request caching.

## v0.1.35 (2025-12-29)
### Logging & Refinement
- **Wikidata Logging**: Added `INFO` level logs for fetched tiles, detailing raw/filtered/rescued counts.
- **Resilience**: Restored `GetCachedTiles` method and improved linter compliance across `pkg/wikidata` and `pkg/classifier`.
- **Optimization**: Fixed index-based iteration in Article processing to avoid large struct copies.

## v0.1.33 (2025-12-29)
### Features
- **Smart Classification**: Implemented recursive Category classification with caching and lazy re-evaluation.
- **Config**: Added yaml-based category configuration loader.
- **Store**: Extended database schema to persist classification results.

### Fixes
- **Linter**: Resolved formatting and named-return issues in classifier package.
- **Config**: "Show Cache Layer" setting is now persisted.
- **Defaults**: Changed default Simulation Source to **SimConnect** (falls back to Mock if failed).

## v0.1.32 (2025-12-29)
-   **Cache Layer Visualization**:
    -   **Backend**: Implemented `GetCachedTiles` and `/api/wikidata/cache` endpoint (15s TTL) to serve cached tile centers.
    -   **Frontend**: Added "Show Cache Layer" config option to InfoPanel.
    -   **Map**: Implemented `CacheLayer` component rendering cached areas as white circles (10km radius) when enabled.
    -   **Persistence**: "Show Cache Layer" setting is persisted in backend state.

## v0.1.31 (2025-12-29)
-   **Wikidata Geosearch**:
    - **Service**: Created `wikidata.Service` to orchestrate fetching, parsing (SPARQL), and caching of geospatial articles.
    - **Caching**: Integrated with `pkg/store` (SQLite) using `wd_hex_` keys.
    -   **Stats Tracking**: Added `pkg/tracker` for granular atomic metrics (Hits, Misses, API Success/Zero/Error).
-   **API Integration**:
    -   **Stats Endpoint**: Added `GET /api/stats` exposing provider-specific metrics (Wikidata).
-   **Frontend**:
    -   **InfoPanel**: Updated to display Wikidata fetch stats (Green/Grey/Red) and Cache Hit Rate.

## v0.1.30 (2025-12-29)
-   **Beacon Service**:
    -   Implemented `pkg/beacon` for visual guidance using SimConnect AI objects (Hot Air Balloons).
    -   **Target Beacon**: Spawns at destination coordinates.
    -   **Formation**: 3 balloons spawn 2km ahead of user, aligning dynamically with the target bearing.
    -   **Auto-Despawn**: Formation vanishes when within 3km of target.
-   **SimConnect Integration**:
    -   Updated `pkg/sim` and `SimConnect` client to support synchronous object spawning (`SpawnAirTraffic`) and position updates (`SetObjectPosition`).
    -   Added `cmd/simtest` PoC for validating formation logic.
-   **Testing**:
    -   Implemented `MockClient` for `pkg/beacon` unit tests, verifying logic without simulator dependency.


## v0.1.29 (2025-12-29)
-   **Map Enhancements**:
    -   **No Panning**: Disabled map dragging for focused view.
    -   **Zoom Limits**: Restricted to zoom 8-13 (~10km to ~200km visible area).
    -   **Range Rings**: Added 5, 10, 20, 50, 100 km circles with labels around aircraft.
-   **Units Configuration**:
    -   Added km/nm toggle in Configuration panel.
    -   Units persisted in backend `persistent_state` via `/api/config`.
    -   Range ring labels adapt to selected unit (km â†” nm).

## v0.1.28 (2025-12-29)
-   **SimState Machine**:
    -   **State Enum**: Added `SimState` (active/inactive/disconnected) in `pkg/sim/state.go`.
    -   **Camera State Handling**: Active states `{2,3,4,30,34}` â†’ active; Inactive states `{12,15,32}` â†’ inactive; others ignored.
    -   **Telemetry Gating**: Scheduler skips telemetry processing when state is not `active`.
    -   **API Integration**: `/api/telemetry` now returns `SimState` field.
    -   **Frontend Status**: InfoPanel displays Active/Paused/Disconnected with color-coded status dot.
-   **Test Coverage**:
    -   Added `TestUpdateSimState` (12 cases) and `TestScheduler_SkipsTelemetryWhenInactive`.

## v0.1.27 (2025-12-29)
-   **SimConnect Integration (Phase 1)**:
    -   **Direct DLL Bindings**: Implemented direct `syscall` bindings to native `SimConnect.dll` in `pkg/sim/simconnect/`.
    -   **Telemetry Reading**: Added `dll.go`, `types.go`, `client.go` with connection loop, dispatch handling, and camera state support.
    -   **Dynamic Source Selection**: `main.go` reads `sim_source` from persistent state to select SimConnect or Mock client.
    -   **Conditional Tests**: Added `client_test.go` that skips gracefully when sim is unavailable.
-   **Build Improvements**:
    -   **Makefile**: Added `BIN_PATH` variable; DLL copied to bin folder on build.

## v0.1.26 (2025-12-29)
-   **Maintenance**:
    -   **Linting**: Fixed significant linting issues (`gocritic`, `gocyclo`, `revive`) to satisfy `make test`.
    -   **Refactoring**: Reduced Complexity in `importMSFS` and `TestSQLiteStore`.
    -   **Performance**: Optimized `Telemetry` passing (`hugeParam` fix) in scheduler and API.
    -   **Fixes**: Resolved regex, octal literal, and resource leak warnings.

## v0.1.25 (2025-12-29)
-   **UI Polish**:
    -   **Coordinates**: Fixed layout (centered, side-by-side) and typography (Small labels, Big values). Limited precision to 2 decimal places.
    -   **Statistics**: Refactored into a 3-column grid layout with boxed counters.
    -   **Responsiveness**: Handled portrait mode layout for all new cards.
    -   **Connection Banner**: Added connection warning banner.

## v0.1.24 (2025-12-29)
-   **UI Final Polish**:
    -   **Consolidation**: Merged HDG/GS into Altitude card; removed superfluous labels ("Alt", "Coords", "Statistics").
    -   **Collapsible Config**: Configuration section is now collapsed by default to save space.
    -   **Spacing**: Reduced vertical padding in cards.

## v0.1.23 (2025-12-29)
-   **Responsive UI**: Implemented portrait layout support (Map Top, Dashboard Bottom) and fixed media queries.
-   **UI Refinement**:
    -   **Labels**: Shortened to HDG, ALT, Coords.
    -   **Typography**: Styled Altitude (AGL/MSL) and Coords.
    -   **Statistics**: Added Wikidata/Wikipedia mock data in a 2-column grid.
    -   **Version**: Styled compact card.

## v0.1.22 (2025-12-29)
-   **UI Fix**: Fixed CSS alignment issues in dashboard cards (Statistics, Configuration).

## v0.1.21 (2025-12-29)
-   **UI Polish**: Adopted consistent card-based desing for Statistics and Configuration sections on the dashboard.

## v0.1.20 (2025-12-29)

**Features:**
- **Dashboard API**: Implemented `GET /api/version` to expose backend version.
- **Frontend Expansion**:
  - **Metrics**: Added "Statistics" section (stubbed).
  - **Config**: Added "Modification" section for Sim Source (stubbed).
  - **Consolidation**: Merged Heading/Speed into "Flight Data" card.
  - **Version Check**: Displays "Frontend: vX / Backend: vY" with warning on mismatch.

## v0.1.19 (2025-12-29)

**Features:**
- **Performance**: Adjusted default loop rates for better stability.
  - Map/Scheduler Loop: 1Hz (was 10Hz).
  - Dashboard Polling: 2Hz (Unchanged).
- **Map**: Removed smooth panning animation to prevent flickering/jitter during flight.

## v0.1.18 (2025-12-29)
-   Feature: Frontend Map Refinement (Smart Positioning, Gold Aircraft Icon).
-   Asset: Added standard POI icon set from 80days project.
-   Fix: Test suite now uses dynamic ports to avoid conflicts with running server.

## v0.1.17 (2025-12-29)
**Features:**
- **Maintenance Module**: Introduced `pkg/db/maintenance` to handle startup tasks (MSFS Import, Cache Pruning).
- **Configuration**: Made API server address configurable via `phileas.yaml` (default `localhost:1920`).

**Refactoring:**
- **pkg/db**: Renamed `pkg/data` to `pkg/db` for clarity.
- **Cleanup**: Removed now-obsolete `pkg/importer`.

**Test Coverage:**
- **New Tests**: Added comprehensive integration tests for `pkg/db/maintenance`.
- **Fixes**: Resolved extensive import paths and variable shadowing issues in test suite.

## v0.1.15 (2025-12-28)
**Features:**
- **Database Wiring**: Fixed regression where database initialization was missing from application startup (`cmd/phileasgo`).
- **MSFS Import**: Enabled automatic MSFS POI import (`pkg/importer`) on startup.
- **Config**: Added default database path (`./data/phileas.db`) to `phileas.yaml`.

**Quality Assurance:**
- **Missing Tests**: Added unit tests for `pkg/logging` and `pkg/model`, achieving 100% package coverage.
- **Component Tests**: Added integration tests for `main.go` wiring (`TestRun`).
- **Fixes**:
  - Resolved file lock resource leaks in `pkg/cache`, `pkg/wikidata`, and `pkg/request` tests (Windows compatibility).
  - Refactored `logging.Init` to return a cleanup function for proper file handle release.

## v0.1.14 (2025-12-28)
**Features:**
- **MSFS POI Import**: Implemented `pkg/importer` to populate the `msfs_poi` table from `data/Master.csv`.
  - Tracks file modification time in `persistent_state`.
  - Automatically runs import on startup if file is updated or table is empty.
- **Bug Fixes**: Resolved unit test build failures (`data.New` vs `data.Init`) ensuring CI/CD passes purely.

## v0.1.13 (2025-12-28)
**Features:**
- **Database Layer**: Implemented SQLite backend using `modernc.org/sqlite` (CGO-free).
  - **Schema**: Tables for `poi`, `msfs_poi`, `wikidata_hierarchy`, `wikipedia_articles`, `persistent_state`, and `cache`.
  - **Repository**: `pkg/store` implements the Repository Pattern to decouple business logic from SQL.
  - **Models**: Pure Go structs in `pkg/model`.
  - **Configuration**: DB path configurable via `db.path` (default: `./data/cache.db`).

## v0.1.12 (2025-12-28)
**Features:**
- **Request Client Infrastructure**: Implemented a robust HTTP client architecture (`pkg/request`) for managing external API interactions.
  - **Queueing**: "One Queue Per Provider" system to enforce sequential requests and respect rate limits per domain.
  - **Resilience**: Built-in exponential backoff for `429 Too Many Requests` and `5xx` server errors.
  - **Metrics**: `pkg/tracker` tracks cache hits/misses and API success rates per provider.
  - **Stubs**: Created foundation for `pkg/cache` (SQLite), `pkg/data`, and `pkg/wikidata`.

## v0.1.11 (2025-12-28)
**Features:**
- **Logging Refinement**: HTTP Request logs are now removed from stdout/console and written ONLY to `logs/requests.log` to reduce noise. Server lifecycle logs (startup/shutdown/error) continue to output to both console and `logs/server.log`.

## v0.1.10 (2025-12-28)
**Features:**
- **Structured Logging**: Implemented `pkg/logging` using Go's `log/slog`.
  - **Server Logs**: Written to `logs/server.log` (Default Level: DEBUG).
  - **Request Logs**: HTTP requests written to `logs/requests.log` (Default Level: INFO).
  - **Format**: Text/Key-Value format (e.g., `time=... level=INFO msg=...`).
  - **Configuration**: configurable via `log.server` and `log.requests` in `configs/phileas.yaml`.

## v0.1.9 (2025-12-28)
**Features:**
- **Configuration**: Implemented `pkg/config` using `gopkg.in/yaml.v3` to load/save `configs/phileas.yaml`.
  - Automatically initializes the file with defaults if missing.
  - Automatically adds new keys with default values to existing files on startup.
  - Current config keys: `tts.engine` (default: "windows-sapi").

## v0.1.8 (2025-12-28)
**UX Improvements:**
- **Split Layout**: Moved to a "Classic Dashboard" layout with a dedicated sidebar for telemetry, preventing the UI from obscuring the map.
- **Visuals**: refined sidebar contrast and card styling.

## v0.1.7 (2025-12-28)
**UX & Dev Experience:**
- **Web UI**: Redesigned the dashboard as a transparent, floating HUD (Head-Up Display) with separate widgets for improved visibility and less occlusion of the map.
- *(Note: Simulation timings remain at default 120s for PARKED state).*

## v0.1.6 (2025-12-28)
**Buf Fixes:**
- **Frontend**: Fixed `QueryClientProvider` missing error by wrapping the app in `main.tsx`.

## v0.1.5 (2025-12-28)
**Features:**
- **Mock Web Interface**: Implemented a responsive flight tracking dashboard.
  - **Map**: Dark-themed interactive map using Leaflet and CartoDB Dark Matter tiles.
  - **Live Tracking**: Real-time aircraft position updates every 500ms via `GET /api/telemetry` polling.
  - **Aircraft Marker**: Animated yellow plane icon that rotates to match heading.
  - **Info Panel**: Floating overlay displaying altitude (AGL/MSL), speed, heading, and flight stage.

## v0.1.4 (2025-12-28)
**Features:**
- **Frontend Dependencies**: Installed `leaflet`, `react-leaflet`, and `@types/leaflet` (v0.1.4).

## v0.1.3 (2025-12-28)
**Features:**
- **Graceful Shutdown**: Implemented signal handling (SIGINT/SIGTERM) in the main server (`cmd/phileasgo`). The application now intercepts interruption signals, gracefully shuts down the HTTP server, and ensures the simulation client is closed properly before exiting.
- **Shutdown API**: Added `POST /api/shutdown` endpoint to trigger server shutdown remotely.
- **Shutdown Script**: Added `shutdown_server.ps1` helper script for users.

## v0.1.2 (2025-12-28)
**Features:**
- **Telemetry API**: Implemented `GET /api/telemetry` endpoint serving real-time simulation data.
- **Test Coverage**: Achieved 100% file coverage with Table-Driven Unit Tests for all packages (`api`, `sim`, `mocksim`, `ui`, `version`, `cmd`).
- **Build System**: Updated `Makefile` to run tests automatically before building backend binaries (`make all`).

## v0.1.1 (2025-12-28)
**Features:**
- Implemented `MockClient` for simulating aircraft telemetry.
  - State machine with PARKED (120s), TAXI (120s), HOLD (30s), and AIRBORNE logic.
  - Automated scenario-based altitude changes (loops climb/cruise/descent).
  - Physics loop iterating every 100ms.
- Added `bin/mocksim.exe` standalone executable for testing simulation logic.
- Added `build-mocksim` Makefile target.
- Refactored `pkg/sim/mock` to `pkg/sim/mocksim` for clarity.
