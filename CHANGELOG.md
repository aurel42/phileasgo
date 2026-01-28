Release History

## v0.3.54 (2026-01-28)
- **Feature**: **LLM Timeouts**. Added per-provider timeout configuration (90s default, 30s for Groq) to prevent API stalls during high latency or service outages.
- **Improved**: **Failover Resilience**. The failover mechanism now proactively cancels hanging requests and switches to faster providers immediately upon timeout.
- **Fix**: **Request Body Exhaustion**. Fixed a bug in the HTTP client that prevented successful retries of POST requests when the first attempt failed.

## v0.3.53 (2026-01-28)
- **Feature**: **Spatial Median Rescue**. Implemented a dynamic rescue strategy that uses local neighborhood dimensions (median height/length/area) to identify significant POIs in remote regions.
- **Refactor**: **Stateless Wikidata Pipeline**. Rebuilt the processing engine as a pure function, decoupling spatial statistics from the core processing logic for better testability and reliability.
- **Fix**: **Classifier Cache Poisoning**. Resolved a critical "deadend" bug in the hierarchy traversal that caused redundant API calls and incorrect caching for unclassified entities.
- **Improved**: **Spatial Awareness**. The application now maintains a 20km "spatial memory" of surrounding tiles to provide context for landmark significance thresholds.

## v0.3.52 (2026-01-28)
- **Performance**: **Streaming JSON Decoder**. Replaced the entire Wikidata SPARQL parsing engine with a streaming decoder. drastically reducing memory spikes (approx -40% peak RAM usage) during tile loading.

## v0.3.51 (2026-01-28)
- **Optimization**: Reduced memory churn (~1MB/s) via Gzip pooling and string optimizations.
- **Cleanup**: Removed temporary profiling instrumentation.

## v0.3.50 (2026-01-28)
- **Fix**: **Replay Deadlock**. Resolved a critical bug where triggering "Replay" during narrative generation could permanently stall the playback queue.
- **Fix**: **International Waters**. Fixed spurious "International Waters" location display when the simulator provided invalid coordinates during initialization (0,0).
- **Refactor**: **Narrator Architecture**.
    - Transited the audio engine to a purely event-driven architecture, removing polling loops for better performance and reliability.
    - Centralized playback lifecycle management to prevent race conditions between "Stop", "Next", and "Replay" actions.
    - Improved test stability for the narrator pipeline.
- **Note**: Version `v0.3.49` was skipped during the release process.

## v0.3.48 (2026-01-27)
- **Feature**: **Correct Screenshot Coordinates**. "Visual Analysis" map markers now persist at the exact location where the photo was taken, rather than following the moving aircraft.
- **Fix**: **Border Announcement Persistence**. Resolved an issue where border crossing announcements would hide the active POI marker.
- **Testing**: **Improved Stability**. Fix test suite reliability for narrator logic.

## v0.3.47 (2026-01-27)
- **Dynamic Narration Length Scaling**: Implemented a sophisticated 3-phase scaling system.
    - **Intelligent Prose Processing**: New `articleproc` engine filters Wikipedia prose to count only meaningful words, ignoring metadata/lists.
    - **Adaptive Prompts**: Narration length instructions are now dynamically injected based on user settings (Short/Medium/Long) and source article density.
- **Narrative Polish**:
    - **Unified "Photograph" UX**: Refactored screenshot handling. Screenshots are now treated as "Photograph Analysis" POIs, sharing the same unified UI as Landmarks.
    - **Prompt Refactoring**: Major cleanup of prompt templates (`script_full.tmpl`, `script_stub.tmpl`) for better maintainability and context injection.
    - **Respect for Pause**: Narrator now checks for user pause state before queuing new items.
- **Bug Fixes**:
    - Reduced log noise by downgrading verbose narrative queue logs to DEBUG.
    - Fixed marker visibility regression.

## v0.3.46 (2026-01-27)
- **Feature**: **Split POI Visibility Stats**. The UI now separates "Competitive" POIs from "Recently Played" POIs in the dashboard footer for better visibility into available content.
- **Improved**: **Stub Narration Instructions**. Wikipedia stubs now receive a simplified, fact-focused instruction set that skips complex aerial identification and interest-matching logic.

## v0.3.45 (2026-01-27)
- **Fix**: **Score Breakdown Stability**. Ensures POI score details remain persistent in the UI while the narrator is generating or playing a script, even if the aircraft moves out of range.
- **Improved**: **Narration Style**. Discouraged the AI from using cliché filler phrases like "rich history" through updated system prompts.

## v0.3.44 (2026-01-27)
- **Fix**: **Urgent & Patient Badges**. Resolved an issue where time-sensitive badges (Urgent/Patient) were suppressed and would hide the main marker icon when active.
- **Improved**: **POI Discovery**. Lowered Wikipedia sitelink thresholds for several categories (Religious, Transit, Settlements) to improve landmark density in less populated areas.

## v0.3.43 (2026-01-27)
- **Refactor**: **Scoring Loop Architecture**
  - **SRP Compliance**: Extracted the core POI scoring logic from the monolithic `poi.Manager` into a dedicated, testable `ScoringJob`.
  - **Logic**: The scoring loop now runs as a standard `core.Job` within the unified Scheduler, strictly enforcing the 5-second interval.
  - **Safety**: Replaced ad-hoc tickers with atomic locking and centralized error handling.

## v0.3.42 (2026-01-27)
- **Refactor**: **Narrator Queue Modularization**
  - **Architecture**: Decoupled `Playback` and `Generation` queues from the main `AIService`, moving them into dedicated, testable managers (`pkg/narrator/playback`, `pkg/narrator/generation`).
  - **Stability**: Enforced stricter thread-safety and simplified queue logic, reducing the risk of race conditions during high-load scenarios.

## v0.3.41 (2026-01-27)
- **Feature**: **Smart Map Label Positioning**
  - **Dynamic Alignment**: Range ring labels now align with the aircraft heading to stay "ahead" of the plane.
  - **Visibility Logic**: Implemented decluttering logic to show only the largest visible label.
  - **Refinement**: Labels are now offset inside the ring to prevent line intersection, and the "space" has been removed ("10nm").
- **Fix**: **Overlay Map Units**
  - **Consistency**: The streaming overlay map now respects the global unit setting (KM/NM) instead of being hardcoded to Nautical Miles.

## v0.3.40 (2026-01-26)
- **UI**: **Stats Grid Layout Fix**
  - **Sizing**: Increased minimum column width to `130px` and reduced header/number font sizes (14px/18px) to safely accommodate larger values (e.g. 4-digit counts) within the Victorian frame.

## v0.3.39 (2026-01-26)
- **Refactor**: **Template Macro Standardization**
  - Unified all Narrator Prompt Template macros to **PascalCase** (`Persona`, `Style`, `FlightData`, `LengthInstructions`).
  - Ensures consistent naming conventions across the entire prompt library (`common/`, `narrator/`, etc.) and eliminates snake_case inconsistencies.
  
## v0.3.38 (2026-01-26)
- **Feature**: **Stub Article Detection & Handling**
  - **Logic**: Automatically detects "Stub" Wikipedia articles (short content < 2000 chars) to prevent hallucinations or repetitive filler text.
  - **Narrator**: Switches to a "Summary-Only" prompt for stubs, instructing the model to generate a concise 1-2 sentence overview without trying to fill a word count.
  - **UI**: Added a "Puzzle Piece" badge (🧩) to map markers for identified stubs, distinct from "Deep Dive" articles.
- **UI**: **Victorian Theme Polish**
  - **Styling**: Implemented a "Victorian Frame" aesthetic across all popup cards, HUD elements, and overlay panels, featuring double-borders and inset gold glows.
  - **Layout**: Converted the API Stats panel to a responsive CSS Grid layout to prevent text wrapping and align critical metrics.

## v0.3.37 (2026-01-26)
- **UI**: **Modernized Typography & Styling**
  - **Settings Panel**: Updated to use the new "Phileas" typography system, fully removing legacy CSS dependencies.
  - **Overlay Polish**: Refined Telemetry Bar with clearer units (`deg`/`kts`), rigid grid alignment, and consistent font usage.
  - **Technical Debt**: Removed significant legacy CSS code (`index.css`), simplifying the stylesheet architecture.

## v0.3.36 (2026-01-26)
- **Refactor**: **Visibility Logic**
  - Reduced cyclomatic complexity in `internal/api/visibility.go` for better maintainability (splitting `calculateEffectiveAGL`).
- **UI**: **API Stats Polish**
  - Refined the API statistics display in the Info Panel with a diamond separator (`◆`) and aligned typography.
- **Testing**: **Telemetry Validity**
  - Added rigorous tests for SimConnect telemetry validity flags to ensure robust data handling.

## v0.3.35 (2026-01-26)
- **Feature**: **Victorian Steampunk Aesthetic (Phileas Fogg Edition)**
  - **Typography**: Adopted a strict 3-font system (IM Fell English SC, Crimson Pro, Cutive Mono) for a consistent 19th-century explorer feel.
  - **Styling**: Introduced `role-label` (Cursive) for interface labels and refined button styles to match the "Brass and Parchment" theme.
  - **Colors**: Transitioned to Deep Coal backgrounds with Aged Parchment text and Brass accents.
- **UI**: **Config Pill Polish**
  - **Sim Status**: Integrated simulation connection state (ACT/PAUSED/DISC) directly into the floating config pill.
  - **Mode Clarity**: Corrected display logic to show "Target Count" in Adaptive Mode and "Min Score" in Fixed Mode.
  - **Clean Dashboard**: Removed redundant status lines from the main dashboard to reduce visual clutter.
- **Fix**: **Map & Marker Synchronization**
  - **Throttling**: Centralized the update loop for the Map, Aircraft Marker, and Auto-Zoom to a synchronized 2-second interval.
  - **Impact**: Eliminates jitter and "formation drift" where the map and plane icon moved at different rates.
- **Fix**: **Visual Regressions**
  - Restored grid alignment for altitude readouts.
  - Fixed active states for Long/Short narration length buttons.
  - Corrected Autozoom toggle styling in the map overlay.

## v0.3.34 (2026-01-26)
- **Markers**: **Optimized Badge Layout**: Increased badge offsets for better visual separation and clarity in high-density areas.
- **Markers**: **Status Visibility**: Restored deferred, urgent, and patient status indicators and implemented dynamic alternation logic.
- **Fix**: **Legacy Code cleanup**: Permanently removed defunct marker components to ensure design consistency.

## v0.3.33 (2026-01-26)
- **Scoring**: **Dynamic Filtering**: Added intelligent skip logic for recently played POIs to maintain a clean map interface.
- **Markers**: **Fixed Badge Consistency**: Restored the Globe (🌐) and Blue Gem (💎) symbols and ensured quality indicators (Deep Dive) remain visible on recently played POIs.
- **Markers**: **Refined Layout**: Corrected placement of status indicators and implemented smooth badge alternation for overlapping states (Urgent, Patient, Deferred).
- **Cleanup**: **Technical Debt**: Removed legacy code and stale comments across UI and Scorer components.
 
## v0.3.32 (2026-01-25)
- **Feature**: **Urgency-Based Narration Sequencing**: Implemented a dynamic prioritization system that balances POI raw score against visibility time ("Time-to-Obscurity"), ensuring fleeting landmarks are not missed.
- **Narrator**: **Competitive Lookahead**: The selection engine now evaluates the top visible candidates and can intelligently swap to a slightly lower-scoring but much more urgent target.
- **Markers**: **New Status Badges**: Added ⏩ (Urgent) and ⏪ (Patient) indicators to map markers to visualize the new sequencing logic.

## v0.3.31 (2026-01-25)
- **Map**: **Autozoom Behavior Refined**: Added a launch grace period to prevent accidental deactivation and restricted manual overrides to zoom events only.
- **Fix**: **Graceful Shutdown**: Corrected a GUI-to-Server signaling issue that prevented the background process from terminating cleanly on exit.

## v0.3.30 (2026-01-25)
- **Feature**: **Dynamic Badge System**: Introduced visual markers on the map to identify "Fresh" (novel), "Deep Dive" (long-form content), and "Deferred" (pending) POIs.
- **Map**: **Autozoom Control**: Copied autozoom feature from overlay to PhileasGUI; includes a compact map-level ON/OFF selector and automatic manual-override detection.
- **Marker**: **Refined Layout**: Optimized badge positioning and collision avoidance for POI markers to ensure readability in dense areas.

## v0.3.29 (2026-01-25)
- **UI**: **Restored Config Pill**: Brought back the simulation status indicator in the dashboard with a direct shortcut to Settings.
- **Geography**: **Improved Boundary Detection**: Fixed a critical race condition in the geolocation service that caused small landmasses (like islands) to be occasionally missed during reordering.
- **Geography**: **Cleaned Up Display**: Suppressed numeric placeholder codes in the region field for a cleaner dashboard in island territories.
- **Improved**: **Code Hygiene**: Resolved React Hook violations and pre-existing linting errors in UI components.

## v0.3.28 (2026-01-25)
- **Installation**: Migrated default border data to 1:50m and integrated the setup into the primary `install.ps1` workflow.
- **Improved**: **Border Crossing Precision**: Detection error for river-based borders (like the Rhine) reduced by over 5km, enabling high-precision territory notifications.

