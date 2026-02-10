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


