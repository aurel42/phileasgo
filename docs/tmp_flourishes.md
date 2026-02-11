# Artistic Map Flourishes Plan (Refined)

This plan outlines the implementation of decorative and functional enhancements for the artistic map, focusing on the Jules Verne compass rose, red wax seals, and enhanced marker aesthetics.

## Proposed Changes

### 1. Compass Rose (High Priority)
- **[NEW] [CompassRose.tsx](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/components/CompassRose.tsx)**: A decorative Jules Verne-style SVG compass rose.
- **[MODIFY] [PlacementEngine.ts](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/metrics/PlacementEngine.ts)**:
    - Add a new "Compass" priority tier (Priority 200) in `getPriority`.
- **[MODIFY] [ArtisticMap.tsx](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/components/ArtisticMap.tsx)**:
    - Track active compass rose state (ID, lat, lon).
    - Lifecycle:
        - If no compass or it left the viewport: find a new geographic spot in one of the 4 viewport corners (preferring the aircraft's heading direction).
        - Offset the spot by 4px padding + half-size.
        - Register it as a high-priority `poi` candidate (fixed size ~2x POI marker).
    - Rendering: Render with 50% opacity.

### 2. Red Wax Seal
- **[NEW] [WaxSeal.tsx](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/components/WaxSeal.tsx)**: Organic red wax seal SVG.
- **[MODIFY] [ArtisticMap.tsx](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/components/ArtisticMap.tsx)**:
    - Render `WaxSeal` behind POI icons based on state (e.g., `is_champion` or `is_narrating`).
    - Scaled slightly larger than the icon.
    - Excluded from `PlacementEngine` registration (it's just a visual layer).

### 3. Enhanced Marker Styles
- **[MODIFY] [ArtisticMap.tsx](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/components/ArtisticMap.tsx)**:
    - **Halos**: Implement `drop-shadow` filters.
        - *Organic*: Single, soft-edged brownish shadow.
        - *Neon*: Multiple, layered bright shadows (e.g. cyan/pink).
    - **Silhouette**: Apply `brightness(0)` filter based on POI state.
    - **Dynamic Outlines**: Pass custom stroke width and color variables to the icon's CSS.

### 4. Debug Symbols
- **[MODIFY] [ArtisticMap.tsx](file:///c:/Users/aurel/Projects/phileasgo/internal/ui/web/src/components/ArtisticMap.tsx)**:
    - Add a `DEBUG_FLOURISHES` constant.
    - If enabled, inject a grid of test POIs at the start location (e.g., 3x3) demonstrating all combinations of the above effects.

## Refinements & Fixes

### 1. Compass Rose Stability
- **Issue**: Constant redrawing/flashing.
- **Fix**: 
    - Reuse the compass rose ID unless it's a truly "new" placement.
    - Stabilize the `spawnCompassRose` logic to avoid unnecessary updates if the aircraft heading change is small and it's still in a valid corner.
    - Ensure `useMemo` for labels doesn't trigger a full re-calc solely on minor heading changes.

### 2. Wax Seal Layering & Aesthetic
- **Layering**: Adjust `zIndex` in `ArtisticMap.tsx` so `WaxSeal` is explicitly below the icon (`zIndex: 14` vs `icon: 15`).
- **Aesthetic**: 
    - Modify the `path` in `WaxSeal.tsx` to be more irregular (less circular).
    - Enhance the `wax-texture` filter for more depth.

### 3. Style Calibrations
- **Silhouette**: Change `brightness(0)` (black) to `brightness(0.2)` (dark charcoal) for better readability.
- **Stroke Weights**: 
    - "Thick" reduced from 3.0px to 1.8px.
    - "Red" remains at 2.0px (user liked this).
- **Dark Halo**: User liked the contrast; ensure this is properly applied.

### 4. Debug Sampler Improvement
- Add explicit labels (using `secondaryLabel`) to all debug icons so each effect can be clearly identified.

## Verification Plan
...

### Manual Verification
- **Compass Rose**: Verify it stays fixed to the ground as you fly, and "respawns" in a new corner when it drifts off-screen.
- **Wax Seals**: Verify they appear behind the correct POIs and overlap naturally without affecting placement.
- **Styling**: Use the debug symbol grid to verify Neon/Organic halos, silhouetting, and outline weights.