## v0.3.27 (2026-01-25)
- **Fix**: **Explorer Application Icon**: Resolved a Windows caching issue where the new icon failed to appear in Explorer; the build system now forces resource regeneration.
- **Improved**: **Makefile Hardening**: Added explicit dependencies for `.syso` resource files to ensure all future changes to application icons are automatically detected and built.
- **Git**: **Hygiene**: Added `*.syso` to `.gitignore` to prevent compiled resource files from being accidentally committed.

## v0.3.26 (2026-01-25)
- **NEW: PhileasGUI** - A native Windows application that provides a complete desktop experience:
  - **Terminal Tab**: Displays startup logs and handles first-run prerequisites (auto-runs `install.ps1`).
  - **App Tab**: Hosts the interactive map and dashboard in a native window.
  - **Config Tab**: Dedicated settings page for all configuration options.
  - **Single Instance Lock**: Prevents multiple copies from running simultaneously.
  - **Clean Shutdown**: Automatically terminates the backend server when the window is closed.
- **Settings UI**: Extracted configuration controls into a dedicated `/settings` page (Settings Panel).
- **Simplified Dashboard**: Removed configuration clutter from the main InfoPanel.
- **Release Pipeline**: Updated `Makefile` to build and include `phileasgui.exe` in binary releases.

## v0.3.25 (2026-01-25)
- **Feature**: **Real-Time UI Synchronization**: The browser app now synchronizes configuration changes (frequency, length) in real-time when updated via cockpit transponder codes, without requiring a page reload.
- **Fix**: **Cockpit Pause Visibility**: Setting the transponder frequency to 0 now correctly displays the "PAUSED" state in the web UI playback controls and triggers a global pause on auto-narration.
- **Improved**: **Narrator Status API**: Enhanced backend status reporting to include active configuration and robust pause state tracking.

## v0.3.24 (2026-01-25)
- **Feature**: **Transponder-Based Control**: Control narration frequency, narrative length, and visibility range directly from the cockpit using squawk codes (7xxx).
- **Feature**: **Transponder IDENT Action**: Link the aircraft's IDENT button to a configurable action, such as skipping the current narration.
- **Control**: **Narrator Playback Controls**: Added high-level pause, resume, and skip functionality to the narrator service, enabling unified control from both the web UI and cockpit hardware.

## v0.3.23 (2026-01-25)
- **Fix**: **Resilient Country Context**: Resolved an issue where the UI and Overlay failed to display the current country in wilderness areas without local city metadata.
- **Fix**: **Wilderness Transit Suppression**: Implemented a proximity-based filter for region transit announcements; border crossings between states/provinces are now suppressed in remote areas unless a known city is nearby.
- **Narrator**: **Unified Prompt Metadata & Robustness**: Consolidated common prompt data (language, persona, TTS instructions) into a reusable pipeline, ensuring consistent narration context.

## v0.3.22 (2026-01-24)
- **Feat**: Platform-aware release filenames (e.g. `windows-x64`) for better distribution.
- **Refine**: Improved maritime border logic to ignore EEZ and territorial waters, only announcing international water crossings.
- **Tuning**: Significantly boosted POI visibility distances at low to mid-altitudes (1,500ft - 6,000ft).
- **Fix**: Adjusted "Behind Check" logic to correctly handle POIs passed at close range.

## v0.3.21 (2026-01-24)
- **Feature**: **Smart Deferral Refinement**
  - Updated the POI deferral logic to intelligently distinguish between short-term (1-3m) and mid-term (5-15m) proximity.
  - **Logic**: Penalty is now only applied if the aircraft is predicted to be significantly closer (>25%) in the mid-term (5-15m) compared to the best short-term (1-3m) position.
  - **Horizons**: Added prediction support for **+15 minutes**, expanding the planning window.
  - **Fix**: Resolved "Behind Check" logic where passing a POI at t=1 could incorrectly invalidate future valid approach angles.

## v0.3.20 (2026-01-24)
- **Fix**: **Classification Priority Logic**
  - Resolved a bug where `ClassifyBatch` incorrectly classified POIs as `__IGNORED__` if an ignored instance (e.g. "County Seat") was encountered before a valid one (e.g. "City").
  - The classifier now correctly evaluates all instances, strictly prioritizing valid category matches over ignored ones.

## v0.3.19 (2026-01-24)
- **Optimization**: Implemented "Proximity-Aware Country Lookup" to reduce steady-flight border checks from O(N) to O(1).
- **Performance**: Upgraded `CountryService` cache to use multi-slot coordinate quantization (~1km tiles) for reliable, jitter-free lookups.
- **Fix**: Resolved structural issues in `CountryService` to ensure stability.

## v0.3.18 (2026-01-24)
- **Configurable Border Cooldowns**: Added new settings to manage the frequency of border crossing announcements, preventing repetitive notifications.
- **Fixed Spurious Border Warnings**: Resolved a coordinate-caching issue that caused false country-change alerts near international boundaries.
- **Improved System Reliability**: Optimized core geographic resolution logic to reduce complexity and resolve potential nil-pointer exceptions.

## v0.3.17 (2026-01-24)
- **Feature**: **Dual-Context Cross-Border Display**
  - **INTRODUCED** a persistent dual-context display in border regions.
  - The UI now explicitly shows "near [City], [Region], [Country]" alongside a separate "in [Legal Country]" indicator when they differ.
- **Fix**: **Border Crossing Detection**
  - Resolved an issue where border announcements failed to trigger in rural areas.
  - Decoupled detection logic from city names, ensuring crossings are detected immediately regardless of local metadata availability.
- **Narrator**: **Border Profile Automation**
  - Added a dedicated "border" LLM profile to `phileas.yaml`.
  - Border announcements are now correctly routed to specific models (e.g. `groq/compound-mini`).
- **Geo**: **Remote Area Location Polish**
  - Improved `GetLocation` to return clean results in remote areas, avoiding "near Unknown" in favor of just legal country context where appropriate.

## v0.3.16 (2026-01-24)
- **Narrator**: **Refined Adaptive POI Filtering**
  - Recently played ("blue") POIs are now excluded from the adaptive threshold calculation.
- **UI**: **Adaptive Mini-Map Zoom**
  - Added an adaptive zoom mechanism for the overlay mini-map that provides the most detailed view of playable POIs.
  - The map intelligently fits all "non-blue" POIs and the aircraft within the viewport while maintaining a persistent forward-looking offset.

## v0.3.15 (2026-01-24)
- **UI**: **Visualization & Branding Refinement**
  - Introduced a configuration pill.
  - Improved dashboard layout by integrating the flight stage indicator (AIR/GROUND) into the Position card.
- **Testing**: **Coverage & Quality**
  - Resolved TypeScript build failures caused by unused/shadowed variables during UI refactoring.

## v0.3.14 (2026-01-24)
- **Fix**: **International Date Line Crossing**
  - Resolved a critical bug where the Wikidata tile scheduler stopped discovering POIs when flying near the International Date Line (±180° longitude).
  - **Root Cause**: `DistKm()` and `calculateBearing()` did not normalize longitude differences, causing tiles across the date line to appear ~40,000 km away instead of a few km.
  - **Solution**: Added longitude normalization to [-180, 180] in both functions.
- **Testing**: Added comprehensive test cases for dateline crossing scenarios.
- **Tooling**: **Natural Earth GeoJSON Download Script**
  - Added `cmd/slim_geojson/download.ps1` to automate downloading and slimming country boundary data.
  - Integrated into Makefile as a file-target dependency for `build-app`.

## v0.3.13 (2026-01-23)
- **Feature**: **Wind-Corrected Ground Track**
  - Implemented automatic path-over-ground calculation using a 5-second rolling position buffer.
  - The `Heading` field now intelligently transitions between nose direction (on ground) and actual track (airborne), ensuring POI scoring and "Look-Ahead" maps remain accurate even in strong crosswinds.
  - Encapsulated logic in a new, unit-testable `pkg/geo/TrackBuffer` component.
- **UI**: **Dashboard Geography Integration**
  - Added human-readable location info ("near City", Region, Country) to the browser app's POSITION card, matching the OBS Overlay's rich data.
  - Refined dashboard typography and color palette for better consistency with the premium "Phileas" aesthetic.
- **Testing**: **Robustness & Parity**
  - Added end-to-end integration tests in `mocksim` to verify reliable switching between ground and airborne heading modes.

## v0.3.12 (2026-01-23)
- **Feature**: **Border Crossing Narrations**
  - Added support for automatic border crossing announcements when flying between countries or states/provinces.
  - Implemented `BorderJob` for efficient detection of geographic boundary transitions.
  - Integrated standard "International Waters" (XZ) handling for transoceanic flights.
- **Narrator**: **Enhanced Border Context**
  - Expanded `border.tmpl` context to include `TripSummary` and `TTSInstructions`.
  - Optimized queue limits to handle border narrations as distinct high-priority events.
- **Testing**: **Robustness & Coverage**
  - Resolved a long-standing non-deterministic race condition in the narrator's pipeline flow tests.

## v0.3.11 (2026-01-23)
- **Feature**: **Multiple Screenshot Paths**
  - Added support for monitoring multiple directories for new screenshots simultaneously.
  - Useful for users alternating between standard MSFS screenshots and VR-specific folders (e.g. OpenXR Toolkit).
  - Updated `phileas.yaml` with example configurations for multiple paths.
- **API**: **Multi-Path Image Serving**
  - Enhanced the ImageHandler to validate and serve images from any of the configured screenshot directories.
- **Testing**: **Coverage Improvements**
  - Added comprehensive tests for categories, interests, and environment secrets overrides.
- **Refactor**: Improved robustness of the configuration loader and screenshot monitoring logic.

## v0.3.10 (2026-01-23)
- **Feature**: **Multi-Horizon POI Deferral**
  - Implemented an intelligent deferral mechanism that predicts the aircraft's position at +1, +3, +5, and +10 minutes.
  - POIs are deferred (0.1x score multiplier) if they will be significantly closer (25% improvement) within the next 10 minutes.
  - Prevents premature narration of distant landmarks that the aircraft is about to fly over.
  - Added visibility for deferrals in the per-POI `ScoreDetails` for transparency in the UI.
- **Refactor**: Added `geo.NormalizeAngle` utility to simplify angle difference calculations.
- **Testing**: Added `pkg/scorer/deferral_test.go` with comprehensive flight scenarios.

## v0.3.9 (2026-01-23)
- **Fix**: **Ignored Category Propagation**
  - Resolved a persistent bug where POIs from ignored categories (e.g., dioceses, administrative entities) slipped through classification.
  - **Root Cause**: When BFS found an ignored category at depth 2+, only the article QID was marked as `__IGNORED__`, not the intermediate hierarchy nodes.
  - **Solution**: `searchHierarchy` now propagates `__IGNORED__` to ALL traversed nodes in the BFS path, preventing stale cache entries from bypassing ignore rules.
- **Testing**: Added "Ignored Category (Depth 2 with Propagation)" test case to verify the fix.

## v0.3.8 (2026-01-23)
- **Feature**: **Smart LLM Backoff**
  - Implemented an incremental skip strategy for transient errors (like 429).
  - Providers are skipped for $N$ subsequent requests after $N$ failures, reducing overhead on exhausted quotas.
- **Reporting**: **Enhanced Narration Stats**
  - Added actual generation time (`acc_gen_time`) and predicted duration (`next_prediction`) to narration logs.
- **Refactor**: **Log Hygiene**
  - Downgraded "Trip summary updated" logs to `DEBUG` to reduce distraction in standard logs.
- **Fix**: **Test Stability**
  - Resolved a race condition in the narrator test suite's pipeline flow verification.


## v0.3.7 (2026-01-23)
- **Feature**: **Configurable History Logging**
  - Added support for enabling/disabling LLM and TTS history logs via configuration.
  - History log paths are now configurable, allowing users to move them outside the default directory.
- **Fix**: **LLM Failover Robustness**
  - Refined the failover provider to treat 400 (Bad Request) and 429 (Too Many Requests) as recoverable errors, preventing unintended provider disabling.
  - Improved retry logic for exhausted providers in the chain.
- **Refactor**: **Log Rotation Handling**
  - Updated the logging system to handle optional history logs during startup rotation.

## v0.3.6 (2026-01-23)
- **Feature**: **Environment Variable Expansion in Paths**
  - Added support for `%VAR%` (Windows) and `$VAR` (Unix) syntax in all configuration paths.
  - Applies to Database, Logs, Screenshots, and Terrain data paths.
- **Refactor**: **Robust Configuration Loading**
  - Refactored the configuration loader to ensure validation (e.g., locale format) always runs, even when loading an existing file.
- **Testing**: **Enhanced Coverage**
  - Implemented comprehensive table-driven tests for the configuration package.

## v0.3.5 (2026-01-22)
- **Fix**: **Screenshot Narration Regression**
  - Redesigned priority handling to use the `generationQueue` instead of cancelling active jobs.
  - Manual requests (screenshots, manual POIs) now wait for the current generation to finish naturally and start immediately after.
- **Fix**: **LLM Failover Reliability**
  - Made the failover provider's retry loop context-aware, preventing blocking waits during cancellation or shutdown.
  - Added strict handling for `context canceled` to avoid unintended retries on stopped requests.
- **Fix**: **Queue Worker Coordination**
  - Resolved an issue where `ProcessGenerationQueue` could skip jobs when the generator was busy.
  - Added self-perpetuating triggers to ensure the queue is always drained as soon as a job completes.
- **Refactor**: **Clean Narrator Pipeline**
  - Removed deprecated cancellation logic and `genCancelFunc` field.
  - Resolved multiple linting issues related to unused variables and imports.

