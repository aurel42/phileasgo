---
name: phileasgo-webapp
description: >
  React 19 + TypeScript web frontend for PhileasGo — the Victorian co-pilot/tour guide for
  Flight Simulators. Use when working on the web UI: components, hooks, map layers, styling,
  state management, or API integration in `internal/ui/web/`. Covers two completely separate
  map implementations (Leaflet dark map, MapLibre artistic map), the placement engine, and
  the Victorian/steampunk aesthetic system.
trigger: model_decision
---

# PhileasGo Web Frontend

## Stack & Tooling

- **React 19** with TypeScript (strict mode, `erasableSyntaxOnly` in tsconfig)
- **Vite 7** — dev server with HMR, build emits to `internal/ui/dist/` (embedded into Go binary via `//go:embed`)
- **Vitest 4** — unit tests (`*.spec.ts` files colocated with source)
- **ESLint 9** — flat config (`eslint.config.js`), `@eslint/js` + `typescript-eslint` + `eslint-plugin-react-hooks`
- **TanStack Query v5** — server-state polling
- **Key libraries**: react-leaflet v5, maplibre-gl v5, d3-force v3, turf.js, rbush (R-tree)

## Directory Layout

```
internal/ui/web/src/
├── main.tsx              # React root, HashRouter, QueryClientProvider
├── App.tsx               # Single state hub (~50 useState/hooks for config)
├── OverlayPage.tsx       # Streaming overlay mode (secondary monitor)
├── components/           # PascalCase .tsx files
├── hooks/                # camelCase: useTelemetry, usePOIs, useNarrator, useAudio, useGeography, useTripEvents
├── services/             # camelCase: labelService (POST /api/map/labels/sync)
├── types/                # camelCase: telemetry, narrator, audio, mapLabels, artisticMap
├── styles/               # artisticMapConfig.ts (MapLibre StyleSpecification), artisticMapStyles.ts
├── metrics/              # PlacementEngine.ts (R-tree solver), text.ts (canvas font measurement)
├── utils/                # poiUtils.ts, replay.ts, artisticMapUtils.ts
├── assets/               # Static assets
├── index.css             # Global styles (~780 lines)
├── theme.css             # CSS custom properties (colors, fonts, spacing)
└── fonts.css             # Web font imports (Pinyon Script for Victorian)
```

## Conventions

- **File naming**: PascalCase for components (`Map.tsx`), camelCase for hooks/utils/services/types (`usePOIs.ts`), `*.spec.ts` for tests.
- **Exports**: Named exports for components. Default exports only for pages (`App.tsx`, `OverlayPage.tsx`).
- **Props**: Interfaces defined locally in the component file, not shared.
- **Imports**: `import type { ... }` enforced by `verbatimModuleSyntax`. No path aliases — relative imports only.
- **Icons**: `lucide-react` for UI chrome icons. POI category icons are inlined SVGs loaded via `InlineSVG.tsx`.
- **Tests**: Colocated `*.spec.ts` files. Run with `npm test` (Vitest).

## State Management

No global store. **App.tsx is the single state hub** (~25 `useState` for config values, plus derived state).

- **Server state**: TanStack Query v5 hooks handle all polling. Backend is source of truth for config.
- **Config sync**: Polled every 2s via `GET /api/config`. Changes sent via `PUT /api/config` with optimistic local update.
- **`localStorage`**: Only for GUI-local prefs (not config values).

## API Communication

REST polling to the embedded Go server. All hooks use TanStack Query.

| Endpoint | Method | Hook | Interval | Purpose |
|---|---|---|---|---|
| `/api/telemetry` | GET | `useTelemetry` | 500ms | Aircraft position, heading, altitude, SimState |
| `/api/pois/tracked` | GET | `usePOIs` | 5s | Filtered POI list (backend-sorted) |
| `/api/narrator/status` | GET | `useNarrator` | 1s | Playback state, current/preparing POI |
| `/api/audio/status` | GET | `useAudio` | 1s | Playback position, volume, title |
| `/api/audio/control` | POST | `useAudio.control()` | manual | Play, pause, stop, replay |
| `/api/audio/volume` | POST | `useAudio.setVolume()` | manual | Set master volume |
| `/api/geography` | GET | `useGeography` | 10s | Reverse geocoding (city, country) |
| `/api/trip/events` | GET | `useTripEvents` | manual | Recorded telemetry for replay |
| `/api/config` | GET/PUT | App.tsx polling | 2s | All configuration read/write |
| `/api/map/labels/sync` | POST | `labelService` | heartbeat (2s) | Settlement labels for artistic map |
| `/api/map/visibility` | GET | `VisibilityLayer` | 15s + moveend | Scored visibility grid |
| `/api/map/coverage` | GET | `CoverageLayer` | once on mount | Coverage footprint circles |
| `/api/version` | GET | `InfoPanel` | 5s | Backend version string |
| `/api/stats` | GET | `InfoPanel` | 5s | Diagnostic statistics |

## Routing

React Router v7, hash-based (`HashRouter` in `main.tsx`). Three routes:

