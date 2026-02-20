# Artistic Map Reference (MapLibre)

maplibre-gl v5 (raw, no React wrapper) with Turf.js for geometry.

**Entry point**: `src/components/ArtisticMap.tsx`

## Tile Sources & Style

Configured in `src/styles/artisticMapConfig.ts` (MapLibre `StyleSpecification`):

| Source | Type | URL | Notes |
|---|---|---|---|
| Stamen Watercolor HD | raster | `watercolormaps.collection.cooperhewitt.org/.../watercolor/{z}/{x}/{y}.jpg` | 128px tiles (HD at half-size) |
| Stadia Terrarium DEM | raster-dem | `tiles.stadiamaps.com/data/terrarium/{z}/{x}/{y}.png` | Hillshade, terrarium encoding |
| OpenFreeMap | vector | `tiles.openfreemap.org/planet` | Aeroway/runway data |

**Layers** (bottom to top):
1. `background` — parchment `#f4ecd8`
2. `watercolor` — HD tiles, saturation -0.2, contrast +0.1
3. `hillshading` — DEM-based, exaggeration interpolated (0.0 at z4 → 0.45 at z6), maxzoom 10
4. `runways-fill` — vector aeroways z8+, green (grass/dirt) vs grey (paved)
5. `runways-line` — vector runway lines z8+

## MapFrame (Atomic State Type)

Defined in `src/types/artisticMap.ts`. The heartbeat produces a single `MapFrame` per tick, consumed by `useLayoutEffect` to sync the camera and all SVG overlays in the same paint cycle.

```typescript
interface MapFrame {
  labels: LabelCandidate[];          // Placed labels from PlacementEngine
  maskPath: string;                  // SVG path for visibility cone mask
  center: [number, number];         // Map center [lat, lon]
  zoom: number;                      // Discrete integer zoom
  offset: [number, number];         // Dead-zone heading offset [dx, dy]
  heading: number;                   // Aircraft heading (degrees)
  bearingLine: Feature<any> | null; // GeoJSON bearing line (Turf)
  aircraftX: number;                // Aircraft screen X
  aircraftY: number;                // Aircraft screen Y
  agl: number;                       // Altitude above ground level
}
```

## Heartbeat Loop

A single `setInterval(2000)` drives all updates. Reads from refs only (never React state).

**Tick sequence:**
1. **Dead-zone check** — compute aircraft screen position. If inside dead zone (heading-forward offset circle), skip pan. If outside, recenter with easing.
2. **Zoom snap** — round to nearest integer. If zoom changed, `resetCache()` on PlacementEngine.
3. **Label sync** — POST `/api/map/labels/sync` with bounding box, aircraft position, heading, zoom. Returns settlement `LabelDTO[]`.
4. **PlacementEngine** — register settlements + POIs, call `compute()`. Returns placed `LabelCandidate[]`.
5. **Aircraft projection** — `map.project()` to get screen coordinates.
6. **Bearing line** — Turf `destination()` + `lineString()` for heading indicator.
7. **Visibility mask** — fetch `/api/map/visibility` mask path for paper overlay.
8. **`setFrame()`** — atomic state update triggers `useLayoutEffect` → camera sync + SVG overlay repaint.

`useLayoutEffect` syncs MapLibre camera to frame in the same paint cycle as SVG overlays, preventing flicker.

## Dead-Zone Panning

Aircraft triggers a camera pan **only when it exits a dead-zone circle**. The dead zone is offset forward along the aircraft's heading (percentage of viewport), centered ahead of the aircraft. This prevents camera jitter during stable straight-line flight. When the aircraft exits the zone, the camera smoothly recenters with easing.

## Zoom Snapping

The artistic map uses **discrete integer zoom** (nearest 0.5 step). Float zoom from MapLibre is rounded before being used for any rendering decisions.

**Key invariant**: POI markers are "stamped" at their discovery zoom level (`placedZoom`). Their visual scale at any subsequent zoom is `2^(currentZoom - placedZoom)`. This creates a permanence effect where markers grow or shrink naturally as you zoom.

## POI Markers

**File**: `src/components/ArtisticMapPOIMarker.tsx`

### Icon Rendering
- `InlineSVG` component fetches and inlines category SVG icons
- CSS custom properties `--stamped-stroke` and `--stamped-width` control the stamped ink aesthetic
- Score-based color lerp: silver (`#C0C0C0`) at score 0 → gold (`#D4AF37`) at score 50

### Visual Elements per Marker

| Element | Description |
|---|---|
| **Icon** | InlineSVG with stamped stroke, score-colored |
| **Halo** | Paper-colored cutout (`#f4ecd8`) for low-score POIs, organic smudge for others |
| **Calligraphic tether** | Bezier SVG line from icon to label, dark grey (`#292929`), opacity 0.6, anchor dot |
| **Wax seal** | `WaxSeal.tsx` — red organic splat SVG for played/special POIs |
| **POI beacon** | `POIBeacon.tsx` — hot-air balloon SVG (envelope + strings + gondola), colored by `beacon_color` |
| **MSFS star badge** | Gold `★` for MSFS POIs |
| **Champion label** | Score ≥10 POIs get a text label placed via PlacementEngine secondary placement |

### Color Palette (`artisticMapStyles.ts`)

```
Gold:        #D4AF37  — High score POIs (≥20)
Silver:      #C0C0C0  — Low score POIs (≤0)
Copper:      #B55A30  — Default/medium score
Historical:  #484848  — Cast iron grey (visited)
Selected:    #e63946  — Comic red (active balloon)
Next:        #E9C46A  — Saffron gold (preparing)
Compass:     #0D3B3F  — Dark teal
```