## v0.3.4 (2026-01-22)
- **Feature**: **Binary Release Preparation**
  - Added `release-binary` target to Makefile for automated packaging.
  - Transparently handles SimConnect DLL and UI asset embedding.
- **Feature**: **Installation Helper Improvements**
  - `install.ps1` now offers to automatically create and open `.env.local` if missing.
- **Fix**: **LLM Logging**
  - Resolved an issue where JSON responses were logged as `<nil>` in `llm.log`.

## v0.3.3 (2026-01-22)
- **Fix**: **Beacon Immersion**
  - Added strict suppression of beacon spawning when the aircraft is on the ground (`sim.Telemetry.IsOnGround`).
- **Feature**: **Variety Scoring Configuration**
  - Made `NoveltyBoost` and `GroupPenalty` configurable in `phileas.yaml`.
  - Tuned default values to allow better variety in high-density areas like the Alps.

## v0.3.2 (2026-01-22)
- **Fix**: **Playback Queue Race Condition**
  - Resolved a bug where system-selected POIs could get stuck in the queue if generation finished after the narrator went idle.
  - Added explicit queue processor triggering in the pipelining flow.
- **Feature**: **TRACE Log Level**
  - Introduced a new `TRACE` level (controlled by `logging.EnableTrace`) to reduce log noise in `DEBUG` mode.
  - Moved high-frequency tile checks, hydrated POI tracking, and scoring instrumentation to the TRACE level.
- **Feature**: **Environment-Based Secret Management**
  - Migrated all API keys and service secrets to environment variables.
  - Added `.env.template` to the repository to streamline setup and protect user secrets.
- **Refactor**: **EdgeTTS Decoupling**
  - Removed hardcoded handshake "secrets" from the binary; these are now configurable via environment variables.
- **Documentation**: **Groq & Fallback Chains**
  - Added instructions for configuring LLM fallback chains and Groq setup.

## v0.3.1 (2026-01-22)
- **Fix**: **Gemini Stats Tracking**
  - Moved API success/failure tracking from the centralized failover provider to individual LLM providers.
  - Gemini client now directly tracks `gemini` stats, eliminating double-counting and clutter from a generic `llm` stat.
- **Fix**: **Groq Request Tracking**
  - Added `groq.com` domain normalization in the request client for accurate per-provider stats.
- **Narrator**: **Rescue Script Headroom**
  - Increased the word limit passed to the rescue script mechanism from 1.0x to **1.5x** of the original `MaxWords`.
  - Gives the rescue LLM more flexibility to preserve interesting content while still trimming contaminated output.
- **Fix**: **Dynamic Config QID Handling**
  - The LLM-suggested QIDs for new subclasses are no longer trusted. The system now triggers a lookup by name instead.
  - Prevents invalid or hallucinated QIDs from polluting the classification hierarchy.

## v0.3.0 (2026-01-22)
- **Feature**: **Multi-Provider LLM Support (Groq & OpenAI)**
  - Introducing a flexible, multi-provider LLM architecture.
  - **Groq Support**: Added native support for Groq's high-speed inference engine via `type: groq`.
  - **Generic OpenAI**: Added a reusable `type: openai` provider compatible with any standard Chat Completions API (Mistral, Ollama, standard proxies).
  - **Failover**: All providers are fully integrated into the failover chain with centralized logging and stats tracking.
  - **Config**: Updated `phileas.yaml` with examples for new providers.
- **Refactor**: **System Reliability**
  - Centralized JSON cleaning utilities to share robustness logic across all providers.
  - Removed internal complexity from Gemini client, making it a pure API wrapper.
- **Refactor**: **Sparse Profile Support & Dynamic Routing**
  - **Smart Failover**: The `Failover` provider now intelligently inspects provider capabilities (`HasProfile`) before routing requests.
  - **Sparse Profiles**: Allows specialized providers (e.g., Vision-only) to coexist with general-purpose providers without static chain configuration.
  - **Dynamic Chains**: Removed rigid chain building logic from `factory.go`; the system now auto-discovers the best available path for each request (Text, JSON, Image).
  - **Simplification**: `NewAIService` now accepts a single flat list of providers, reducing configuration complexity.

## v0.2.138 (2026-01-22)
- **Fix**: **LOS Log Refinements**
  - Resolved a confusing log message "All POIs blocked by LOS or Filter" when zero candidates were in range.
  - **Logic**: No POI candidates now correctly logs as a single `DEBUG` message.
  - **Log Levels**: Downgraded mountain-blocked POI visibility warnings from `WARN` to `INFO`.
  - **Refactor**: Split the complex `getVisibleCandidate` logic into smaller, maintainable helper functions to reduce cyclomatic complexity and satisfy linting rules.

## v0.2.137 (2026-01-21)
- **Narrator**: **Word Count & Profile Synchronization**
  - Synchronized `MaxWords` targets for Screenshots (~50 words) to ensure visual details are preserved.
  - Updated Essays to use topic-specific word counts from `essays.yaml` while respecting the user's length multiplier.
  - Fixed dynamic profile selection for different narrative types (POI, Screenshot, Essay, Debrief).
  - Improved test coverage for the narration pipeline to over 80%.
  - Added instrumentation to confirm multimodal image attachments in logs.

## v0.2.136 (2026-01-21)
- **Narrator**: **Script Rescue Metrics**
  - Updated the "Script Rescue successful" log message to include original and rescued word counts.
  - Provides better visibility into the performance and necessity of the rescue mechanism.

## v0.2.135 (2026-01-21)
- **Feature**: **Generic Narration Panels**
  - Added support for displaying "Flight Debrief" and "Regional Essay" information in the UI even when no POI is active.
  - Added `current_type` to narrator status API to allow UI to differentiate between narration categories.
  - Improved Dashboard auto-open logic to show the debrief panel when playing.
  - Updated Overlay POI panel to show descriptive categories for non-POI content.

## v0.2.134 (2026-01-21)
- **Refactor**: Improved clarity and naming of Narrator queue management.
  - Renamed `queue` to `playbackQueue` and `priorityGenQueue` to `generationQueue`.
  - Standardized method names using "Playback" and "Generation" terminology.
  - Updated all internal services, API handlers, and test mocks to reflect the new naming.

## v0.2.133 (2026-01-21)
- **Fix**: **Narrative Playback on Pause**
    - Prevented narrations from starting while the application is paused.
    - **Logic**: Narratives that finish preparation (LLM/TTS) while paused are now kept in the queue.
    - **Resume**: Added logic to automatically trigger the playback queue when the user unpauses, ensuring no waiting content is delayed further.

## v0.2.132 (2026-01-21)
- **Fix**: **Robust Essay Title Fallback**
    - Implemented a guaranteed fallback mechanism in `PlayNarrative`: if the title cannot be extracted from the script, the system now constructs a default title (e.g., "Essay: Topic Name") to ensure the UI Panel always appears.
    - **Logic**: Enhanced title extraction regex to handle Markdown bold/italic markers (`**TITLE:**`, `Title :`) and irregular spacing, resolving cases where the LLM's formatting prevented metadata parsing.

## v0.2.131 (2026-01-20)
- **Fix**: **Essay Title Extraction & Display**
    - Resolved a bug where Essay/Debrief titles were not being correctly extracted from the LLM script, causing the Overlay Info Panel to remain hidden.
    - **Logic**: The narrator now extracts the "TITLE:" line *before* TTS generation, preventing the "Title colon..." prefix from being read aloud.
    - **State**: Fixed `PlayNarrative` to correctly restore the essay title state during playback, ensuring the UI panel appears.

## v0.2.130 (2026-01-20)
- **Feature**: **Unified Essay/Debrief Info Panel**
    - Essays and Debriefs now appear in the Overlay POI Panel, displaying their title (e.g., "Topic: Aviation History" or "Debrief").
    - Updated `OverlayPOIPanel` to support items with only a Title (no POI/Image), ensuring consistent UI feedback for all narrative types.
    - Renamed "Flight De-brief" to "Debrief" for cleaner UI presentation.
- **Narrator**: **Word Count Multipliers**
    - Implemented consistent word count scaling for all narrative types (Essays, Debriefs, Screenshots) based on the user's "Text Length" setting (1-5).
    - **Essays/Debriefs**: Use "Long" target (base 200 words) * multiplier.
    - **Screenshots**: Use "Short" target (base 50 words) * multiplier.
- **Narrator**: **Relative Script Rescue**
    - Changed the script rescue threshold from a fixed +100 words buffer to a **+30%** relative buffer.
    - Allows longer narratives (e.g., multiplier x2.0) to have proportionally larger buffers before triggering a rescue rewrite.
- **UI**: **Friendly "Unknown" Cities**
    - The Overlay Telemetry Bar now displays "**Far from civilization**" instead of "near Unknown" when no city data is available.

## v0.2.129 (2026-01-20)
- **Fix**: **Fish Audio Reliability**
    - Added retry logic (3 attempts) and empty validation to prevent `wav: EOF` errors.
- **Refactor**: **Queue Playback Logging**
    - Added POI Title to queue error logs for better visibility and debugging.
- **Fix**: **River Prompts**
    - Strengthened instructions for river narrations to avoid source/mouth confusion.
- **Fix**: **Test Stability**
    - Fixed panic in `pkg/core` tests by initializing mocks correctly.

## v0.2.128 (2026-01-20)
- **Fix**: **Restored Essay Rules (Gap-Filling)**
    - Essays now correctly function as "gap-fillers": they only trigger if NO visible POIs are eligible (`hasEligiblePOI`).
    - Added comprehensive silence checks: Essays cannot trigger while audio is playing (paused, generating, or active).
    - Added **Takeoff Grace Period** for essays: Essays must wait at least `delay_before_essay` (default 4m) after takeoff before firing.
- **Fix**: **Duplicate Narration Prevention**
    - Introduced `IsPOIBusy(qid)` check in the narrator service.
    - Prevents `NarrationJob` from selecting a POI that is currently generating or sitting in the queue, eliminating "double narration" of the same POI.

## v0.2.127 (2026-01-20)
- **Fix**: **SPARQL Duplicate QID Elimination**
    - SPARQL queries can return multiple rows for the same entity when multi-valued properties (e.g., `area`) differ between rows.
    - Added QID deduplication at parse time in `parseBindings()` — first occurrence wins.
    - Added secondary deduplication in `filterExistingPOIs()` to avoid redundant `TrackPOI` calls.
    - Reduced log noise by only logging "Tracked POI (hydrated)" for genuinely new POIs.
- **Refactor**: **SPARQL Parsing Cleanup**
    - Extracted `parseInstances()` and `parseLocalTitles()` helpers to reduce cyclomatic complexity.

## v0.2.126 (2026-01-20)
- **Fix**: **Screenshot Narration Regression**
    - Restored multimodal image support lost in the unified pipeline refactor.
    - `GenerateNarrative` now calls `GenerateImageText()` when an `ImagePath` is provided, ensuring the actual screenshot is sent to Gemini.
    - Fixed template formatting bug in `screenshot.tmpl` where coordinates were being double-formatted (`%!f(string=...)` error)—values are now passed through directly since they're already formatted in Go.
    - **Impact**: Gemini now correctly analyzes the image content instead of hallucinating based solely on location context.

## v0.2.125 (2026-01-20)
- **Fix**: **Duplicate Thumbnail LLM Requests**
    - Implemented a **singleflight pattern** in `HandleThumbnail` to coalesce concurrent requests for the same POI.
    - When multiple frontend components (POIInfoPanel + OverlayPOIPanel) request a thumbnail simultaneously, the second request now waits for the first to complete and receives the same result.
    - Ensures both UIs display the same thumbnail from a single LLM call.
- **Fix**: **Debrief Queue Bypass**
    - Removed redundant `s.generating` check from `PlayDebrief` that caused debriefs to be skipped instead of queued.
    - Debriefs are now properly enqueued as high-priority items and will play when the audio is free.

## v0.2.124 (2026-01-20)
- **Fix**: **Scheduler Pause Button**
    - Resolved a bug where the Pause button in the browser app did not stop auto-narration.
    - The scheduler was checking `audio.IsPaused()` (transient playback state) instead of `audio.IsUserPaused()` (user-initiated pause).
    - Updated `AIService.IsPaused()` in `pkg/narrator/service_ai_state.go` to correctly check the user pause state.
- **Fix**: **Missing POI Name in Thumbnail Selector**
    - The thumbnail selection prompt was using `p.NameEn` which can be empty for POIs with only local names.
    - Changed to `p.DisplayName()` in `internal/api/pois.go` to ensure a name is always available (fallback: NameUser > NameEn > NameLocal > WikidataID).
- **UI**: **Simplified Narration Length Selector**
    - Removed the **M** (Medium) button from the POI Info Panel.
    - Renamed **S** to **SHORT** and **L** to **LONG** for clarity.
    - Default selection is now **SHORT** instead of Medium.

## v0.2.123 (2026-01-20)
- **Refactor**: **Unified Narration Generation Pipeline & Modularization**
    - Split the monolithic `service_ai.go` into specialized files: `service_ai_queue.go`, `service_ai_tts.go`, `service_ai_state.go`, `service_ai_stats.go`, and `service_ai_generation.go`.
    - Implemented a standardized `GenerateNarrative` pipeline with a unified `GenerationRequest` struct, simplifying the flow for POIs, Essays, Debriefs, and Screenshots.
    - Removed redundant generation methods (e.g., `GenerateScreenshotNarrative`) in favor of the unified pipeline.
- **Testing**: **Narrator Coverage & Stability**
    - Added tests for `handleTTSError` (fallback logic), `rescueScript` (LLM script cleanup), and comprehensive state getters.
    - Verified complex queue limits and priority boosting logic.
