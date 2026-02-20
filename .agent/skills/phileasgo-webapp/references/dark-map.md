# Dark Map Reference (Leaflet)

react-leaflet v5 / Leaflet 1.9.4 with CARTO Dark Matter tiles.

**Entry point**: `src/components/Map.tsx`

## Component Tree

```
Map.tsx
├── MapStateController     # Toggles interaction based on SimState
├── MapEvents              # Captures zoomstart + click
├── AircraftPaneSetup      # Creates dedicated z-2000 pane for aircraft
├── TileLayer              # CARTO Dark Matter basemap
├── MapBranding            # "Phileas Tour Guide" (fallback/paused only)
├── CoverageLayer          # Blue circles from /api/map/coverage (paused only)
├── TripReplayOverlay      # Animated path + markers (replay mode only)
├── CacheLayer             # White circles from /api/map/cache (optional, 5s poll)
├── VisibilityLayer        # Scored grid from /api/map/visibility (optional, 15s poll)
├── RangeRings             # Blue dashed circles with smart label
├── AircraftMarker         # Rotated gold SVG plane
└── SmartMarkerLayer       # POI markers with d3-force collision avoidance
```

## Camera & Zoom

| State | Zoom Range | Interaction | Notes |
|---|---|---|---|
| Connected (active) | [8, 13] | Locked (no pan/zoom) | Auto-zoom follows POIs |
| Paused (inactive) | [2, 18] | Full interaction | World view at zoom 2 |
| Replay | [2, 18] | Full interaction | flyToBounds on route |

**Auto-zoom algorithm:**
1. Identify "non-blue" POIs (not yet played).
2. Compute symmetric bounding box around aircraft ± POI spread.
3. Call `map.getBoundsZoom()` with 60px padding, clamp to [8, 13].
4. Apply heading-forward offset: camera leads aircraft by 25% of smallest map dimension.
5. Mark move as automated (100ms grace period ignores subsequent zoom events).

Disables on manual zoom (detected via `zoomstart`). Throttled position updates at 2s interval.

## SmartMarkerLayer (d3-force POI Markers)

**File**: `src/components/SmartMarkerLayer.tsx`

Uses `createPortal()` into Leaflet's marker pane for GPU-accelerated positioning.

### D3-Force Simulation

- **Collide force**: radius = `MARKER_RADIUS (14px) * scale + 5px padding`, strength 1.0 (hard)
- **X/Y anchor forces**: strength 0.1 (soft — prioritizes separation over accuracy)
- **Iterations**: 300 per frame (synchronous, no animation)
- **Symmetry breaking**: tiny deterministic offset seeded from `wikidata_id`

### Marker Priority & Styling

| State | Color | Scale | Z-Index | Opacity |
|---|---|---|---|---|
| Narrating/Selected | Green | 1.5x | 80000+ | 1.0 |
| Preparing | Dark green | 1.3x | 60000+ | 1.0 |
| MSFS POI (unplayed) | Score-based | 1.0x | 40000+ | visibility-aware |
| Unplayed | Score-based | 1.0x | 20000+ | visibility-aware |
| Played | Blue | 0.6x | 0+ | 0.8 |

**Score-based color**: HSL yellow→green hue mapped from POI score (1–50).

**Badges** (rendered as emoji overlays):
- Top-right: `★` MSFS (gold)
- Top-left: `💎` fresh
- Bottom-right: `🌐` deep_dive / `🧩` stub
- Bottom-left: `🕒` deferred / `⏩` urgent / `⏪` patient / `🚫` LOS blocked

Hidden during zoom animation to prevent artifacts.

## AircraftMarker

**File**: `src/components/AircraftMarker.tsx`

- Gold SVG plane, 48×48px, with black stroke
- Rotation via `transform: rotate(${heading}deg)` with 0.1s transition
- Renders to custom `aircraftPane` (z-index 2000)
- Non-interactive (`interactive: false`)

## RangeRings

Embedded in `Map.tsx` (lines 71–179).

- Ring distances: [5, 10, 20, 50, 100] in user's unit (km or nm)
- Blue dashed circles (`#4a9eff`, weight 1, opacity 0.4)
- **Smart label**: only ONE label visible — largest ring still within map bounds, placed 20px inward from edge

## Optional Layers

### VisibilityLayer (`src/components/VisibilityLayer.tsx`)
- Canvas-based grid overlay from `/api/map/visibility`
- Three sub-layers by distance category: M (gold, z=502), L (yellow, z=501), XL (pale, z=500)
- Alpha: `sqrt(score)` boosted, range [15%, 40%]
- Refreshes every 15s + on `moveend`

### CacheLayer (`src/components/CacheLayer.tsx`)
- White circles (fill opacity 0.075) from `/api/map/cache`
- 5s polling with bounds-based query
- Default radius 9800m per tile

### CoverageLayer (`src/components/CoverageLayer.tsx`)
- Blue-400 circles (`#60a5fa`, fill opacity 0.2) from `/api/map/coverage`
- Single fetch on mount, no polling

## Trip Replay

**File**: `src/components/TripReplayOverlay.tsx`

Triggered when `SimState === 'disconnected'` AND `tripEvents.length > 1`, or when narrator is `debriefing`.

### Marker Lifecycle Animation

| Phase | Duration | Scale | Color |
|---|---|---|---|
| Grow | 0–4s | 0→100% | Yellow→Orange |
| Live | 4–14s | 100% | Orange→Green→Blue |
| Shrink | 14–16s | 100%→shrinkTarget | Blue |

**Dynamic shrink target** (logarithmic): 1 POI → 100%, 64 POIs → 50%.

### D3-Force (Replay-Specific)
- Collide strength: 0.8 (softer than live)
- Anchor strength: 0.08 (floatier feel)
- Iterations: 100 per frame (reduced for performance)
- Track nodes (departure/destination airports): pinned, repulsion only

### Route Rendering
- Double polyline: outer `#FCF5E5` (parchment, weight 10), inner crimson (weight 6, dashed `10,5`)
- Custom panes: `trailPane` (z=610), `terminalPane` (z=615), `replayPlanePane` (z=620)

### Credit Roll
- POI names added at spawn time
- `CreditRoll.tsx` rendered as portal to `<body>` (z=9999)
- Adaptive scroll speed: `max(3000, 9000 - totalCount * 75)` ms
- Special colors: magenta for screenshots, lime for essays
