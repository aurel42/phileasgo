# Distributed Settlement Labeling

## 1. Objective & Design Philosophy
The goal is to maintain a sparse, "artistic" map aesthetic (4–6 labels total) that feels like a hand-printed travel log. We want to avoid the "label salad" typically found in urban regions (like the Ruhrgebiet) by prioritizing landmarks that offer the most "Information Density" (high population, short name length).

**Core Principles:**
*   **The Discovery Theme:** The map is revealed as the balloon flies. We should prioritize settlements *ahead* of the aircraft.
*   **Visual Balance:** We use a "Minimum Separation Radius" (MSR) to ensure labels don't bunch up in the center, even in dense megalopolises.
*   **Ink Economy:** Long names (e.g., *Mönchengladbach*) are "expensive." Short names (e.g., *Ulm*) are "efficient." We prefer efficient labels.
*   **Two-Stage Pipeline:** We pre-populate known global heavyweights from `cities1000` and backfill with local POI discoveries only in otherwise empty regions.

## 2. Technical Architecture
*   **Backend (Go):** The source of truth for "Global Anchors." It manages the `cities1000` database and performs high-cost spatial/heuristic calculations.
*   **Frontend (Vite/MapLibre):** The rendering engine. It manages the R-Tree (for physical icon collisions), tracks the aircraft's discovery of local POIs, and handles the "printed" visual transitions.

## 3. Development Phase 1: Backend Infrastructure (Go)

### Task B1: Scoring & Heuristic Logic
Implement the math that defines which cities "deserve" a spot on the map.
*   **Importance Score:** `pop / len^2`. Squaring the length creates a "quadratic penalty" for long names. This ensures a small town with a short name is often more valuable for map-glanceability than a large city with an excessively long name.
*   **Directional Weight ($W_d$):** Use the dot product of the aircraft's heading and the settlement's relative position.
    *   *Calculation:* `clamp(normalizedAircraftHeading · normalizedTargetVector, 0.1, 1.0)`.
    *   *Purpose:* This ensures we don't "discover" a city that is already behind us.
*   **Final Selection Score:** `importanceScore * W_d`.

### Task B2: The Delta API (`POST /map/labels/sync`)
The frontend calls this during map jumps or zoom changes.
*   **Input:** Viewport bounding box, zoom level, and `existingLabelIds`.
*   **Logic:**
    1.  Query `cities1000` for all candidates in the viewport.
    2.  Filter out labels the frontend already has.
    3.  **MSR Selection:** Using a radius of ~30% of the viewport width, pick the best 4–6 candidates that don't violate each other's exclusion zones.
*   **Output:** A JSON array of new labels to be rendered.

## 4. Development Phase 2: Frontend Core (MapLibre)

### Task F1: State & Panning Logic
Since the map moves in small steps (Translation) or Zoom jumps (Rescale):
*   **Label Persistence:** Labels must be anchored to geographic coordinates. They move with the map—do not re-calculate them.
*   **Culling:** On every step, check if an active label has entirely moved off-screen. If so, remove it from the registry and the R-Tree.
*   **Sync Trigger:** After a jump, call the Backend API to fetch any "Global Anchors" that have entered the new area.

### Task F2: R-Tree & Icon Collision (ALREADY IMPLEMENTED)
*   **The Priority Rule:** Labels are "Kings." They are placed in the R-Tree first.

## 5. Development Phase 3: Local POI Backfill