- **Fix**: **Project-wide Lint & Reliability**
    - Resolved `SA5011` potential nil pointer dereferences in `service_ai_image.go`.
    - Cleaned up unused fields and methods across the `narrator` package to satisfy static analysis.
    - Fixed race conditions and state assertion mismatches in narrator tests.

## v0.2.122 (2026-01-19)
- **Refactor**: **Unified Narration Queue**
    - Centralized queue management for all narrative types (POI, Debrief, Essay, Screenshot).
    - Enforced strict concurrency rules: only one manual/debrief/screenshot item in queue at a time.
    - Updated `PlayPOI`, `PlayDebrief`, `PlayImage`, and `PlayEssay` to use the unified queue.
- **Fix**: **Narrator State & Testing**
    - Resolved race conditions in playback monitoring and queue processing.
    - Added `service_ai_queue_test.go` to verify asynchronous queue behavior and priority logic.
    - Fixed lint errors and template loading issues in narrator tests.

## v0.2.121 (2026-01-19)
- **Fix**: **EdgeTTS Handshake Reliability**
    - Updated the EdgeTTS client headers (`User-Agent` and `Sec-MS-GEC-Version`) to align with Edge version 131.
    - Resolves transient `403 Forbidden` handshake failures caused by version mismatch with the upstream service.

## v0.2.120 (2026-01-19)
- **Feature**: **Screenshot Thumbnail Display**
    - Screenshots are now displayed as thumbnails in both the Main Info Panel and the Overlay (`/overlay`) during narration playback.
    - Added a new secure endpoint `/api/images/serve` to serve local screenshot files to the frontend.
- **Narrator**: **Context-Aware Visuals**
    - The Screenshot Prompt now receives the **Trip Summary** context, allowing Gemini to reference previous events ("As we saw earlier...").
    - Explicitly instructed Gemini to ignore **Hot Air Balloons** (multiplayer traffic) in screenshots.
- **Fix**: **Frontend Integration**
    - Updated `NarratorStatusResponse` API and frontend TypeScript interfaces to expose `current_image_path`.

## v0.2.119 (2026-01-19)
- **Fix**: **Classifier Cache Logic**
    - Resolved a bug where the `__IGNORED__` sentinel was treated as a valid category, causing ignored entities (e.g., administrative districts) to appear on the map.
    - Updated `classifier.go` to strictly respect the ignore signal in both initial search and hierarchy traversal.

## v0.2.118 (2026-01-19)
- **Refactor**: **Wikidata Pipeline Modernization**
    - Split the monolithic `pkg/wikidata/pipeline.go` into modular components: `pipeline_filter.go` (Filtering), `pipeline_enrich.go` (Enrichment), and `pipeline_hydration.go` (API Hydration), improving maintainability.
    - Removed legacy `Service` shims and lazy initialization logic.
    - Updated all tests to instantiate and test the `Pipeline` struct directly, ensuring stricter type safety and cleaner architecture.

## v0.2.117 (2026-01-18)
- **Feature**: **Screenshot Narration Priority**
    - Screenshots are now detected immediately at the start of the narration loop, ensuring they trigger even in remote areas (e.g. over the ocean) where no POIs are available.
    - Bypasses the standard "Frequency" checks for immediate feedback.
- **Narrator**: **Detailed Screenshot Descriptions**
    - Switched screenshot narration to use `long` form word targets (default ~200 words) instead of short, allowing for richer scene descriptions.
- **Fix**: **Narrator Crash & Logs**
    - Resolved a nil pointer dereference when the Beacon system attempted to read coordinates from a screenshot (which has no POI data).
    - Reduced log spam by downgrading "Screenshot described" messages to DEBUG level.

## v0.2.116 (2026-01-18)
- **Feature**: **Unified Narrator Pipeline**
    - Refactored all narrative types (POI, Screenshot, Essay, Debrief) to use the shared `PlayNarrative` method.
    - Added `Type` and `Title` fields to the `Narrative` struct to support non-POI narratives.
    - This unifies state management, playback monitoring, and queue handling across all narration types.
- **Fix**: **Screenshot TTS Issues**
    - Fixed screenshot narration that was not playing due to incorrect TTS call with wrong argument order.
    - Screenshots now properly queue behind current narrations instead of interrupting them.
    - Removed dummy POI usage from Debrief narration.

## v0.2.115 (2026-01-18)
- **Fix**: **Played POI Blue Color**
    - Fixed a bug where played MSFS POIs were not displayed in blue because the `isMSFS` check had higher priority than the `isPlayed` check in the marker color logic.
- **Fix**: **Overlay Layout Stability**
    - Fixed the log line width causing the stats boxes to resize. The log line now inherits width from the container instead of determining it.
    - Added `white-space: nowrap` to memory value cells to prevent "MB" from wrapping.

## v0.2.114 (2026-01-18)
- **Fix**: **"Next POI" Marker Restoration**
    - Restored the "Preparing" state visualization in `SmartMarkerLayer`.
    - The POI currently being generated/prepared by the Narrator is now highlighted (darker green, larger scale) on both the Main Map and Overlay Mini-Map.
    - Resolves a regression where the "Look Ahead" marker was missing from the UI.
- **Refactor**: **Frontend Filtering Cleanup**
    - Removed redundant client-side filtering (e.g., `minPoiScore` checks) from React components.
    - The Frontend now relies strictly on the Backend API as the single source of truth for visibility and filtering.
- **Fix**: **Adaptive Fallback Threshold**
    - Corrected the fallback score threshold in `POIManager`'s adaptive mode to allow negative-score POIs when necessary to meet the target count.
- **Fix**: **Overlay Layout Polish**
    - Fixed a layout regression in the Overlay Telemetry Bar where the "System Stats" block was incorrectly nested inside the API Status column.
    - The stats (Memory, Tracked POIs) now appear in their own dedicated column, matching the grid aesthetic.

## v0.2.113 (2026-01-18)
- **Fix**: **Classifier Cache Poisoning**
    - Resolved a bug where ignored categories were saved as "Unknown" (`""`) in the database, causing them to be incorrectly rescued in subsequent runs.
    - The system now explicitly checks ignore rules before saving intermediate hierarchy nodes, preventing cache corruption.

## v0.2.112 (2026-01-18)
- **Feature**: **Configurable Screenshot Model**
    - The Gemini model used for screenshot analysis is now independently configurable via `phileas.yaml` (`llm.profiles.screenshot`).
    - Defaults to `gemini-2.5-flash-lite` for efficient vision analysis.
    - **Fix**: Resolved implicit dependency on deprecated `gemini-2.0-flash` model.

## v0.2.111 (2026-01-18)
- **Feature**: **Screenshot Narration**
    - Implemented a watcher service to detect and narrate screenshots taken during flights.
    - Integrated Gemini 1.5 Pro/Flash Vision capabilities for multimodal analysis (scenery, landmarks) with flight context.
    - Added configurable prompt with dynamic word count limits via `narrator/screenshot.tmpl`.
    - **Fix**: Screenshot narration properly interrupts/blocks lower-priority POI narration.

## v0.2.110 (2026-01-18)
- **Fix**: **Improved "Lone Wolf" Logic**
    - The `CountScoredAbove` competition metric now correctly ignores POIs that are on cooldown (recently narrated).
    - This ensures that a "Silent Giant" (high-score but played POI) does not artificially force its neighbors into "High Competition" mode, allowing valid "Lone Wolf" narration for active candidates.

## v0.2.109 (2026-01-18)
- **Fix**: **Expired POI Filtering**
    - The backend now strictly filters out "Played" POIs that have exceeded their `RepeatTTL` (cooldown) from the UI response.
    - Resolves an issue where old/expired POIs remained permanently colored blue on the map, even after their cooldown had elapsed.
    - **Impact**: Map markers now accurately reflect the "Playable" state of landmarks.

## v0.2.108 (2026-01-18)
- **Feature**: **System Stats in Overlay**
    - Added a **System Stats** section to the Overlay Telemetry Bar, displaying Memory Usage (RSS/Max) and Tracked POI count.
    - Provides real-time visibility into application performance and resource consumption.
- **Narrator**: **Persona & Prompt Refinement**
    - **Persona Macro**: Added documentation for `common/macros.tmpl`.
    - **Rescue Script**: Updated `rescue_script.tmpl` to strip stage directions (e.g., `[pauses]`) and emotional cues from the final output.
    - **TTS**: Explicitly forbade stage directions and sound effects in the `edge-tts` prompt to prevent them from being read aloud.
- **Fix**: **Debrief Audio Synthesis**
    - The "Landing Debrief" now correctly performs an Audio Synthesis step before attempting playback, ensuring the summary is actually voiced.
- **Fix**: **Cache Layer Visibility**
    - The debug Cache Layer (circles) now only renders when the client is actively connected to the simulator, preventing visual clutter in disconnected states.

## v0.2.107 (2026-01-18)
- **Feature**: **Debrief After Landing**
    - Automatically generates a "Trip Summary Debrief" when the aircraft lands (< 15kts).
    - Summarizes the flight's highlighted POIs using the rolling trip summary memory.
    - **Config**: Added `narrator.debrief` section to `phileas.yaml` (enabled by default).
    - **Logic**: Triggered by `LandingJob` detecting airborne-to-ground transition. Includes a 5-minute cooldown.
- **Narrator**: **Dynamic Length Control**
    - The Debrief narration now respects the `narrator.narration_length_long_words` setting (default 200 words).
    - Updated `debrief.tmpl` to use dynamic `MaxWords` instead of hardcoded limits.

## v0.2.106 (2026-01-18)
- **Feature**: **Configurable Overlay Log Line**
    - The server log display in the overlay is now optional and configurable.
    - **Config**: Added `overlay.log_line` (default: `true`) to `phileas.yaml`.
    - **API**: Exposed the setting via `/api/config` for frontend conditional rendering.

## v0.2.105 (2026-01-18)
- **Feature**: **Overlay Polish & Log Display**
    - Refined the Overlay Telemetry Bar layout with high-contrast stats, vertical API status block (Grid Layout).
    - Added a **Server Log Line** display to the overlay, showing the latest filtered log message for debugging.
    - **Visuals**: Switched to "Inter" font, darkened backgrounds, and improved typography for readability.
- **Fix**: **Log Readability & Context**
    - **Empty POI Name**: Thumbnail selection logs now correctly identify the POI using fallback names (Local or QID) if the English name is missing.
    - **Formatting**: Trimmed trailing spaces from log parameters and removed spurious warning logs about unused Gemini tools.
    - **Best Name**: Updated API logging to use standard `DisplayName()` logic.

## v0.2.104 (2026-01-18)
- **Feature**: **Viewport-Based Cache API**
    - Refactored the Frontend and API to query cached tiles using the visible map bounding box (`min_lat`, `max_lat`, `min_lon`, `max_lon`) instead of a fixed center-point radius.
    - **Optimization**: Significant reduction in over-fetching by ensuring the cache layer requests exactly what is visible on screen.
- **Backend**: **Spatial Cache Metadata**
    - Updated `pkg/store` and `pkg/wikidata` to store and retrieve cache tiles with precise spatial metadata (`lat`, `lon`, `radius`), enabling accurate reconstruction of the grid without full re-computation.
    - Added `GetGeodataInBounds` to the store interface for efficient spatial range queries.

## v0.2.103 (2026-01-17)
- **Feature**: **Overlay Configuration**
    - The three overlay components (Map Box, POI Info, Info Bar) are now individually configurable via `phileas.yaml`.
    - **Config**: Added `overlay.map_box`, `overlay.poi_info`, and `overlay.info_bar` boolean toggles.
    - **Impact**: Provides flexibility for users who want a cleaner or customized overlay layout (e.g. for OBS/Streaming).

## v0.2.102 (2026-01-17)
- **Fix**: **Rescue Script Word Limit**
    - The "Rescue Script" mechanism (which cleans up LLM output) now respects the `MaxWords` constraint.
    - Updated `rescue_script.tmpl` to explicitly instruct the LLM to limit its output, preventing rescued scripts from becoming overly verbose.
- **Fix**: **Thumbnail Selection**
    - Strengthened instructions in `thumbnail_selector.tmpl` to strictly reject panoramic images (wide aspect ratios), which display poorly in square UI elements.
- **Fix**: **Test Suite Stability**
    - Synced frontend and backend version numbers to pass `TestVersionSync`.
    - Fixed linting errors in experiment scripts and `pkg/geo`.
- **Documentation**: **Classification & Rescue**
    - Added `docs/CLASSIFICATION.md` detailing the classification flow and rescue logic.
    - Resolved a persistent "Unknown Category" bug by purging corrupted cache entries (Operational Fix).

## v0.2.101 (2026-01-17)
- **Feature**: **Topics to Avoid**
    - The Narrator now respects the `avoid` list in `configs/interests.yaml`.
    - **Logic**: Explicitly instructs the LLM to avoid mentioning or discussing specific user-defined topics (e.g., "Demographics", "Administrative units").
    - **Impact**: Provides a more tailored and enjoyable narration experience by filtering out unwanted subject matter.
- **Fix**: **World Map Cleanup**
    - **Smart Markers**: POI markers are now hidden when the application is disconnected from the simulator (World Map mode).
    - **Impact**: Ensures a clean view of the global coverage map without stale or distracting POI markers overlaying the terrain.

