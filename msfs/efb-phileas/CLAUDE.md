# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Primary references

- **MSFS Avionics framework docs**: https://microsoft.github.io/msfs-avionics-mirror/2024/docs/intro
  This is the canonical reference for `FSComponent`, `DisplayComponent`, `Subject`, `EventBus`, `MapSystemBuilder`, and all SDK primitives. Check here first before assuming React or standard JS patterns.
- **Installed SDK types**: `PackageSources/phileas/node_modules/@microsoft/msfs-sdk/` — full TypeScript declarations, browsable directly.
- **Installed EFB API types**: `PackageSources/efb_api/dist/*.d.ts` — type declarations for `App`, `AppView`, `GamepadUiView`, `TTButton`, `List`, `ViewService`, etc. This is the only EFB API documentation available.

## Build

From the **project root**:
```bash
make build-efb
```

From `PackageSources/phileas/` directly:
```bash
npm run build    # single build
npm run watch    # rebuild on file changes
npm run rebuild  # clean then build
```

No tests exist for this package.

## Architecture

### Technology stack
- TypeScript + JSX, but the JSX factory is `FSComponent.buildComponent` (not React). Fragments use `FSComponent.Fragment`. Do not import React.
- Rollup bundles to a single IIFE (`dist/phileas.js`). `@microsoft/msfs-sdk` is **external** — provided at runtime as the global `msfssdk`, never bundled.
- SCSS compiled by PostCSS and **automatically prefixed** with `.efb-view.phileas` for CSS scoping. Never add the prefix manually in source.
- Build-time constants injected by `@rollup/plugin-replace`: `BASE_URL` (`coui://html_ui/efb_ui/efb_apps/phileas`), `VERSION`, `BUILD_TIMESTAMP`.

### App lifecycle
`Phileas.tsx` registers `PhileasApp` with `Efb.use()`. Boot mode is `HOT` (mounted immediately on EFB start), suspend mode is `SLEEP` (paused but not destroyed when the user navigates away).

`PhileasAppView` owns the data loop: a 1-second `setTimeout` chain polling the Go backend at `http://127.0.0.1:1920/api/...`. All fetched data is held in `Subject<any>` fields and passed as props down the component tree.

### Critical EFB constraint: no SimVar publisher on the bus
The EFB `EventBus` has **no `GNSSPublisher`** and no SimVar wiring. This means:
- `gps-position` events never fire.
- `withOwnAirplanePropBindings([...], n)` SimVar bindings do nothing.
- Aircraft position (`position`) and heading (`trackTrue`) must be driven **manually** from the HTTP telemetry Subject into `MapOwnAirplanePropsModule` on every telemetry update.

### Data flow
```
PhileasAppView   — data loop, all Subjects
  └─ PhileasPage — tab UI, location card, POI/settlement lists
       └─ MapComponent — MapSystemBuilder, manual position/heading drive
            └─ PhileasPoiLayer — custom MapLayer, two-pass DOM rendering
```

### Map (MapComponent.tsx)
Built with `MapSystemBuilder`:
- `withBing("bing")` — Bing satellite tiles. **Requires** `MapTerrainColorsModule` to be initialised with a 61-entry RGB-packed color array after `build()`; otherwise Bing renders black.
- `withOwnAirplanePropBindings([], 1)` — empty list; the module is required by `withOwnAirplaneIcon` but values are set manually.
- `withOwnAirplaneIcon` — aircraft icon rotates via `ownAirplaneModule.trackTrue.set(heading, UnitType.DEGREE)`.
- Map stays **north-up** (no `withRotation()`).
- Framing: bounding box of aircraft + active (non-cooldown) POIs, clamped to 1–50 NM range, recomputed each second.

### POI layer (PhileasPoiLayer)
Custom `MapLayer` rendering markers as raw DOM elements. Uses **two-pass rendering**: all disc `div`s in one container, then all icon `img`s in a second container at higher z-index — prevents adjacent discs from occluding icons.

Coordinate projection: `project()` may use the map target as its origin, so positions are normalised as pixel offsets from `project(getTarget())` then re-centred on canvas midpoint.

POI disc colour scheme:
- `#2ecc71` green — currently playing
- `#2a9d8f` teal — preparing
- `#356285` blue — cooldown (`last_played` within 8 h, matching `repeat_ttl` in `phileas.yaml`)
- `#E9C46A` yellow → `#e63946` red — score 0 → 20

### Assets
SVG icons in `src/Assets/icons/` are copied to `dist/assets/icons/` at build time. Reference at runtime via `${BASE_URL}/assets/icons/<name>.svg`.