### Tether Styling
- Stroke: `#292929`, width 1.2px, opacity 0.6
- Anchor dot: radius 3px, opacity 0.8

## Paper Overlay

**File**: `src/components/ArtisticMapPaperOverlay.tsx`

SVG-masked parchment texture with a visibility cone cutout. Opacity varies by sim state:
- `paperOpacityFog` — base fog opacity (configurable)
- `paperOpacityClear` — clear area opacity (configurable)
- Mask path comes from `/api/map/visibility` response, rendered as SVG `<path>` inside a `<mask>`

## Settlement Labels

**File**: `src/components/ArtisticMapSettlement.tsx`

Plain `<div>` elements positioned absolutely. Labels fetched via `POST /api/map/labels/sync` on each heartbeat tick.

- Tier-based font sizing (city > town > village)
- Text color: `#0a0805` (active) or `#3a2a1d` (historical)
- Text shadow: parchment-colored halo for readability
- Fade-in animation over 2s on first appearance
- Placed by PlacementEngine alongside POI markers (priority: cities 100, towns 95, villages 90)

## PlacementEngine

**File**: `src/metrics/PlacementEngine.ts`

Greedy label collision solver using RBush (R-tree spatial index).

### Two-Phase Algorithm

**Phase 1 — Locked items** (already placed in previous tick):
- Force-inserted at cached position (no collision check).
- Collision box scaled by `2^(currentZoom - placedZoom)`.
- Preserves the "labels are static between zoom snaps" aesthetic.

**Phase 2 — New items** (never placed, in priority order):
1. **True position**: try centered at projected lat/lon. Settlements dropped if blocked here.
2. **8-point anchor search**: top-right → top → right → bottom → left → top-left → bottom-right → bottom-left.
3. **Radial step-search**: expanding rings (radius += 2px, angle += 10°) until placed.

### Priority Queue

| Category | Priority | Notes |
|---|---|---|
| Landmarks | 300 | Fixed viewport elements |
| Cities | 100 | Settlement tier |
| Towns | 95 | Settlement tier |
| Villages | 90 | Settlement tier |
| Active Small POIs | 85–87 | Size S + freshness bonus + score |
| Active Medium POIs | 75–77 | Size M |
| Active Large POIs | 65–67 | Size L |
| Active XL POIs | 55–57 | Size XL |
| Historical variants | each -5 | Same buckets, lower priority |

### Marker Labels (Secondary Placement)

POIs with score ≥10 get a text label. After the marker icon is placed, `tryPlaceMarkerLabel()` runs the same 8-anchor search for a small text box adjacent to the icon. Suffixed `_sec` in R-tree to avoid self-collision.

### Caching

- `placedCache: Map<id, PlacementState>` — persists anchor + zoom across heartbeats
- `placedIds: Set<id>` — tracks which items are in R-tree
- `resetCache()` — called on zoom snap (integer change)

## Text Measurement

**File**: `src/metrics/text.ts`

Canvas-based text measurement with two-level caching:
- `getFontFromClass(className)` — extracts computed font string from CSS class using a hidden DOM probe. Cached by class name.
- `measureText(text, font, letterSpacing)` — canvas `measureText()` with manual letter-spacing correction. Cached by `text:font:spacing` key.

## Replay Mode (InkTrail)

**File**: `src/components/InkTrail.tsx`

Canvas-drawn crimson dashed path with quill-nib calligraphic effect.

### Quill Nib Rendering
- `NIB_ANGLE = π/4` — 45° nib orientation
- `NIB_RATIO = 0.75` — contrast between thick/thin strokes
- `BASE_WIDTH = 4.5px`
- Width varies by stroke direction: perpendicular to nib → thick, parallel → thin

### Dash Construction
- `DASH_LEN = 15px` with ±15% human variation (seeded random)
- `WOBBLE_AMP = 2.5px` — perpendicular offset for hand-drawn feel
- Gap size: 10–16px (seeded random)
- Bezier control point offset creates organic curvature
- `seededRandom(seed)` — deterministic pseudo-random (stable across frames)

### Airport X Marks
- Hand-drawn "X marks the spot" at departure/destination
- Two strokes: `\` (perpendicular to nib → thick) + `/` (parallel → thin)
- Seeded wobble for organic consistency

### Canvas Rendering (Two Passes)
1. **Halo pass**: widthScale 3.0, alpha 0.25, blur 3px (ink bleed effect)
2. **Core pass**: widthScale 1.0, alpha 0.9, no blur (solid strokes)

Tip fade: last 3 dashes fade progressively (drawing-in-progress effect).

POIs appear progressively as replay progresses, matching their original discovery timestamps.

## Debug Tools

### ArtisticMapDebugBoxes (`src/components/ArtisticMapDebugBoxes.tsx`)

R-tree bounding box visualization (enabled via settings `showDebugBoxes`):
- Red boxes (`rgba(255,60,60,0.7)`) — marker collision boxes
- Blue boxes (`rgba(60,120,255,0.7)`) — label collision boxes
- Full-viewport SVG overlay, z-index 25, pointer-events none

## Key Utilities

### `artisticMapUtils.ts`
- `lerpColor(c1, c2, t)` — hex color interpolation (0≤t≤1)
- `adjustFont(fontObj, offset)` — adjust font size by pixel offset (e.g., for zoom-relative text)
- `getMaskColor(opacity)` — convert 0–1 opacity to greyscale RGB string for canvas masks

### `replay.ts`
- `calculateHeading(from, to)` — great-circle bearing (0–360°)
- `interpolatePositionFromEvents(events, progress)` — timestamp-based interpolation (preferred)
- `getSignificantTripEvents(events)` — truncate to take-off → landing segment
- `isTransitionEvent(type)` / `isAirportNearTerminal(poi, departure, destination)` — event classification