## v0.2.100 (2026-01-17)
- **Fix**: **Region Name Resolution**
    - Resolved issue where non-US regions appeared as numeric codes (e.g., "11") in the overlay.
    - **Installation**: Now automatically downloads `admin1CodesASCII.txt` from GeoNames to map numeric admin codes to human-readable names (e.g., "Île-de-France").
- **Fix**: **Overlay POI Image Cutoff**
    - Relaxed the vertical size constraint on POI thumbnails in the overlay.
    - Images can now span up to 50% of the screen height, preventing cropping of vertical or square images.
- **Narrator**: **Transition Logic Refinement**
    - Updated `script.tmpl` to replace strict "No Forced Transitions" rules with positive encouragement for thematic continuity.
    - **Logic**: Explicitly instructs the LLM to actively look for thematic links (e.g. "Speaking of history...") while avoiding verbatim repetition of facts.

## v0.2.99 (2026-01-17)
- **Fix**: **Narrator Prompt Repetition**
    - Updated `script.tmpl` to explicitly forbid the LLM from repeating the last sentence of the previous narration.
    - Added specific instruction: "Do not repeat the last sentence of the previous POI." to the prompt context.
    - **Impact**: Reduces repetitive phrasing and improved narrative flow between consecutive POIs.

## v0.2.98 (2026-01-16)
- **Feature**: **OBS Overlay (Streamer Mode)**
    - Added a dedicated, transparent overlay view (`/overlay`) designed for OBS/Streamlabs integration.
    - **Components**: Includes a transparent "Look-Ahead" Mini-Map, compact Telemetry Bar (Alt/HDG/Speed/Location), and an animated POI Info Card.
    - **Optimization**: Optimized for compositing with a fully transparent background and high-contrast, readable typography.
- **Fix**: **Map Visibility & Overlay Polish**
    - **Map Tiles**: Restored visibility of mini-map tiles in the OBS overlay by removing aggressive CSS opacity masks and switching to `dark_all` provider.
    - **Sync**: Synchronized POI filtering (score thresholds) between Overlay and Main App to ensure consistent content.
    - **Look-Ahead**: Implemented "Look-Ahead" camera offset for the overlay map, matching the main application's perspective.
    - **Telemetry**: Refined telemetry bar typography, merged location data, and added "Next Town/Country" readout.
- **Fix**: **Classifier Cache Poisoning**
    - Resolved a critical bug where "ignored" categories were saved as empty strings (`""`) in the database, causing them to be treated as "Unknown" on subsequent runs.
    - **Logic**: The system now explicitly persists an `__IGNORED__` sentinel value.
    - **Impact**: Permanently suppresses unwanted category branches (e.g., "Census-designated places") that were previously slipping through as "Rescued" landmarks after cache eviction/reload.

## v0.2.97 (2026-01-16)
- **Feature**: **Streaming Mode Toggle**
    - New checkbox in Configuration panel: "Keep updating in background".
    - When enabled, the UI continues polling the backend even when the browser tab is backgrounded.
    - Ideal for OBS capture and streaming setups.
- **Fix**: **Smart Markers Stabilization**
    - Replaced animated D3 force simulation with a synchronous one-shot layout.
    - Markers no longer wiggle endlessly; they compute their collision-free positions once and stay put.
- **Fix**: **Clickable Smart Markers**
    - Added `pointer-events: auto` to marker elements so they respond to clicks.
- **Fix**: **World Map Disconnected Mode**
    - Aircraft icon, range rings, and auto-recentering now only appear when `SimState === 'active'`.
    - Prevents spurious (0,0) map resets and phantom aircraft in disconnected/world map mode.
    - `CoverageLayer` (consolidated hexagons) now correctly displays when disconnected.

## v0.2.96 (2026-01-16)
- **Feature**: **Configurable Wikidata Fetch Interval**
    - The rate limit for Wikidata tile fetching is now configurable via `phileas.yaml` (`wikidata.fetch_interval`).
    - Defaults to `5s` (same as before).
    - Allows fine-tuning the balance between data freshness and API load/rate limits.

## v0.2.95 (2026-01-16)
- **Feature**: **Smart Markers (Physics-Based Layout)**
    - Replaced the stacking Z-Index system with a **Force-Directed Physics Simulation** (D3).
    - **Logic**: Markers now naturally repel each other to avoid overlap while staying tethered to their geographic location via a leader line.
    - **Impact**: Creates a clean, "ATC-style" display where even dense clusters of 50+ POIs are individually readable and clickable.
    - **Priority**: The currently active POI pushes harder to stay visible, while markers settle elegantly around it.
- **Tweak**: **360° Blind Spot**
    - Expanded the "Cockpit Blind Spot" logic to cover a full 360-degree radius beneath the aircraft (previously only forward +/- 90°).
    - Ensures that POIs directly below the plane are consistently hidden regardless of heading, mimicking the inability to see straight down through the floor.
- **Optimization**: **Map Startup Performance**
    - Fixed a race condition where the heavy "Global Coverage" calculation (World Map mode) would trigger unnecessarily during the split-second startup phase.
    - The Map now waits for a definitive connection status from the Simulator before deciding which mode to render, saving CPU cycles on boot.

## v0.2.94 (2026-01-16)
- **Feature**: **Dynamic "Corridor" Tile Scheduling**
    - The tile scheduler now dynamically adjusts the "heading penalty" based on the aircraft's ground speed to prioritize cells directly in front of the flight path.
    - **Logic**: Above 100kts, the penalty weight increases linearly (e.g., 0.9 at 300kts).
    - **Impact**: At high speeds, the scheduler ignores tiles to the side, focusing strictly on filling the map "dead ahead" to keep up with the aircraft.
- **UI**: **MSFS POI Star Icon**
    - Points of Interest sourced from MSFS (SimObjects) now display a gold star (★) icon next to their name in the Playback Controls.
    - Matches the visual style of the map markers, making it easier to identify high-quality scenery during flight.
- **Tweak**: **Thumbnail Prompt Logic**
    - Updated `thumbnail_selector.tmpl` to explicitly avoid "Overly panoramic" images which become illegible at small thumbnail sizes.
- **Doc**: **Configuration Updates**
    - Updated `README.md` with details on `categories.yaml`, `visibility.yaml`, and prompt templates.
    - Clarified usage of `fish-audio` (Trump voice) and standard category prompts.
- **Feature**: **Startup World Map (Cached Coverage)**
    - When the simulator is disconnected or inactive, the map now displays a visualization of all historically flown areas (cached geodata).
    - **Performance**: Tiles are aggregated into larger hexagonal regions (H3 Resolution 4) to ensure smooth rendering of global history.
    - **UX**: The map is fully unlocked (drag, zoom, scroll) in this mode, allowing users to explore their flight history before starting a flight. Locks automatically when the Sim connects.
- **Tweak**: **Essay Titles**
    - Updated `essay.tmpl` to force descriptive, two-part titles (e.g. "Topic: Specific Subject") instead of abstract or clickbaity headlines.
- **Tweak**: **Realistic Blind Spot**
    - Updated visibility logic (`calculator.go`) to model a realistic cockpit blind spot that scales partially with altitude.
    - **Logic**: No blind spot below 500ft. Scales linearly to **5.0 NM** at 35,000ft. Capped at 5nm above FL350.
- **UI**: **Map Marker Visibility**
    - Implemented a Z-Index priority system for map markers to resolve overlapping issues.
    - **Priority**: Active POI (Top) > MSFS POIs (Middle) > Standard POIs (Bottom).
    - Ensures high-value markers are always clickable and visible even in dense clusters.

## v0.2.93 (2026-01-16)
- **Feature**: **Smart Tile Prioritization (Heading Bias)**
    - The Wikidata Scheduler now prioritizes tiles directly in front of the aircraft (`AngleDiff < 60°`).
    - **Logic**: Adds a penalty to tiles at the edge of the scan cone, allowing the system to fetch "dead ahead" tiles 4-5km further away than peripheral tiles.
    - **Impact**: Ensures the "Gap" in front of the aircraft is filled first during high-speed flight.
- **Optimization**: **Geodata Cache Fast-Forward**
    - Resolved a bottleneck where the scheduler was rate-limiting **Cache Hits** to the same speed as Network Requests (1 per 5s).
    - **Logic**: The system now iterates through up to 20 cached tiles *per tick* (instant verification), drastically reducing the time required to "burn through" known areas and reach new data boundaries.

## v0.2.92 (2026-01-16)
- **Feature**: **Intelligent Thumbnail Selection**
    - Implemented LLM-based image selection for POI Thumbnails.
    - Instead of randomly picking the first image, the system now asks Gemini to select the best visual representation of the POI from the available Wikipedia images.
    - Prioritizes wide-angle shots and clear photography over maps, diagrams, or portraits.
    - **Config**: Externalized the selection prompt to `configs/prompts/context/thumbnail_selector.tmpl` for easy tuning.
    - **Config**: Added `llm.profiles.thumbnails` (default `gemini-2.5-flash-lite`) to allow using a cheaper model for image selection.
    - **Fix**: Switched thumbnail selection to use **Image URLs** instead of filenames to drastically improve LLM accuracy, and applied a filter to remove unwanted SVGs/Icons before the prompt.
- **Refactor**: **Clean API Logic**
    - Reduced cyclomatic complexity in `internal/api/pois.go` by extracting helper functions for filename matching.


## v0.2.91 (2026-01-15)
- **Feature**: **Manual Text Length Controls**
    - Implemented a segmented length selector (1-5) in the Config Panel to scale narration length.
    - **Logic**: Applies a multiplier (1.0x to 2.0x) to the base target words (Short: 50, Long: 200).
    - **Refactor**: Replaced legacy "Skew Strategy" sampling with a deterministic multiplier system.
    - **Cleanup**: Removed dead code (`SampleSkewedValue`) and associated legacy tests.
- **Feature**: **Improved Takeoff Behavior**:
    - **Grace Period**: Narrations for non-airport POIs are now suppressed immediately after takeoff until the aircraft reaches 500ft AGL.
    - **Visibility Boost Threshold**: The visibility boost mechanism is now disabled below 500ft AGL.
    - **Ground Narration**: Fixed logic to ensure airport narrations triggered while on the ground always use the "max_skew" strategy.
- **Tools**: **Log Analysis Improvement**:
    - Refactored `cmd/experiments/log_analysis` script to fix linting errors and improve maintainability.
    - Updated latency tracking logic in analysis tools.
- **Tests**: **Coverage**
    - Added comprehensive table-driven tests for the new Multiplier logic in `pkg/narrator/multiplier_test.go`.
    - Verified proper state persistence for text length settings.
- **Narrator**: **Manual Override Logic**:
    - **Fix**: Clicking a POI manually now immediately cancels any pending or generating auto-narration to prioritize the user's request.
    - Resolves the "Narrator already generating" error when intervening during background pipeline work.
- **Narrator**: **Essay Logic Refinement**:
    - **Frequency**: Essays are now completely disabled when Narrator Frequency is set to "Rarely".
    - **Silence Rule**: Added `delay_before_essay` (default: 2m) to prevent essays from triggering too soon after a POI narration.
    - **Config**: Renamed `cooldown` to `delay_between_essays` for clarity.
- **Fix**: **Takeoff Delay**:
    - The 1-minute takeoff suppression grace period is now bypassed if the server is started while the aircraft is already airborne (mid-flight start).
- **Fix**: **SimConnect State**:
    - Correctly transitions to `Inactive` state instead of `Disconnected` when SimConnect is connected but data is paused/invalid.
- **Fix**: **Frontend Focus Stealing**:
    - Resolved a regression where the Info Panel would automatically switch focus to the narrator's target even if the user had manually selected a different POI.
    - The UI now respects manual selection state (`autoOpened` flag) to prevent interrupting user interaction.

## v0.2.90 (2026-01-14)
- **Feature**: **Sparse Tile Retrieval (Continuous Adaptive Density)**:
    - Implemented a proximity-based redundancy check in the Wikidata Scheduler.
    - Tuning: Fetches are now prioritized to sparsely fill ~2-3 rings (approx. 17km coverage) before backfilling gaps, reducing initial load and improving broad coverage.
    - Updated `pkg/wikidata/service.go` to strictly use predicted coordinates for tile candidates.
- **Fix**: **Narrator Regression**:
    - Resolved an issue where essays triggered immediately on startup or teleport by strictly enforcing eligibility checks in the `NarrationJob` flow.
- **Refactor**: **Test Performance**:
    - Optimized `pkg/wikidata` request-heavy tests with aggressive backoff settings, drastically reducing test execution time.

## v0.2.89 (2026-01-14)
- **Refactor**: **Linting & Complexity**:
    - Reduced cyclomatic complexity in `pkg/sim/simconnect/client.go` (`formatAPStatus`) and `pkg/poi/manager.go` (`StartScoring`).
    - Resolved `gocyclo` linting errors to maintain code quality standards.
- **Fix**: **Thumbnail Quality Filter**:
    - Enhanced filtering in `pkg/wikipedia/client.go` to reject unsuitable images (SVGs, maps, logos, flags, diagrams).
    - Added aspect ratio check to avoid vertical portrait images (Height > 1.3x Width), prioritizing landscape photos for the UI.

## v0.2.88 (2026-01-14)
- **Tuning**: **Restricted Visibility Boost**
    - The dynamic visibility boost (expanding search radius in empty areas) is now strictly limited to **XL** POIs (e.g., Mountains, Large Cities).
    - Prevents small local POIs from being detected at unrealistic distances.
- **Documentation**: **Installation Guide**
    - Updated `README.md` to accurately reflect the actions of `install.ps1`, including ETOPO1 and GeoNames download steps.
