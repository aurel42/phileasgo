---
name: phileasgo-backend
description: >
  Go backend skill for PhileasGo — an intelligent co-pilot/tour guide for Flight Simulators.
  Use when working on Go code in this project: API endpoints, store methods, scheduler jobs,
  narrator pipeline, POI management, config, or any pkg/internal code. Covers coding standards,
  architecture, critical patterns that prevent common mistakes, and step-by-step workflows.
trigger: model_decision
---

# PhileasGo Go Backend

## Domain Context

**PhileasGo** connects to MSFS (SimConnect), tracks aircraft telemetry, fetches POIs from
Wikidata/WP, generates AI narration (LLM), and plays audio (TTS).

**Hard constraints:**
- **Non-blocking** — The flight loop (`Scheduler.tick`) is critical. Network/LLM/TTS calls
  must run in goroutines. Never block the tick path.
- **Resilient** — Handle offline modes and API failures gracefully. Narrator degrades to
  `StubService`; config falls back to static YAML.
- **Mock-first** — All code must run with the Mock Simulator for headless development.
  Use interface-based DI so every dependency is swappable.

## Coding Standards

### Language & Layout
- **Go 1.22+**, idiomatic style.
- `cmd/` — Main entry points. `cmd/experiments/` — Temporary test/debug scripts.
- `pkg/` — Reusable library code (business logic).
- `internal/` — Private application code (API handlers, specific implementations).
- `configs/` — YAML configuration files.
- `data/` — Supporting data files (downloaded or generated).

### Naming
- **PascalCase** for exported, **camelCase** for unexported. **No snake_case.**
- Short, descriptive variable names: `ctx` not `context`, `err` not `error`.

### Error Handling
- Wrap with `fmt.Errorf("doing X: %w", err)`.
- Log with `log/slog` (structured). **Never panic** in request paths — return errors.

### Concurrency
- `sync.RWMutex` for shared state (see `pkg/poi/manager.go`).
- `context.Context` for cancellation and timeouts.
- Goroutines for long-running tasks (LLM generation, scoring, DB writes).

### Dependency Injection
- Interface-based constructor injection: `NewManager(cfg, store, ...)`.
- Define interfaces near the consumer (Consumer pattern), not near the implementation.
- Narrow interfaces: consumers declare only the methods they need.

## Architecture Map

### Core Loop — `pkg/core/scheduler.go`
`Scheduler` holds a slice of `Job` implementations and a list of `SessionResettable` components.
On each `tick()`, it iterates jobs, calls `ShouldFire(*Telemetry)`, and launches `go job.Run()`
for those that fire. Teleport detection compares tick positions and triggers `ResetSession` on
all resettables.

**`Job` interface:**
```go
Name() string
ShouldFire(t *sim.Telemetry) bool
Run(ctx context.Context, t *sim.Telemetry)
```

**Built-in job types (`pkg/core/jobs.go`):**
- `BaseJob` — `atomic.Int32` running guard with `TryLock()`/`Unlock()` to prevent re-entry.
- `DistanceJob` — Fires when aircraft moves beyond a distance threshold from last fire position.
- `TimeJob` — Fires when a duration elapses since last fire time.

Both accept a `func(context.Context, sim.Telemetry)` closure. Business logic (narration,
eviction, river detection, etc.) is injected at wiring time.

Concrete jobs: `narration_job.go`, `eviction_job.go`, `river_job.go`, `restore_job.go`,
`dynamic_config_job.go`, `transponder_watcher_job.go`.

### POI Management — `pkg/poi/manager.go`
`Manager` owns the in-memory `trackedPOIs map[string]*model.POI` cache, protected by
`sync.RWMutex`. It depends on a narrowed `ManagerStore` interface:
```go
type ManagerStore interface {
    store.POIStore
    store.MSFSPOIStore
    store.StateStore
}
```

Key behaviors:
- `UpsertPOI` / `TrackPOI` both call `upsertInternal` which does **in-place field copy**
  via `updateExistingPOI` to maintain pointer stability.
- `GetNarrationCandidates` — filters by cooldown, visibility, and score.
- `PruneTracked` / `PruneByDistance` — eviction by age or distance + rear-semicircle bearing.
- Setter injection for optional components: `SetPOILoader`, `SetRiverSentinel`,
  `SetScoringCallback`, `SetValleyAltitudeCallback`.

### Narrator Pipeline — `pkg/narrator/`
Three-tier architecture:
1. **`Service`** (interface in `service.go`) — ~40 methods: start/stop, play, queue,
   playback state, cooldowns, session reset.
