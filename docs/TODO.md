# Blueprint: Valley Visibility (Effective AGL Scoring)

This document outlines the architecture and implementation plan for the "Valley Visibility" feature.

## 1. Context & Goals
**Valley Visibility** (or **Effective AGL Scoring**) adjusts the visibility range of POIs based on the aircraft's height relative to the lowest surrounding terrain.

*   **The Problem**: Flying close to the ground over a high peak or ridge, the altitude (AGL) is low (e.g., 100ft). Standard logic treats this as "flying low," drastically reducing the visibility radius and hiding distinct POIs in the valley below.
*   **The Goal**: Calculate `EffectiveAGL = AircraftAltitudeMSL - LowestValleyFloorAltitudeMSL`. Calculate the visibility score using both `RealAGL` and `EffectiveAGL`, then use the **Maximum of the two scores**. This handles the non-linear relationship between altitude and visibility (e.g., small objects disappearing at high altitudes) by selecting the most favorable scoring perspective.
*   **Performance Constraint**: Calculating "Valley Floor MSL" requires querying the ETOPO1 database (`GetLowestElevation`). This is an expensive I/O operation (O(10ms)). Doing this for every POI (N=500+) in the scoring loop is unacceptable.
*   **Result**: The fetch must happen **once per scoring cycle** (O(1)), not per POI (O(N)).

## 2. Architecture: Scorer Session Pattern
To decouple the high-level `poi.Manager` from low-level terrain operations while maintaining performance, we use the **Scorer Session** pattern.

*   The `poi.Manager` requests a `Session` at the start of a cycle.
*   The `Scorer` (which holds the `ElevationProvider`) performs the expensive `GetLowestElevation` check **during session creation** (setup once).
*   The returned `Session` object is lightweight and contains the cached `ValleyAltitude` context to score multiple POIs (run many).

## 3. Implementation Plan

### Phase 1: Terrain Capability
**Goal**: Implement the optimized terrain scanning logic.
1.  [x] Define the `ElevationGetter` interface in `pkg/terrain` (or shared interface package).
    ```go
    type ElevationGetter interface {
        GetLowestElevation(lat, lon, radiusKM float64) (int16, error)
    }
    ```
2.  [x] Implement `GetLowestElevation` in `pkg/terrain/elevation.go`.
    *   **Requirement**: Must use the optimized scanning algorithm (buffered reads, longitude wrapping) provided in Section 4 to ensure performance.

### Phase 2: Scorer Architecture
**Goal**: Establish the Session pattern.
1.  [x] Define a `Session` interface in `pkg/scorer`:
    ```go
    type Session interface {
        Calculate(poi *model.POI)
    }
    ```
2.  [x] Update `Scorer` to implement a factory method:
    *   `NewSession(input *ScoringInput) Session`
    *   Inject `ElevationGetter` into the `Scorer` struct.
    *   Inside `NewSession`, call `GetLowestElevation` once and store the result in the cached session struct.

### Phase 3: Visibility Logic
**Goal**: Connect cached data to calculation.
1.  [x] Update `visibility.Calculator.CalculatePOIScore` to accept `valleyAltMSL`.
2.  [x] Implement the logic: Calculate score for `RealAGL`, calculate score for `EffectiveAGL`, return `Max(ScoreA, ScoreB)`.

### Phase 4: Orchestration
**Goal**: Wire it up.
1.  [x] Update `poi.Manager` to use the new pattern:
    ```go
    // Start of cycle
    session := m.scorer.NewSession(ctx, telemetry)
    // Loop
    for _, p := range pois {
        p.Score = session.Score(p)
    }
    ```
2.  [x] Ensure `poi.Manager` has NO direct dependency on `ElevationProvider`.

> [!NOTE]
> All phases completed. Valley Visibility (Effective AGL) feature is fully implemented.

## 4. Required Implementation: Efficient Elevation Scanning
The following `GetLowestElevation` implementation logic is required to meet performance targets. It handles ETOPO1 grid scanning, date-line wrapping, and row buffering.

```go
// pkg/terrain/elevation.go

// GetLowestElevation returns the minimum altitude (meters) within radiusNM.
func (e *ElevationProvider) GetLowestElevation(lat, lon, radiusNM float64) (int16, error) {
    if radiusNM < 0 { return 0, fmt.Errorf("negative radius") }

    // 1. Calculate Grid Bounds (1 NM = 1 arc-minute = 1 row)
    radiusRows := int(math.Ceil(radiusNM))
    
    // Adjust for longitude convergence: radiusCols = radiusRows / cos(lat)
    cosLat := math.Cos(lat * math.Pi / 180.0)
    if math.Abs(cosLat) < 0.01 { cosLat = 0.01 }
    radiusCols := int(math.Ceil(float64(radiusRows) / cosLat))

    centerRow := int(math.Round((90.0 - lat) * 60.0))
    centerCol := int(math.Round((lon + 180.0) * 60.0))

    minElev := int16(math.MaxInt16)
    startRow, endRow := centerRow - radiusRows, centerRow + radiusRows
    startCol, endCol := centerCol - radiusCols, centerCol + radiusCols
    width := endCol - startCol + 1

    // 2. Scan Grid
    for r := startRow; r <= endRow; r++ {
        // Clamp Latitude
        row := r
        if row < 0 { row = 0 } else if row >= etopo1Rows { row = etopo1Rows - 1 }

        // Scan row (handling wrap-around)
        if err := e.scanRowSegment(row, startCol, width, &minElev); err != nil {
            return 0, err
        }
    }
    
    if minElev == math.MaxInt16 { return e.GetElevation(lat, lon) }
    return minElev, nil
}

// Helper: Scan a row segment handling date-line wrapping
func (e *ElevationProvider) scanRowSegment(row, startCol, width int, minElev *int16) error {
    normStart := (startCol%etopo1Cols + etopo1Cols) % etopo1Cols
    if normStart+width <= etopo1Cols {
        return e.scanChunk(row, normStart, width, minElev)
    }
    // Wrap
    firstLen := etopo1Cols - normStart
    if err := e.scanChunk(row, normStart, firstLen, minElev); err != nil { return err }
    return e.scanChunk(row, 0, width-firstLen, minElev)
}

// Helper: Read contiguous chunk (buffered)
func (e *ElevationProvider) scanChunk(row, colStart, count int, minElev *int16) error {
    if count <= 0 { return nil }
    offset := int64(row*etopo1Cols+colStart) * 2
    b := make([]byte, count*2)
    if _, err := e.file.ReadAt(b, offset); err != nil { return err }
    for i := 0; i < count; i++ {
        val := int16(binary.LittleEndian.Uint16(b[i*2 : i*2+2]))
        if val < *minElev { *minElev = val }
    }
    return nil
}
```