| Route | Component | Notes |
|---|---|---|
| `/` | `App` | Main view: map + sidebar dashboard |
| `/settings` | `SettingsPanel` | Full config editor (40+ settings) |
| `/overlay` | `OverlayPage` | Streaming overlay for secondary monitor |

All route components are `React.lazy()` with dark `Suspense` fallback.

## Styling

Plain CSS with CSS custom properties defined in `theme.css`. Victorian/steampunk aesthetic:

- **Accent**: `--accent: #d4af37` (brass gold)
- **Borders**: `3px double rgba(212, 175, 55, 0.3)` (Victorian frame)
- **Shadows**: Deep inner shadow + brass glow (`inset 0 0 20px rgba(0,0,0,0.5)`)
- **Fonts**: `--font-main` (Georgia/serif), `--font-display` (Inter/sans), Pinyon Script (calligraphic headings)
- **Text**: `--text-color: #e5e0d5` (parchment white on dark)
- **Responsive**: `@media (orientation: portrait), (max-width: 900px)` — stacks map above sidebar

## Map System

Two completely separate map implementations selected via `activeMapStyle` config:

| Map | Library | Style | File |
|---|---|---|---|
| **Dark Map** | react-leaflet v5 / Leaflet 1.9.4 | CARTO Dark tiles | `Map.tsx` |
| **Artistic Map** | maplibre-gl v5 (raw, no wrapper) | Stamen Watercolor + hillshade | `ArtisticMap.tsx` |

Both are lazy-loaded. They share almost no code — different rendering models, different marker systems, different camera control.

See reference files for details:
- [`references/dark-map.md`](references/dark-map.md) — Leaflet dark map
- [`references/artistic-map.md`](references/artistic-map.md) — MapLibre artistic map

## Critical Patterns

### Ref-Based Heartbeat (Artistic Map)

ArtisticMap reads **all state from refs** inside a 2s `setInterval`. Never read React state directly inside the interval — it captures stale closures. Props are synced to refs in `useEffect`, and the heartbeat reads only from refs.

```tsx
// CORRECT: read from ref inside interval
const poisRef = useRef(pois);
useEffect(() => { poisRef.current = pois; }, [pois]);
setInterval(() => { const currentPois = poisRef.current; ... }, 2000);

// WRONG: reading state directly captures stale value
setInterval(() => { const currentPois = pois; ... }, 2000); // stale!
```

### PlacementEngine (R-tree Solver)

`metrics/PlacementEngine.ts` — greedy label placement with RBush spatial index.

**Two-phase algorithm:**
1. **Locked items** (already placed in previous tick): force-inserted at cached positions, collision boxes scaled by `2^(currentZoom - placedZoom)`.
2. **New items** (never placed): try true position first → 8-point anchor search (top-right preferred) → radial step-search fallback.

**Priority order**: landmarks (300) > cities (100) > towns (95) > villages (90) > active POIs by size (85→55) > historical POIs.

Cache survives across heartbeats; resets only on zoom snap.

### Zoom Snapping (Artistic Map)

Artistic map uses **discrete integer zoom** (rounds to nearest 0.5-step). Never use float hypothetical zoom for rendering. POI markers are "stamped" at discovery zoom; their visual scale is `2^(currentZoom - placedZoom)`.

### Dead-Zone Panning (Artistic Map)

Aircraft triggers a camera pan **only when it exits a dead-zone circle** (heading-forward offset from center). Prevents camera jitter during stable flight. The dead zone is a percentage of viewport size centered ahead of the aircraft's current heading.

## Common Workflows

### Add a New Component

1. Create `src/components/MyComponent.tsx` (PascalCase).
2. Define props interface locally. Use named export.
3. Import with `import { MyComponent } from '../components/MyComponent'`.
4. Style in `index.css` or component-scoped CSS file if complex.

### Add a New Polling Hook

1. Create `src/hooks/useMyData.ts`.
2. Use TanStack Query pattern:
   ```tsx
   import { useQuery } from '@tanstack/react-query';
   import type { MyType } from '../types/myType';

   export const useMyData = () => {
     return useQuery<MyType>({
       queryKey: ['myData'],
       queryFn: () => fetch('/api/my-endpoint').then(r => r.json()),
       refetchInterval: 5000,
     });
   };
   ```
3. Add type definition in `src/types/myType.ts`.

### Add a New API Endpoint Consumer

1. Add the endpoint to the table above.
2. Create a hook in `src/hooks/` (for polling) or a service in `src/services/` (for one-shot/POST requests).
3. Wire into `App.tsx` or the consuming component.
4. For config values: add `useState` in `App.tsx`, read from config poll, write via `PUT /api/config`.

## Build & Test

```bash
cd internal/ui/web
npm install          # Install dependencies
npm run build        # Production build → ../dist/
npm run dev          # Dev server with HMR
npm test             # Vitest unit tests
npm run lint         # ESLint check
```

Build output is embedded into the Go binary. After `npm run build`, restart the Go server to pick up changes.