2. **`Orchestrator`** (in `orchestrator.go`) — Implements `Service`. Composes a `Generator`
   and `audio.Service`. Manages playback queue (`playback.Manager`) and beacon colors.
3. **`AIService`** (in `service_ai*.go`) — Implements `Generator`. Real LLM + TTS pipeline:
   prompt building, generation queue, essay system, state management.
4. **`StubService`** — No-op implementation for tests and headless/mock mode.

Key interfaces (in `interfaces.go`):
- `POIProvider` — Consumed by AIService; satisfied by `poi.Manager`.
- `GeoProvider`, `WikipediaProvider`, `LanguageResolver`, `BeaconProvider`.
- `Generator` — Consumed by Orchestrator; satisfied by AIService.

`GenerationRequest` carries all parameters for a single narration: type, POI, prompt data,
coordinates, essay topic, two-pass flag, etc.

### Store — `pkg/store/`
**Sub-interfaces** (in `interfaces.go`), each with `context.Context` first arg:

| Interface | Purpose |
|---|---|
| `POIStore` | CRUD for POIs, recently-played queries, cooldown persistence |
| `CacheStore` | Generic key-value byte cache |
| `GeodataStore` | Spatial geodata cache with lat/lon bounds queries |
| `HierarchyStore` | Wikidata hierarchy and classification lookups |
| `ArticleStore` | WP article storage |
| `SeenEntityStore` | Tracks seen Wikidata entities per session |
| `MSFSPOIStore` | MSFS-native POI storage with spatial check |
| `StateStore` | Key-value string store (config overrides, session state) |

**Composed interface** (in `sqlite.go`):
```go
type Store interface {
    POIStore; CacheStore; GeodataStore; HierarchyStore
    ArticleStore; SeenEntityStore; MSFSPOIStore; StateStore
    Close() error
}
```

`SQLiteStore` is the sole implementation. Consumers depend on specific sub-interfaces,
not the full `Store`. Full `Store` is only used at the wiring layer (`cmd/`).

### Config — `pkg/config/provider.go`
`UnifiedProvider` composes static `*Config` (from YAML) with `store.StateStore` (runtime
overrides). Every method calls a typed helper (`getString`, `getInt`, `getFloat64`, `getBool`,
`getDuration`). Each helper checks `StateStore` first; on miss, falls back to static config.
This allows any setting to be overridden at runtime via the API without restart.

### API — `internal/api/server.go`
Go 1.22 `http.NewServeMux` with `METHOD /path` routing. All handler objects injected via
`NewServer(...)` constructor. Optional handlers guarded with `if h != nil`. CORS middleware
allows `coui://html_ui` (MSFS EFB WebView origin).

### Sim — `pkg/sim/client.go`
`Client` interface: `GetTelemetry`, `GetState`, `GetLastTransition`, `SetPredictionWindow`,
`Close`, persistence methods, and `ExecuteCommand`. Separate `ObjectClient` interface for
AI traffic (spawn/remove/position). Sentinel errors: `ErrNotConnected`, `ErrWaitingForTelemetry`.

### Models — `pkg/model/`
- `POI` — Identity (WikidataID, Category, Icon), coordinates, names (En/Local/User with
  `DisplayName()` priority), scoring fields, cooldown state, optional `RiverContext`.
- `Narrative` — Type, Title, Script, AudioPath, Duration, latency, associated POI, essay metadata.
- `NarrativeType` — String enum: `"poi"`, `"essay"`, `"screenshot"`, `"debriefing"`, `"border"`,
  `"letsgo"`, `"briefing"`.
- `sim.Telemetry` — Lat, Lon, altitudes, heading, speeds, ground/engine/AP state, squawk.
- `LocationInfo` — City, country, admin1, zone.

## Development Workflow

- **Build**: `make build` (builds API and UI).
- **Test**: `make test` (runs unit tests + lint).
  - **Table-driven tests** for logic. `_test.go` files alongside source.
- **Versioning**: Semantic versioning in `pkg/version/version.go`.
  - **Always** bump version on release. Update `CHANGELOG.md`.

## Critical Patterns

### Interface Propagation Chain
Adding a method to a store or provider interface requires updating every mock that
implements it. **Check all these locations:**

**`store.POIStore`** (or any sub-interface in `store/interfaces.go`):
1. Interface definition → `pkg/store/interfaces.go`
2. SQLite implementation → `pkg/store/sqlite.go`
3. Mocks → `pkg/narrator/mocks_dev_test.go` (`MockStore`),
   `pkg/wikidata/service_test.go` (`mockStore`),
   and any other `_test.go` that declares a store mock.