- **Config**: **Updates**
    - Added "Aviation" to default essay topics.
    - Extended category mapping for `administrative territorial entity`.

## v0.2.87 (2026-01-14)
- **Feature**: **Autopilot Status Display**
    - Integrated Autopilot telemetry (Master, FD, YD, Lateral/Vertical Modes) from SimConnect.
    - Updated the Frontend Info Panel to display AP status (e.g., "HDG 270 AP ALT 5000ft") below GPS coordinates.
    - Fixed struct alignment issues in `TelemetryData` to ensure accurate data readings from SimConnect.
- **Feature**: **Valley Altitude (Effective AGL)**
    - Exposed the calculated "Valley Floor" elevation (Lowest Elevation in visibility radius) to the frontend API.
    - Added "VAL" (Valley AGL) readout to the Altitude card in the Info Panel, showing the aircraft's height above the valley floor.
- **Tweak**: **Narrator Rescue Logic**
    - Increased the script length tolerance buffer from 50 to 100 words. This allows narratives to slightly exceed the target word count without triggering an expensive LLM rescue/rewrite pass.

## v0.2.86 (2026-01-14)
- **Fix**: **Beacon Lag**: Beacons now move to the *next* POI immediately after playback ends, even if the next narrative is still generating. Previously, the beacon would stay on the last POI until the new one was fully synthesized.
- **Fix**: **Beacon Flicker**: `SetTarget` now ignores redundant calls for the same location (~11m threshold), preventing the despawn/respawn blink caused by consecutive calls from the pipeline and `PlayPOI`.
- **Fix**: **Manual Override**: Clicking a POI manually now correctly discards any staged (pipelined) narrative, ensuring the requested POI is always played instead of the pre-prepared one.

## v0.2.85 (2026-01-13)
- **Fix**: **Jumping Beacons**: Exposed `GetPreparedPOI` to the scheduler to allow it to respect pre-calculated/staged narratives.
    - Updated `NarratorJob` to check for staged content before selecting new candidates.
    - Resolves the visual glitch where the beacon would jump to a new target just before the audio for the *previous* target started playing.

## v0.2.84 (2026-01-13)
- **Cleanup**: **SimConnect Unification**: Unified SimConnect disconnect logic to be idempotent and thread-safe.
    - Updated `disconnect` to return early if already disconnected, eliminating redundant logs.
    - Updated `dispatchLoop` to terminate immediately on handle loss, preventing race conditions that triggered spurious watchdog timeouts.

## v0.2.83 (2026-01-13)
- **Fix**: **Beacon Cleanup**: `ResetSession` (triggered by teleport or new flight) now explicitly clears any active beacons (balloons) from the simulator, preventing visual clutter.

## v0.2.82 (2026-01-13)
- **Fix**: Resolved a regression where manual play requests repeated endlessly due to client-side timeouts. The API handler now processes requests asynchronously (`HandlePlay` returns immediately).N
- **Cleanup**: Removed spammy "NarrationJob: Pipeline trigger" log from `pkg/core/narration_job.go`.

## v0.2.81 (2026-01-13)
- **Dynamic Visibility Boost**: implemented a mechanism to dynamically increase visibility range (up to 1.5x) when POIs are scarce, improving content discovery in sparse areas.
- **Frontend Sync**: updated the map visibility API to accurately reflect the boosted visibility radius in the UI.
- **Narrator**: added logging for current visibility boost factor during narration triggers.

## v0.2.80 (2026-01-13)
### Bug Fixes
- **Telemetry Loop Stall**: Fixed a regression where the main telemetry loop would stall after a SimConnect disconnection. Added a **Watchdog Timer** to the client that forces a reconnection if no data is received for 5 seconds.
- **Narrator Continuity**: Updated script template to explicitly forbid the LLM from repeating the previous sentence's ending, reducing redundant narration.
- **POI Playback**: Fixed a race condition where prepared narrations were discarded if the scheduler updated the target POI mid-generation. The narrator now prioritizes the prepared script.
### Improvements
- **Dynamic Baseline**: Refined the "Dimension Rescue" logic to use the full global median (100%) as the baseline for filtering small POIs, reducing noise in sparse areas.
- **Content Expansion**: Added "myths" and "mysteries" to interest categories and essay topics.
- **Flight Status**: Refined the flight status description prompt logic for better natural language generation.

## v0.2.79 (2026-01-13)
*   **Fix: Filter Markdown Artifacts from TTS**: Asterisks (`*`) are now stripped from LLM-generated scripts before TTS synthesis. This prevents markdown formatting (like `**bold**`) from being read aloud.
*   **UI: Reduced Cache Layer Opacity**: The map's cache layer circles are now 50% more transparent for better visibility of underlying terrain.

## v0.2.78 (2026-01-13)
*   **Maintenance**: General stabilization and verification of the Headset Audio Effect and Pipelined Narration logic.
*   **Testing**: Verified all audio and core tests pass in a clean state.

## v0.2.77 (2026-01-13)
*   **Feature: Optional Headset Audio Effect**: Added a configurable digital bandpass filter to simulate the sound of an aviation headset or radio.
    *   Configurable frequency range (defaults to 400Hz - 3500Hz for speech intelligibility).
    *   Toggleable in `phileas.yaml`.
*   **Refactor: Audio Pipeline**: Refactored `audio.Manager` to support real-time audio effects using a custom Biquad filter implementation.
*   **Fix: Pipelined Narration**: Reduced cyclomatic complexity in audio playback logic to improve maintainability.

## v0.2.76 (2026-01-13)
*   **Refactor: Enable Pipelined Narration Fix**: Previously, the cooldown logic doubled the wait time in some cases (Waiting Cooldown + Then Generating). The logic has been adjusted to subtract `AverageLatency` from the `Cooldown` wait time. This ensures playback initiates closer to the target cadence.
*   **Feature: Narrator Plausibility Check**: To prevent infinite stalls caused by "reasoning leaks" (where the LLM generates thousands of words instead of the requested amount), the Narrator now validates the script length. Scripts exceeding the limit (Requested + 200 words) are rejected with a warning, unblocking the system immediately.
*   **Fix: Pipelined Narration Tests**: Added specific test cases to verify script length validation and pipeline logic stability.

## v0.2.75 (2026-01-12)
- **Refactor**: **Pipelined Narration Tests**
    - Refactored `pkg/core/narration_job_test.go` and `pkg/narrator/service_ai_test.go` into comprehensive table-driven tests.
    - Verified pipeline logic: standard triggers, just-in-time preparation, and high-latency compensation.
    - Verified staging flow: consuming staged narratives, handling mismatches, and empty staging.
- **Refactor**: **Audio Manager Tests**
    - Consolidated individual state accessor tests in `pkg/audio/manager_test.go` into a unified table-driven suite (`TestManager_StateAccessors`) for better maintainability.
- **Feature**: **Random Mock Simulator Heading**
    - The Mock Simulator now automatically picks a random starting heading (0-360°) if `sim.mock.start_heading` is not configured (set to `null` or omitted).
    - Prevents repetitive testing scenarios always starting in the exact same direction.
    - Updated `pkg/config` and `pkg/sim/mocksim` to handle optional heading configuration.
- **Fix**: **Test Stability**
    - Resolved `make test` failures caused by `SA9003` lint errors (empty branch) in `pkg/core/narration_job.go`.
    - Removed duplicate code blocks in test files that were causing syntax errors.
- **Log**: **Cooldown Visibility**
    - The "Triggering narration" log message now explicitly includes `cooldown_after` duration, making it easier to verify pipeline timing logic.

## v0.2.74 (2026-01-12)
- **Feature**: **Encapsulated Audio Shutdown**
    - Implemented `Shutdown()` in `Audio Manager` to delete the final residual audio file when the application closes.
    - Encapsulated this logic within `AIService.Stop()`, adhering to clean separation of concerns.
    - Ensures zero audio artifacts remain in `%TEMP%` after a graceful exit.
    - **Fix**: Resolved build issues by updating test mocks in `pkg/narrator`.

## v0.2.73 (2026-01-12)
- **Feature**: **Strict Audio File Lifecycle (Rotation)**
    - Implemented a "One In, One Out" rotation strategy in the Audio Manager (`pkg/audio/manager.go`).
    - **Logic**: When a new audio file is loaded for playback, the *previously* played file is immediately deleted from disk.
    - **Impact**: Ensures disk usage is minimal (typically 1 active audio file in `%TEMP%` at any time) while strictly preserving the ability to **Replay** the current narration.
    - **Fix**: Prevents indefinite accumulation of `.mp3` artifacts in the system temporary directory.
    - **Fix**: Added explicit file handle closing in `Stop` and `Play` to prevent file lock leaks on Windows.

## v0.2.72 (2026-01-12)
- **Refactor**: **Decoupled Narration Generation**
    - Split the monolithic `PlayPOI` workflow into granular `GenerateNarrative` and `PlayNarrative` methods in `AIService`.
    - Allows the system to prepare scripts and synthesis in the background (potentially for a Playlist/Queue system) without blocking or requiring immediate playback.
    - Updated `Service` interface to expose these primitives.
- **Tuning**: **Visibility Distance Calibration**
    - Adjusted `visibility.yaml` lookup table to increase visibility ranges for Small (S) and Medium (M) POIs at lower altitudes (0-2000ft).
    - Ensures better detection of local landmarks when flying low in valleys, while maintaining performance at high altitudes.

## v0.2.71 (2026-01-12)
- **Fix**: **Hierarchy Caching Infinite Refetches**
    - Resolved a critical bug where empty cache entries for unclassified nodes caused an infinite refetches in the hierarchy classifier.
    - Optimized `classifyHierarchyNode` to strictly treat empty cache entries as "Dead Ends" and return immediately, preventing unnecessary re-traversal.
- **Refactor**: **Simplified Dynamic Config**
    - Removed the `ReprocessNearTiles` feature and associated `ForceRefresh` logic from `service.go` and `dynamic_config_job.go`.
    - This simplification prevents cache-busting behaviors that contributed to instability and high query counts.
- **Maintenance**: **Code Hygiene**
    - Downgraded `sqlite.go` hierarchy save logs from INFO to DEBUG to reduce console noise.
    - Resolved `gocritic` lint errors in `pkg/wikidata/merger.go` by adding named return parameters.
    - Verified fix with `make test` passing cleanly.

## v0.2.70 (2026-01-11)
- **Refactor**: **Unified Narration Selection**
    - Consolidated all POI selection logic into `GetNarrationCandidates`, removing deprecated methods and redundant loops.
    - `NarrationJob` now relies entirely on the `POIManager` to provide pre-filtered candidates, reducing complexity and potential for logic drift.
- **Feature**: **Dynamic Adaptive Filtering**
    - `NarrationJob` now dynamically respects the `filter_mode` (fixed/adaptive) and `min_poi_score` settings from the store.
    - **Adaptive Mode**: When enabled, the job requests candidates without a score threshold, allowing the Manager to return the best available items to meet the target count.
    - **Responsiveness**: Changes to settings in the UI are applied immediately to the next narration cycle without restart.
- **Maintenance**: **Test Coverage**
    - Refactored `pkg/poi` and `pkg/narrator` tests to align with the new interface.
    - Verified full compliance of mocks (`MockPOIProvider`, `TestPOIProvider`) with the updated `POIProvider` interface.

## v0.2.69 (2026-01-11)
- **Refactor**: **Strict Narration Filtering (Split Pipeline)**
    - Split `GetFilteredCandidates` into `GetPOIsForUI` (permissive, for map) and `GetNarrationCandidates` (strict, for narrator).
    - **Fix**: Resolved narration loop stalls where "Played" items were being reconsidered as candidates.
    - **Fix**: `GetBestCandidate` now strictly enforces `isPlayable`, `IsVisible`, and Score thresholds.

## v0.2.68 (2026-01-11)
- **Fix**: **Duplicate TTS Generation Eliminated**
    - Refactored narrator prompt templates (`script.tmpl`, `edge-tts.tmpl`, `azure.tmpl`) to remove redundant language instructions at the end of templates.
    - Consolidated formatting rules and language requirements to prevent the LLM from "echoing" or repeating the curation script.
- **Feature**: **In-Memory Skew Strategy Exposure**
    - Added a `NarrationStrategy` field to the `model.POI` struct to track the length strategy used (Short, Medium, Long).
    - Updated the frontend UI to synchronize with this strategy, automatically highlighting the corresponding S|M|L buttons in the POI Info Panel.
    - Captures "Competition-based Skew" dynamically during the narration cycle for better user visibility into system decisions.
- **Maintenance**: **Manual Rollback of DB Persistence**
    - Ensured skew strategy remains a transient in-memory attribute; rolled back initial database schema/migration changes to avoid polluting persistent storage with ephemeral session state.


## v0.2.67 (2026-01-11)
- **Fix**: **Metadata Pipeline Optimization & Stability**
    - Eliminated redundant secondary metadata fetches (API calls) in the Wikidata pipeline. Classification now relies strictly on high-performance SPARQL tile data.
    - Improved stability by dropping items without valid category data (`P31`) early, unless they are rescued by notable physical dimensions (e.g., Height, Length).
- **Feature**: **Generalized Group Isolation (POI Merging)**
    - Replaced hardcoded "Island Group" logic with a generalized rule: **POI merging is now strictly forbidden across different `category_groups`**.
    - This ensures that distinct features like a **City** (Settlement) and its **Airport** (Aerodrome) will always coexist on the map, even if they are physically overlapping.
    - Removed legacy `isIslandGroup` helper function.
