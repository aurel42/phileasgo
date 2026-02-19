# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & development

From `internal/ui/web/`:
```bash
npm run dev      # Vite dev server with HMR
npm run build    # tsc + Vite production build → ../dist
npm run test     # vitest run --coverage
npm run lint     # ESLint (flat config)
```

Vite splits `maplibre-gl` into its own chunk; everything else goes to `vendor`. Chunk warning threshold is 1024 KB.

## Architecture

**Stack**: React 19 + TypeScript, React Router v7 (hash-based), TanStack Query v5, Vite 7.

**Pages**:
- `/` — main map interface (`App.tsx`)
- `/settings` — configuration panel (lazy-loaded)
- `/overlay` — streaming overlay (`OverlayPage.tsx`): a minimal floating POI card + small map

`App.tsx` is the state hub: it runs all data hooks, owns `selectedPOI`, syncs config with the backend every 2 s, and conditionally renders one of the two map components.

**Data layer** — all hooks poll the Go backend at `http://127.0.0.1:1920`:
| Hook | Endpoint | Purpose |
|------|----------|---------|
| `useTelemetry` | `/api/telemetry` | Aircraft state, SimState |
| `useTrackedPOIs` | `/api/pois/tracked` | POI list |
| `useNarrator` | `/api/narrator/status` | Playback state |
| `useTripEvents` | `/api/telemetry/trip-events` | Replay history |
| `labelService` | `/api/labels` | Settlement labels |

Config lives on the backend (`/api/config` GET/PUT) so multiple tabs stay in sync; GUI-only state (e.g. `showArtisticDebugBoxes`) goes in `localStorage`.

**Replay mode** is entered whenever `SimState === 'disconnected' && tripEvents.length > 1`. Both maps implement replay differently (see below).

---

## The two maps

Map selection in `App.tsx`:
```tsx
activeMapStyle === 'artistic' ? <ArtisticMap ... /> : <Map ... />
```

### Map.tsx — Leaflet (interactive)

- **Library**: `react-leaflet` v5 wrapping `leaflet` v1.9.4; CARTO Dark tiles.
- **Rendering**: React component tree; layers are React children of `<MapContainer>`.
- **Camera**: auto-follows aircraft (25% forward offset in heading direction). Auto-zoom watches POI proximity. Manual pan/zoom disables auto-zoom until re-enabled.
- **Zoom ranges** differ by mode: `[8, 13]` during connected flight, `[2, 18]` while paused, `[0, 12]` in replay.
- **Key layers**: `SmartMarkerLayer` (per-frame POI collision avoidance), range rings, rotating aircraft marker, optional `VisibilityLayer` and `CacheLayer`.
- **Replay**: `TripReplayOverlay` overlaid on the map; animates the aircraft along recorded events.

### ArtisticMap.tsx — MapLibre GL (heartbeat-controlled)

- **Library**: `maplibre-gl` v5 used **directly** (no React wrapper). The map instance lives in a ref; all mutations go through the MapLibre API, not React state.
- **Tiles**: Stamen Watercolor HD (128 px, parchment look) + DEM hillshading + OSM vector runways.
- **Discrete "Real Zoom"**: the camera only snaps to integer zoom levels. A float "Hypothetical Zoom" from telemetry drives snapping. This is the most important design rule — **never scale icons continuously**; the discrete step prevents icon bloat and preserves the parchment aesthetic.
- **Permanence**: POIs are "stamped" at the integer zoom level when first discovered (`Z_placed`). Icon scale = `2^(Z_current − Z_placed)`, so they shrink as the camera zooms out — giving a visual history of altitude changes.
- **Heartbeat** (`setInterval` 2000 ms): the single source of truth that updates camera, placement, visibility fog, and labels. All closure values are read from refs (not state) to avoid stale-closure bugs in the interval callback.
- **Dead-zone panning**: the map stays locked until the aircraft exits a centre circle; then the camera repositions.
- **Label placement** (`PlacementEngine.ts`): R-tree (`RBush`) spatial index + priority queue (Playing > Preparing > Settlements > Discovered > New). Anchors are tried in 8 positions; overflow falls back to radial placement. Labels fade in over 2 s. Placement is re-solved on every zoom snap and state transition.
- **Replay**: all item placements pre-computed once at start (expensive). `InkTrail` animates the path. POIs appear only after the aircraft historically discovered them; balloon position interpolates between events.

### Key differences at a glance

| | `Map.tsx` | `ArtisticMap.tsx` |
|--|-----------|-------------------|
| Library | react-leaflet | maplibre-gl (raw) |
| Zoom | Continuous, user-controlled | Integer snaps only |
| Icon scale | Grows with zoom | Shrinks after discovery |
| Label solving | Per-frame (SmartMarkerLayer) | Per-snap (PlacementEngine, cached) |
| Updates | React re-renders | Heartbeat writes to refs/MapLibre API |
| Aesthetic | Modern dark | Historic parchment |

---

## src layout (non-obvious parts)

```
src/
  metrics/
    PlacementEngine.ts   # RBush R-tree + d3-force label solver
    text.ts              # canvas font measurement
  styles/
    artisticMapStyles.ts # colour schemes
    artisticMapConfig.ts # MapLibre style spec (watercolor, hillshade, runways)
  utils/
    artisticMapUtils.ts  # font adjustment, replay helpers
    poiUtils.ts          # POI filtering & scoring
  types/
    artisticMap.ts       # MapFrame — atomic heartbeat state
```

Heavy components (`Map`, `ArtisticMap`, `SettingsPanel`) are `React.lazy`-loaded.
