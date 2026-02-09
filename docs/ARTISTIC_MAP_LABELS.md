# Implementation Plan: Artistic Victorian Cartographic Stamping Engine (PoC)

## 1. Architectural Overview
This system implements a new static label placement engine for web-based cartography, utilizing a greedy constraint-satisfaction algorithm to ensure legible, non-overlapping text and iconography.

### System Components
*   **Prioritization Engine (Go Backend)**: A server-side service that filters, weights, and ranks Geospatial Points of Interest (POIs) based on telemetry and intrinsic importance scores.
*   **Placement Logic (React/JS Frontend)**: A client-side engine that calculates bounding box dimensions and executes a coordinate-projection-based placement algorithm.
*   **Spatial Index (R-Tree)**: A high-performance spatial data structure used to manage and query occupied screen-space coordinates.

## 2. Backend: Prioritization & Culling Logic (Go)
The backend acts as the data authority, ensuring the frontend only receives a relevant subset of features to process. **THIS IS ALREADY IMPLEMENTED AND WORKING. ONLY THE NEW SETTLEMENT ENDPOINT IS REQUIRED.**

### New Settlement Endpoint Requirements
*   **Highest-Tier-Only Selection**: The endpoint must query the current viewport bounds and identify the single highest settlement tier present (e.g., if a City is present, ignore all Towns and Villages; if no Cities, then Towns, and so on).
*   **Standardized Response**: The endpoint returns a JSON array of existing POI structs.
*   **POI Context**: Continue providing the standard set of tracked/recent POIs for symbol placement (icons) via existing endpoints.

### Persistence & TTL
*   **Trail Persistence**: POIs with a LastPlayed attribute within the TTL (narrated recently) are flagged as "Historical."
*   **State Logic**: Active POIs and Historical POIs are treated as the same data type but sorted differently in the placement queue.

## 3. Frontend: Placement Engine (React + MapLibre)
The frontend translates geographic coordinates into a collision-free visual layout, using a sorted intake queue to prioritize spatial accuracy for smaller features.

### Execution Pipeline
1.  **Projection and Viewport Synchronization**: Captured map bounds are used to request settlement data and standard POIs. All coordinates are projected to Pixel(x, y) relative to the current viewport.
2.  **The Sorted Intake Queue (Priority Order)**:
    1.  Highest-Tier Settlements (Label + Symbol Placeholder).
    2.  Active POIs (Size S)
    3.  Historical POIs (Size S)
    4.  Active POIs (Size M)
    5.  Historical POIs (Size M)
    6.  Active POIs (Size L)
    7.  Historical POIs (Size L)
    8.  Active POIs (Size XL)
    9.  Historical POIs (Size XL)
3.  **Collision Detection & Placement**: For each item, the engine checks the R-Tree for intersections (including a 4px buffer). If blocked, it executes the Multi-Stage Conflict Resolution.

### Candidate Search Algorithm (Placement Strategy)
*   **Stage 1: The 8-Position Anchor Model (Discrete Moves)**: Attempts to "dock" the bounding box at 8 specific anchor points (Top-Right preferred).
*   **Stage 2: Radial Step-Search (Continuous Displacement)**: If anchors are occluded, executes an exhaustive radial search in 5px steps until space is found. Symbols are never dropped.

## 4. State Persistence & Zoom Handling
*   **In-situ Updates**: Labels and symbols remain locked to geographic anchors. Exhaustive search ensures every feature is assigned a unique coordinate.
*   **Zoom-Level Discretization**: Re-initialization (flush and re-run) is triggered only when `Math.floor(zoom)` changes.

## 5. Visual Specifications
*   **Typography**: *Pinyon Script* (Copperplate style) for settlements.
*   **Atmospheric Dispersion (Ink Bleed)**: `text-shadow` or `text-halo` with Color: `rgba(245, 245, 220, 0.6)` and Blur: 1.5px to 2px.
*   **Iconography (PoC)**: Utilize the SVG versions of Maki icons (Mapbox) as the baseline asset set. These must be embedded as inline SVGs or loaded via an icon component to allow for dynamic CSS styling of fill and stroke.

### The Ink Palette (State & Score Mapping)
*   **Selected**: "Rich Black" or "Deep Indigo."
*   **Next in Queue**: "Burned Sienna" or "Deep Ochre."
*   **Metallic Range (Score)**: High (Gold: `#D4AF37`), Mid (Silver: `#C0C0C0`), Low (Bronze/Antique Copper).
*   **Visibility Mapping**: Input range (0.0-1.0) maps to opacity range (0.5-1.0) for a subtle effect.
*   **Historical POIs**: Rendered at 40% desaturated "Faded Ink" opacity.

## 6. Technical Challenges & Mitigation
*   **Font Loading**: Use `FontFaceSet` API to defer placement until ready.
*   **Asynchronous Race Conditions**: Ensure "Zoom Step" clear events cancel pending calculations.
*   **Viewport Clipping**: Discard bounding boxes intersecting viewport boundaries.

## 7. Feature-Symbol Tethering
Rendered only if displacement > 30px.
*   **Graphic Convention**: The Hairline Lead Line.
*   **Geometry**: 0.5px solid stroke using a subtle SVG "S-curve."
*   **Termination**: Ends at the true coordinate with a 1.5px ink "dot."

## 8. Development Roadmap

### Phase 1: Go Service Integration
*   Implement specialized settlement endpoint and ensure POI struct contains all necessary fields.

### Phase 2: Infrastructure & Scaffolding
*   Setup `rbush` mutable ref.
*   Implement `useMapProjection` and stub `useStampingEngine`.
*   Implement font-loading listener.

### Phase 3: Client-Side Spatial Management
*   Integrate `rbush` and develop `useLabelPlacement` hook (Sorting, Anchors, Radial Search, Compound Reservation).
*   Implement dynamic tethering arcs.

### Phase 4: Rendering & Aesthetics (PoC)
*   Configure MapLibre watercolor background and HTML Markers.
*   **Ink-Bleed PoC**: Implement a global SVG `feTurbulence` filter proposal. Apply this filter to the Maki SVGs to "distress" their modern geometry and simulate hand-stamped ink.

## 9. Future Refinements: Historic Iconography
Transition from Maki icons to custom single-color SVGs representing 19th-century conventions:
*   **Mountains**: Hachured Peaks (Arced reverse "V").
*   **Water Features**: Shoreline Ripples (undulating horizontal lines).
*   **Forest**: Individual Deciduous sketch clusters.