- **Maintenance**: **Expanded Test Coverage**
    - Implemented comprehensive table-driven unit tests in `merger_test.go` and `service_test.go`.
    - Verified new isolation logic and Phase 1 dropout/rescue scenarios across 25+ files.

## v0.2.66 (2026-01-11)
- **Fix**: **Generic "Camera" Icons (Healing on Load)**
    - Resolved the issue where many POIs were displaying a generic camera icon instead of their category-specific icon (e.g., `Length` -> `arrow`, `peak` -> `mountain`).
    - Implemented a "Healing" mechanism in `POIManager.upsertInternal`: every POI now gets its icon validated and assigned immediately upon loading or ingestion.
    - This fix handles both "Internal Categories" (via hardcoded fallbacks for `Length`, `Area`, etc.) and Case-Sensitivity mismatches (via normalized config lookup).
- **Doc**: **System Flows Updated**
    - Updated `SYSTEM_FLOWS.md` with a new section documenting the Icon Assignment & Healing logic.

## v0.2.65 (2026-01-11)
- **Fix**: **Nameless POI Invalidation (Final Cleanup)**
    - Implemented strict name validation in `POIManager`. The system now automatically rejects any POI that lacks a valid English, Local, or User-language name.
    - This fix successfully removes "zombie" entries—nameless database records from older versions that were erroneously pinned to the map due to having been previously "played."
    - All existing "Unknown" entries on the map are now dropped from active tracking, ensuring a cleaner POI overlay.
- **Maintenance**:
    - Cleaned up experimental diagnostic scripts for Wikidata hydration.
    - Updated `pkg/poi` test suite to use properly named POIs for all pruning and eviction scenarios.

## v0.2.64 (2026-01-11)
- **Feature**: **Ground-Aware POI Filtering (The "Airport First" Rule)**
    - Centralized ground-aware filtering in `POIManager`. When the aircraft is on the ground (taxiing or parked), the system now strictly includes only POIs in the `Aerodrome` category.
    - This fix resolves the "shadowing" issue where high-scoring non-aviation POIs (e.g., "Innsbruck City") would prevent nearby airports ("Innsbruck Airport") from being narrated.
    - **API Autonomy**: The map display remains unaffected, continuing to show all landmarks and cities while on the ground by explicitly bypassing the ground-filter for the map API.
- **Refactor**: **Lean Narration Job**
    - Removed redundant and complex `checkGroundProximity` logic from `NarrationJob`. The job now relies on the `POIManager` to provide correctly filtered candidates based on the aircraft's `IsOnGround` state.
    - Simplified candidate selection pipeline and improved reliability for ground-based triggers.
- **Tests**: **Aviation-Specific Scenarios**
    - Updated `manager_test.go` and `narration_job_test.go` with specific table-driven tests for ground-mode Aerodrome priority.
    - Verified full interface compliance for all mock providers across the test suite.

## v0.2.63 (2026-01-11)
- **Refactor**: **Unified POI Thresholding & Filtering**
    - Implemented **Adaptive Filtering** mode: the system automatically adjusts the score threshold to show a target number of POIs (default 20).
    - **Persistence**: All POIs that have been narrated (`LastPlayed` is not zero) remain permanently visible on the map in blue.
    - **Zero Zombie Logic**: Removed `RecentlyPlayed` and "5-minute bypass" hacks. All freshness and visibility logic is now driven strictly by `LastPlayed` and physical LOS.
    - **Backend-Driven**: Filtering logic is now centralized in `POIManager`, ensuring 1:1 parity between map markers and narrator candidates.
    - **Pure Quality Score**: The `Scorer` now returns a pure quality/interest score without temporal penalties (cooldowns are handled by `NarrationJob`).
- **Refactor**: **Documentation Structure**
    - Removed all numerical prefixes from `SYSTEM_FLOWS.md` headers (e.g., "1. Wikidata..." -> "Wikidata...").
    - Updated internal cross-references to use named anchors for better stability.
- **Tests**: **Comprehensive Backend Coverage**
        - `POIManager`: Validated adaptive/fixed filtering and persistence logic.
        - `NarrationJob`: Verified `isPlayable` cooldown logic.
        - `api/config`: Confirmed persistence of new filter settings.
        - `api/pois`: Validated backend-to-frontend filtered delivery.

## v0.2.62 (2026-01-11)
- **Fix**: Resolved a deadlock in the `ReplayLast` mechanism caused by an early cancelled context.

## v0.2.61 (2026-01-11)
- **Feature**: **Valley Visibility (Effective AGL)**
    - Implemented a new visibility logic that calculates "Effective AGL" based on valley floor elevation.
    - **Logic**: Use the *Lowest* elevation point within a **dynamic radius** (determined by Max Visible Distance for XL POI) as the reference floor.
    - **Impact**: Aircraft flying at low AGL above a deep valley floor will now "see" POIs as if they were flying much higher, drastically boosting visibility for mountain flying.
    - **Map Overlay**: Updated the visibility heatmap API (`GET /api/map/visibility`) to match this logic.
- **Refactor**: **Scorer Session Pattern**
    - Introduced `scorer.Session` to optimize elevation lookups. The valley scan is performed once per scoring cycle (O(1)) and reused for all POIs.
    - **Dynamic Radius**: Switched from hardcoded 50km to precise Nautical Mile radius based on altitude.
- **Refactor**: **API Complexity Reduction**
    - Decomposed `VisibilityHandler.Handler` by extracting grid computation into `computeGrids` helper, reducing cyclomatic complexity to meet linting standards.
- **Documentation**: **Flow Specifications**
    - Updated `SYSTEM_FLOWS.md` Section 6. sections to reflect the new Effective AGL formula and dynamic scan radius.

## v0.2.60 (2026-01-10)
- **Fix**: **Nameless POI Filtering**
    - `service_enrich.go` now strictly drops POIs if they have no valid names (User, English, or Local).
    - Eliminates "Unknown" entities caused by source-filtering removing all sitelinks for unsupported languages (e.g., Russian-only nature reserves).


## v0.2.59 (2026-01-10)
- **Fix**: **Ground Narration Filter**
    - Updated `NarrationJob` to strictly filter for `Category == "Aerodrome"` when on the ground.
    - Prevents nearby non-aviation POIs (Castles, Villages) from triggering unrelated narrations during taxi/takeoff.

- **Fix**: **Strict Language Filtering**
    - `wbgetentities` now uses `sitefilter` to fetch only relevant languages (English + User + Local), preventing fallback to random languages (e.g., Russian).
- **Fix**: **Deterministic Hydration**
    - Removed random map iteration in `service_enrich.go`. Fallback sequence is now strictly priority-based.
- **Doc**: **System Flows Updated**
    - Updated `SYSTEM_FLOWS.md` to reflect the new hydration pipeline and efficiency gains.


## v0.2.57 (2026-01-10)
- **Refactor**: **Scheduler Split** (`pkg/core`)
    - Split `scheduler.go` (580 lines) into 3 focused files: `scheduler.go` (120), `jobs.go` (123), `narration_job.go` (341).
    - Cleaner imports, better testability, idiomatic Go file sizes.
- **Fix**: **Lone Wolf Detection Tightening**
    - Changed threshold from `score * 0.5` to `max(score * 0.2, 0.5)` in `pkg/narrator/skew.go`.
    - Makes it harder to be "lone hero" → more short narrations to cover POIs faster.
- **Tests**: **Coverage Improvements**
    - Added `jobs_test.go` with table-driven tests (~95% for jobs.go).
    - Added `eviction_job_test.go` for eviction logic (ShouldFire at 100%).
    - Added `version_test.go` with backend/frontend version sync check.
    - Improved `pkg/visibility` coverage: 63% → 91%.
- **Documentation**: Expanded Section 6 in `SYSTEM_FLOWS.md` with full scheduler architecture.

## v0.2.56 (2026-01-10)
- **Fix**: **POI Size Bias Tuning**
    - Added **Size Penalty** in `pkg/scorer/scorer.go`: L POIs now receive a 0.85x multiplier, XL POIs receive 0.70x.
    - Reduced category weights: Nature (1.3 → 0.9), Water (1.0 → 0.6) to prevent distant rivers/forests from drowning out nearby monuments.
    - These changes ensure smaller but culturally significant POIs (castles, monuments) can compete against distant geographic features.
- **Documentation**: **POI Scoring & Visibility System**
    - Added comprehensive Section 6.6 to `docs/SYSTEM_FLOWS.md` documenting the complete scoring formula.
    - Includes visibility table, distance decay formula, bearing multipliers, blind spot detection, and a worked example.

## v0.2.55 (2026-01-10)
- **Feature**: **Teleport Detection & Session Reset**
    - `Scheduler` now detects large position jumps (> `sim.teleport_distance`, default 80km) between ticks.
    - On teleport, registered `SessionResettable` components are reset: `Narrator` (clears trip summary), `POIManager` (clears candidates cache), `DynamicConfigJob` (resets regional context).
- **Feature**: **Ground/Inactive Narration Logic**
    - `NarrationJob` now checks `sim.GetState()` to block narration during menus/pause.
    - When on ground, narration is only allowed if the best POI is within 5km (e.g., departure airport on large airfields).
- **Refactor**: **Unified Distance Configuration**
    - Added `ft` (feet) unit support to `config.Distance` parser.
    - Renamed `MinSpawnAltitudeFt` → `MinSpawnAltitude` and `AltitudeFloorFt` → `AltitudeFloor` in `BeaconConfig` (now `Distance` type accepting any unit).
- **Testing**: Added `TestScheduler_TeleportDetection` and enhanced `TestNarrationJob_GroundSuppression` with distance-based checks.

## v0.2.54 (2026-01-10)
- **Feature**: **Strict Essay Triggering**
    - Implemented a rigorous "Gap Filler" logic for Regional Essays in `pkg/core/scheduler.go`.
    - **Priority Rule**: Essays only fire if NO viable POIs (Score > Threshold) are available.
    - **Silence Rule**: Essays require at least `2 * CooldownMax` (e.g., 60s) of prior silence to prevent overcrowding.
    - **Cooldown Rule**: Essays now respect the `narrator.essay.cooldown` config (default: 10m), enforced by a new `lastEssayTime` state tracker.
- **Testing**: **Table-Driven Scheduler Validation**
    - Added `TestNarrationJob_EssayRules` to verify priority, silence, and cooldown logic behaves exactly as specified.
    - Updated legacy tests to align with the new cooldown calculations.
- **Documentation**: **Flow Specifications**
    - Updated `SYSTEM_FLOWS.md` with the new "Intelligent Trigger (Strict Gating)" rules to ensure documentation matches the implemented logic.

## v0.2.53 (2026-01-10)
- **Refactor**: **Narrator Workflow Segmentation**
    - Decomposed the monolithic `service_ai_workflow.go` into modular components: `service_ai_poi.go` (POI Logic), `service_ai_essay.go` (Essay Logic), `service_ai_state.go` (State Management), and `service_ai_common.go` (Shared Helpers).
    - Significantly improved code maintainability and testability by isolating functional domains.
- **Optimization**: **Cache-First Wikidata Fetching**
    - Refactored `fetchTile` strategy to check the persistent cache *before* pre-calculating tile geometry or query strings.
    - Eliminates redundant CPU cycles and query construction overhead for tiles that are already locally available.
- **Documentation**: **Pipeline Correction**
    - Corrected the "Wikidata Tile Pipeline" flow in `SYSTEM_FLOWS.md` to accurately reflect the Cache -> Radius sequence.
- **Testing**: **Workflow Coverage**

## v0.2.52 (2026-01-10)
- **Place-Centric Rolling Summaries**: Refined summary prompt to eliminate directional cues and formulaic lists, focusing on narrative continuity.
- **Improved Summary Evolution**: Implemented logic to prune regional "filler" facts (like distance to national capitals) to prioritize unique session history.
- **Narrative Bridge Logic**: Instructed LLM to weave stories naturally (e.g., "Continuing along the coast...") instead of using repetitive introductory phrases.
- **Enhanced Test Coverage**: Increased statement coverage for `pkg/narrator` to 80.4% with new table-driven tests for mission memory consolidation.
- **Verification Experiment**: Created a dedicated experiment script to validate summary quality against historical flight logs.
- **Bug Fixes**: Restored missing `essay` profile in `phileas.yaml` and resolved linting issues in `service_ai.go`.


## v0.2.51 (2026-01-10)
- **Feature**: **Configurable Summary Limits**
    - Added `summary_max_words` (default: 500) to the narrator configuration, allowing users to control the depth of the trip memory.
    - Updated the summary update prompt to dynamically respect the configured limit.

## v0.2.50 (2026-01-10)
- **Feature**: **Rolling Trip Summaries (Phase 2)**
    - Replaced individual script history with a single, evolving Trip Summary. After each narration, the system consolidates the session memory in the background, maintaining a chronological account of everything discussed.
- **Feature**: **Context Continuity**
    - Gemini now receives a structured summary of the trip so far, enabling it to bridge stories between stops naturally and with higher factual density.
- **Optimization**: **Token Efficiency**
    - History is now consolidated into a summary (max 300 words), preventing the context window from growing indefinitely.
- **Prompt**: **Summary Update Template**
    - Created `summary_update.tmpl` with strict instructions for chronological ordering and thematic consolidation.
- **Architecture**: **Async Memory Updates**
    - The trip summary is updated in a non-blocking background task after each narration.