**`narrator.POIProvider`** (in `pkg/narrator/interfaces.go`):
1. Interface definition → `pkg/narrator/interfaces.go`
2. Real implementation → `pkg/poi/manager.go`
3. Mocks → `pkg/narrator/mocks_dev_test.go` (`MockPOIProvider`),
   `pkg/core/narration_job_test.go`, and other test files with `MockPOIProvider`.

**Recipe:** After adding a method, run `go build ./...` — the compiler will flag every
unsatisfied interface. Fix each one before moving on.

### Pointer Stability in trackedPOIs
`trackedPOIs` uses stable pointers. When a POI is updated, `updateExistingPOI` copies
fields **onto the existing `*model.POI`** so downstream references (in-progress narrations,
scoring callbacks) remain valid. **Never replace a pointer** in the map — always mutate
the existing one.

### Store Composition
`Store` (in `sqlite.go`) embeds all 8 sub-interfaces + `Close()`. Consumers declare
**narrow, composed interfaces** scoped to their needs:
```go
// in pkg/poi/manager.go
type ManagerStore interface {
    store.POIStore
    store.MSFSPOIStore
    store.StateStore
}
```
At wiring time, pass the full `*SQLiteStore` — it satisfies any composition of sub-interfaces.

### Config Layering
`UnifiedProvider` checks `store.StateStore` first, falls back to static `*Config`. Typed
helpers: `getString`, `getInt`, `getFloat64`, `getBool`, `getDuration`, `getDistance`,
`getStringSlice`, `getOptionalString`. When adding a new config method:
1. Add to `Provider` interface (`provider.go`).
2. Implement on `UnifiedProvider` using the appropriate helper.
3. Add the config key to the YAML struct if it needs a default.

### Job System
Jobs use `BaseJob` with an `atomic.Int32` running guard (`TryLock()`/`Unlock()`) to prevent
overlapping executions. Two built-in types:
- `DistanceJob` — fires when distance from last fire position exceeds threshold.
- `TimeJob` — fires when duration since last fire exceeds threshold.

Both wrap a plain `func(context.Context, sim.Telemetry)` closure. To create a new job,
either use these types with a closure, or implement the `Job` interface directly.

### Mock Pattern
**Func-field structs** with nil-check delegation and zero-value defaults:
```go
type MockPOIProvider struct {
    GetPOIFunc func(ctx context.Context, qid string) (*model.POI, error)
    // ...
}
func (m *MockPOIProvider) GetPOI(ctx context.Context, qid string) (*model.POI, error) {
    if m.GetPOIFunc != nil { return m.GetPOIFunc(ctx, qid) }
    return nil, nil
}
```

For large interfaces (like `narrator.Service`), use **embedding over full rewrite**:
embed `narrator.StubService` and override only the methods under test.

### Non-Blocking DB Writes
Fire-and-forget persistence for non-critical writes:
```go
go pm.SaveLastPlayed(context.Background(), poiID, time.Now())
```
Use `context.Background()` (not the request context) so the write survives request
cancellation. Only use this for writes where failure is tolerable (cooldown timestamps,
seen entities). Critical writes must be synchronous with error handling.

## Common Workflows

### Add a Store Method

1. **Interface** — Add the method signature to the appropriate sub-interface in
   `pkg/store/interfaces.go`.
2. **SQLite impl** — Implement on `*SQLiteStore` in `pkg/store/sqlite.go` (or the
   relevant `sqlite_*.go` file for that domain).
3. **Mocks** — `go build ./...` will show every test mock that needs the new method.
   Add a func-field + nil-check delegation to each.
4. **Consumer** — Call the new method from the consuming package.

### Add an API Endpoint

1. **Handler** — Create or extend a handler struct in `internal/api/`. Follow existing
   patterns: parse request, call service, write JSON response.
2. **Route** — Register in `internal/api/server.go` with `mux.HandleFunc("METHOD /path", h.Method)`.
3. **Wiring** — If the handler needs a new dependency, add it to the `NewServer` constructor
   and wire it in `cmd/`.

### Add a Scheduler Job

1. **Choose type** — `DistanceJob` (spatial trigger) or `TimeJob` (temporal trigger),
   or implement `Job` directly.
2. **Create** — In `pkg/core/`, create a constructor function that returns the job.
   Wire business logic via a closure or method reference.
3. **Register** — Add to the `Scheduler.jobs` slice during wiring in `cmd/`.
4. **Guard** — `BaseJob.TryLock()`/`Unlock()` prevents overlapping runs automatically
   for `DistanceJob`/`TimeJob`. If implementing `Job` directly, embed `BaseJob` and
   use the same guard.