### Task B3: The "Premonition" Check (`POST /map/labels/check-shadow`)
This prevents us from labeling a tiny village right before we reach a major city.
*   **Input:** A candidate POI coordinate.
*   **Logic:** Check the `cities1000` database within the MSR of that coordinate.
*   **Output:** `true` if a significantly "better" city exists in the database nearby (even if it's not on screen yet).

### Task F3: Backfill Placement
When the balloon discovers a Local POI (Settlement Category):
1.  **MSR Check:** Is there an existing label (Global or Local) within 30% of the viewport width?
2.  **Shadow Check:** Call `/check-shadow`. If the backend says a major city is coming up, stay silent.
3.  **Commit:** If the area is empty and no "Global Anchor" is imminent, render the POI name as a label.

## 6. Development Phase 4: Visual Polish

### Task F4: The "Ink Drying" Transition
*   **Fade-In:** New labels should use a slow CSS opacity transition (3000ms).
*   **Placement:** Ensure labels fade in *behind* the current position of the discovery balloon to reinforce the "uncovering" effect.

### Task B4: Performance (Spatial Indexing)
Ensure the `cities1000` data is indexed (e.g., using an R-Tree in Go or a spatial extension) so that queries remain sub-50ms. We want the map updates to feel instantaneous.

## 7. Summary of Responsibilities

| Component | Responsibility | Technical Context |
| :--- | :--- | :--- |
| **Backend (Go)** | Predictive Selection | Manages `cities1000` and heavy math |
| **Frontend (Vite)** | State & Visuals | Manages R-Tree, MapLibre, and Fades |
| **MSR (Spacing)** | Territorial Balance | 30% viewport radius point-to-point check |
| **Shadow Check** | "Premonition" | Prevents labeling suburbs before cities |
| **Movement** | Persistence | Labels stay put during 15% map steps |


## 8. REVISED Phase B4: Stateful Backend & Shadow Integration
This phase moves the spatial authority and session state to the backend. The `labels.Manager` will now track active settlements, perform proactive discovery (Shadows), and respect the user-defined density limit.

### Proposed Changes

#### Backend: Labels Manager
*   **[MODIFY]** `manager.go`
    *   **State Fields:**
        *   `mu sync.Mutex`
        *   `activeSettlements map[string]*LabelCandidate`
        *   `currentZoom float64`
    *   **Updated `SelectLabels` logic:**
        1.  **Zoom Check:** If `floor(zoom) != floor(m.currentZoom)`, clear `activeSettlements`.
        2.  **Pruning:** Remove settlements from the map if they are significantly outside the viewport (plus buffer).
        3.  **Discovery:** Calculate an expanded BBox (current + 30% along aircraft heading).
        4.  **Greedy Selection:**
            *   Sort all candidates from Global + Local sources.
            *   Skip if already in `activeSettlements`.
            *   Skip if MSR collision with any in `activeSettlements`.
            *   If valid:
                *   If inside viewport -> add to `activeSettlements` (normal).
                *   If outside viewport -> add to `activeSettlements` with `IsShadow: true`.
        5.  **Limit:** Stop adding "normal" settlements once the limit `N` is reached.
        6.  **Return:** Only the "normal" (non-shadow) settlements.

#### Backend: API Handler
*   **[MODIFY]** `map_labels.go`
    *   Update `SyncRequest` to include `Zoom float64`.
*   **[DELETE]** `HandleCheckShadow` and related structures.
    *   Fetch `N` (Settlement Limit) from `config.Provider`.
*   **[MODIFY]** `server.go`
    *   Remove `check-shadow` route.

#### Frontend: Map Integration
*   **[MODIFY]** `labelService.ts`
    *   **[DELETE]** `checkShadow` method.
*   **[MODIFY]** `ArtisticMap.tsx`
    *   Pass current `map.getZoom()` in the `SyncRequest`.
    *   **Lingering Logic:** Continue to maintain `accumulatedSettlements`. Only remove labels if they are effectively off-screen (PlacementEngine rejection). If a label is not in the `/sync` response but is still in `accumulatedSettlements`, it remains until it naturally drops off.

### Verification Plan

#### Automated Tests
*   Update `manager_test.go` to test sequence of calls (panning) and ensure a shadow item correctly blocks a future item inside the viewport.
*   Verify zoom-level resets.

#### Manual Verification
*   Observe map behavior during slow flight towards a large city. Verify that the city area remains empty of small towns before the city name appears.
*   Verify that zooming in/out clears the map state for a fresh start.
*   Verify no more calls to `/api/map/labels/check-shadow` are made.