## v0.2.49 (2026-01-10)
- **Feature**: **Short-Term Memory (Context History)**
    - The narrator now maintains a session-wide memory of generated scripts. Every new prompt includes the last $N$ narrations, enabling the AI to cross-reference previous stops and build a cohesive "narrative arc."
- **Feature**: **Spatial Memory Pruning**
    - Integrated context history with the POI lifecycle. Scripts are only included in the prompt context if their corresponding POI is still spatially "tracked" (not evicted due to distance).
- **Refinement**: **Continuity Instructions**
    - Added specific guidance for Gemini to avoid repetition of facts and phrasing while emphasizing thematic expansion.
- **Refinement**: **Narrative Flow**
    - Instructed the AI to use phrases like "as we saw earlier" to bridge separate narrations into a continuous tour experience.
- **Config**: **Local Config**
    - Added `context_history_size` (default: 3) to `phileas.yaml` for fine-grained memory control.
- **Documentation**: **Architecture Guide (SYSTEM_FLOWS.md)**
    - Extensively refined the technical documentation. Professionalized the tone, removed persona-specific "Ava" branding in favor of functional terms like "The Narrator" and "Context Orchestration," and documented the new short-term memory architecture.
- **Cleanup**: **Verification Checklist**
    - Updated the internal verification list to reflect recent logic improvements.

## v0.2.48 (2026-01-10)
- **Feature**: **Manual Narration Length Control**
    - Implemented a segmented length selector (**S**, **M**, **L**) in the `POIInfoPanel` for manual narration triggers.
    - Updated the `/api/narrator/play` endpoint to accept a `strategy` parameter (min_skew, uniform, max_skew).
    - Enables users to force concise, standard, or detailed descriptions for specific POIs.
- **Documentation**: **Granular Narration Engine Details**
    - Extensively documented the **POI Narration Workflow** in `docs/SYSTEM_FLOWS.md`.
    - Detailed **Marker Beacon** lifecycle, high-frequency updates (~20Hz), and depth-clipping safety logic (Altitude Floor at 2000ft).
    - Documented the **Prompt Engine** data aggregation (telemetry, regional profiles, Wikipedia extracts).
    - Explained the **Dynamic Latency-Aware Prediction** which projects the plane's position based on a rolling average of observed selection-to-playback time.
    - Documented the **Skew Strategy** ("Lone Wolf" vs. "High Competition") used for automated narration.
- **Cleanup**: **UI Refinement**
    - Removed redundant volume slider from `InfoPanel.tsx` and associated legacy state logic.
    - Cleaned up `SYSTEM_FLOWS.md` by replacing Mermaid diagrams with human-readable textual flows for better preview compatibility.

## v0.2.47 (2026-01-09)
- **Refactor**: **Cleanup & Safety**
    - Removed legacy radius fallback (9.8km jump) in Wikidata fetching logic.
    - Simplified geospatial sampling to rely strictly on calculated tile radius.

## v0.2.46 (2026-01-09)
- **Fix**: **Geodata Cache Routing**
    - Resolved critical architectural flaw where `wd_h3_*` geodata entries were being stored in the generic `cache` table instead of the dedicated `cache_geodata` table.
    - Extended `Cacher` interface with `GetGeodataCache` and `SetGeodataCache` methods to ensure geodata and radius metadata are handled explicitly.
    - Added `PostWithGeodataCache` to the request client to route geodata requests correctly.
    - Updated `wikidata.client` to pass the calculated radius down to the cache layer, ensuring circles in UI are drawn with correct diameter.
    - Added `ListGeodataCacheKeys` to `store.Store` interface to correctly retrieve keys from the `cache_geodata` table.
- **Feature**: **Switch to H3 Resolution 6**
    - Increased grid resolution from 5 to **6** (~3.8km edge length) for finer geospatial granularity.
    - Adjusted tile spacing to 5.6km and updated grid radius calculations.
    - Updated all geospatial tests and assertions to match Res 6 geometry.
- **Feature**: **Provider-Based Backoff Strategy**
    - Implemented a more sophisticated exponential backoff with jitter, tracked independently per provider (domain).
    - Added gradual recovery: successful requests now slowly reduce the backoff delay instead of resetting it instantly, preventing "thundering herd" scenarios.
    - Backoff state now persists across the client lifecycle rather than being request-bound.
- **Config**: **Enhanced Request Settings**
    - Added `request.retries` (default: 5) and `request.timeout` to `phileas.yaml`.
    - Added `request.backoff` (base_delay, max_delay) for fine-grained control over retry timing.
- **Performance**: **Range Loop Optimization**
    - Updated `pkg/wikidata/merger.go` to use indexing in range loops over large `Article` structs (208 bytes), eliminating unnecessary memory copies and increasing merge throughput.
- **Logging**: **Reduced Console Noise**
    - Downgraded "SPARQL Query Completed" log from INFO to DEBUG.
    - Fixed several `gocritic` linting issues (unnamed results, pointer copies).

## v0.2.45 (2026-01-09)
- **Feature**: **Startup Health Checks**
    - Implemented a robust startup probe system (`pkg/probe`).
    - The application now validates critical dependencies (e.g., LLM Provider API Key and Model availability) before starting the server.
    - Added `HealthCheck` method to `llm.Provider` interface.
- **Refactor**: **Modernized Error Handling**
    - Standardized error handling in `pkg/wikidata` and `pkg/poi` using sentinel errors (`ErrNetwork`, `ErrPOINotFound`) and `fmt.Errorf("%w")` wrapping for better error inspection.
    - `poi.GetPOI` now explicitly returns `ErrPOINotFound` instead of `nil, nil` when a POI is missing.
- **Fix**: **Wikidata SPARQL Robustness**
    - Switched Wikidata SPARQL queries from `GET` to `POST` (form-urlencoded) to eliminate HTTP 414 (URI Too Long) errors for complex geospatial queries.
    - Updated `pkg/request` client to support caching for POST requests (`PostWithCache`).
- **Refactor**: **Wikidata Pipeline Optimization**
    - Split `pkg/wikidata` service into `pipeline`, `query`, and `hydration` components for better separation of concerns.
    - Implemented a "Cheap Query" strategy to fetch only essential data first, eliminating 503 errors caused by complex SPARQL joins.
    - Added a hydration step to fetch Labels and Titles via API only for valid candidates, significantly reducing timeout risk.
- **Testing**: **Coverage & Mocking**
    - Introduced `WikidataClient` and `WikipediaProvider` interfaces to enable robust, network-free table-driven tests.

## v0.2.44 (2026-01-09)
- **Feature**: **Mock Sim Terrain Following**
    - The Mock Simulator now automatically maintains a minimum altitude of **500ft AGL** above the terrain (using ETOPO1 data if available), effectively "following" the ground to prevent collisions during unattended simulations.
- **Refactor**: **Relaxed LOS Tolerance**
    - Increased the Line-of-Sight blockage tolerance to **50 meters**. This prevents false "blocked by terrain" results when the Line-of-Sight ray grazes the ground or water surfaces due to minor ETOPO1 resolution inaccuracies.

## v0.2.43 (2026-01-09)
- **Refactor**: **Optimized Auto-Narration Frequency**
    - Decoupled the `NarrationJob` from the high-frequency telemetry loop (100ms).
    - It is now event-driven, triggered via callback immediately after the POI Scorer completes (every 5 seconds).
    - **Optimization**: The Line-of-Sight (LOS) check now aborts early if it encounters a POI below the score threshold, significantly reducing CPU usage (since candidates are pre-sorted).
    - **Logging**: Added deduplication to the "All POIs blocked by LOS" log message; it now only appears when the count of visible candidates changes, eliminating console spam.
- **Config**: **Configurable Essays**
    - Added `settings.narrator.essay.enabled` (default: `true`) to `phileas.yaml`.
    - Allows disabling regional essays entirely via configuration.
- **Feature**: **Fatal LLM Configuration Error**
    - The application now exits fatally (code 1) if the LLM client is not configured when a narration request is made, preventing "zombie" states where requests silently fail.
- **Testing**: **Improved Mock Simulator**
    - The Mock Sim now dynamically adjusts its flight profile altitudes based on the starting airfield elevation, ensuring relevant visibility testing regardless of starting terrain height.
- **Config**: **Beacon Settings**
    - Removed hardcoded constants for Beacon formation and triggering distances.
    - These values are now fully configurable via `phileas.yaml` (under `beacon` section) to allow fine-tuning of visual behavior.

## v0.2.42 (2026-01-09)
- **Testing**: Increased `pkg/store` test coverage.

## v0.2.41 (2026-01-09)
- **Refactor**: **Store Interface Segregation**
    - Split the monolithic `store.Store` interface (19 methods) into 8 focused sub-interfaces:
        - `POIStore` (5 methods) — POI CRUD operations
        - `CacheStore` (4 methods) — Generic key-value caching
        - `GeodataStore` (2 methods) — Geodata cache with radius metadata
        - `HierarchyStore` (4 methods) — Wikidata classification hierarchy
        - `ArticleStore` (2 methods) — Wikipedia article persistence
        - `SeenEntityStore` (2 methods) — Negative cache for seen entities
        - `MSFSPOIStore` (3 methods) — MSFS POI data
        - `StateStore` (2 methods) — Persistent application state
    - `Store` now composes from all sub-interfaces (fully backward compatible).
    - Updated `classifier.Classifier` to depend on `store.HierarchyStore` instead of full `Store`.
    - Updated `poi.Manager` to depend on `poi.ManagerStore` (combines `POIStore` + `MSFSPOIStore`).
    - **Benefits**: Improved testability (mocks need fewer methods), clearer documentation, compile-time safety.

## v0.2.40 (2026-01-09)
- **Feature**: **Line-of-Sight (LOS) for POI Selection**
    - Implemented terrain-aware POI filtering during auto-narration.
    - The narrator now checks if a POI is visible from the aircraft or blocked by terrain (mountains) before selecting it.
    - Uses **ETOPO1** elevation data (1 arc-minute resolution) with 0.5km ray-marching steps.
    - LOS is enabled by default (`terrain.line_of_sight: true`).
- **Config**: **New `terrain` Configuration Section**
    - Added `line_of_sight` setting to enable/disable LOS checks.
    - Added `elevation_file` setting to configure the path to the ETOPO1 binary data file.
    - Default path: `data/etopo1/etopo1_ice_g_i2.bin`.
- **Instrumentation**: Added comprehensive debug logging for:
    - POI selection path (`ShouldFire`, `getVisibleCandidate`).
    - Manual play API (`HandlePlay`, `PlayPOI`).
    - LOS terrain blocking decisions.

## v0.2.39 (2026-01-08)

- **Improvement**: **Eviction Job Optimization**
    - Reduced eviction frequency from 30s to **300s** (5 minutes) to prevent aggressive cache clearing.
    - Added **Ground Safety Check**: Eviction is now skipped when the aircraft is on the ground (parked or taxiing), ensuring loaded POIs remain available during turnaround.

## v0.2.38 (2026-01-08)
- **Fix**: **Remote Narration After Teleport**
    - Implemented a **Location Consistency Check** in the POI Scheduler.
    - The Scheduler now verifies that the `POIManager`'s scores were calculated for the aircraft's *current* location (within 10km) before triggering narration.
    - Resolves the issue where stale high scores from a previous location (or startup coordinates) caused distant POIs (e.g., 350km away) to be narrated immediately after spawning or teleporting.

## v0.2.37 (2026-01-08)
- **Feature**: **Cooldown Skew Mechanism**
    - Implemented a unified skew strategy (Min/Max/Uniform) based on POI density (rival count).
    - Ensures `MaxWords` and subsequent `Cooldown` are consistent for each narration.
    - Centralized skew logic in `pkg/narrator/skew.go`.

## v0.2.36 (2026-01-08)
- **Fix**: **Frontend Stats Display**
    - `InfoPanel` now correctly displays statistics for fallback TTS providers (e.g. `edge-tts`) even when they are not the primary configured engine.

## v0.2.35 (2026-01-08)
- **Fix**: **Edge TTS Connectivity (Sec-MS-GEC)**
    - Resolved `websocket: bad handshake` (403 Forbidden) errors by implementing the required `Sec-MS-GEC` token generation and `MUID` cookie usage.
    - Updated `pkg/tts/edgetts` to use correct URL parameters for authentication.
- **Feature**: **Azure TTS Fallback**
    - Introduced automatic fallback to Edge TTS when Azure Speech returns fatal errors (4xx/5xx).
    - Session-scoped fallback ensures narration continues even during Azure outages or rate limits.
    - Skips the current POI on fallback to prevent SSML/Prompt mismatch.

## v0.2.34 (2026-01-08)
- **Fix**: **Dynamic Prediction Window**
    - Fixed regression where aircraft position prediction was stuck at 60s instead of adapting to observed LLM+TTS latency.
    - `updateLatency()` now calls `SetPredictionWindow(avg)` to complete the feedback loop.
- **Maintenance**: **Reduced Log Spam**
    - Commented out high-frequency DEBUG logs: "Job firing", "Merged POI", "Insufficient sitelinks", "Article dropped", "Traversing hierarchy", "Ignored category found in hierarchy".

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
- **Thumbnail Logic**: Switched from heuristic filtering to LLM-based Smart Selection for POI thumbnails. This ensures better selection of aerial and representative photos while avoiding maps and logos (e.g., Lechtaler Alpen map issue).
- **Backend**: `pkg/core` tests fixed for "Start Airborne" bypass logic.
- **Frontend**: Fixed Info Panel focus stealing issues.
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
