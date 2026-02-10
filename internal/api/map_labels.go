package api

import (
	"encoding/json"
	"net/http"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/map/labels"
)

type MapLabelsHandler struct {
	manager *labels.Manager
}

func NewMapLabelsHandler(m *labels.Manager) *MapLabelsHandler {
	return &MapLabelsHandler{manager: m}
}

// SyncRequest represents the payload for synchronizing labels.
type SyncRequest struct {
	BBox     [4]float64  `json:"bbox"` // [minLat, minLon, maxLat, maxLon]
	ACLat    float64     `json:"ac_lat"`
	ACLon    float64     `json:"ac_lon"`
	Heading  float64     `json:"heading"`
	Existing []geo.Point `json:"existing"`
}

// SyncResponse represents the delta update for labels.
// For now, we just return the full list of "Active" labels for the viewport.
// The frontend can diff this if needed, or we can just replace the set.
// A true delta would require tracking client state ID, which we might add later.
// For simplicity in B2, we return the "Target State" for the viewport.
type SyncResponse struct {
	Labels []LabelDTO `json:"labels"`
}

type LabelDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Pop      int     `json:"pop"`
	Category string  `json:"category"`
}

// HandleSync calculates the optimal labels for the given viewport and aircraft state.
// POST /api/map/labels/sync
func (h *MapLabelsHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	candidates := h.manager.SelectLabels(
		req.BBox[0], req.BBox[1], req.BBox[2], req.BBox[3],
		req.ACLat, req.ACLon, req.Heading,
		req.Existing,
	)

	resp := SyncResponse{
		Labels: make([]LabelDTO, len(candidates)),
	}

	for i, c := range candidates {
		resp.Labels[i] = LabelDTO{
			ID:       c.GenericID,
			Name:     c.City.Name,
			Lat:      c.City.Lat,
			Lon:      c.City.Lon,
			Pop:      c.City.Population,
			Category: c.Category,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CheckShadowRequest payload
type CheckShadowRequest struct {
	ACLat   float64 `json:"ac_lat"`
	ACLon   float64 `json:"ac_lon"`
	Heading float64 `json:"heading"`
}

type CheckShadowResponse struct {
	Shadow bool `json:"shadow"`
}

// HandleCheckShadow checks if a major city is looming ahead to suppress local discoveries.
// POST /api/map/labels/check-shadow
func (h *MapLabelsHandler) HandleCheckShadow(w http.ResponseWriter, r *http.Request) {
	var req CheckShadowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Logic: Query a "Lookahead BBox" in front of the aircraft.
	// For B2, let's look 50km ahead.
	// This is a simplified implementation. A proper one would project a polygon.
	// We'll reuse SelectLabels with a synthesized bbox ahead of the plane
	// and check if any HIGH SCORE items appear.

	// Create a point 20km ahead
	center := geo.DestinationPoint(geo.Point{Lat: req.ACLat, Lon: req.ACLon}, 20000, req.Heading)

	// Make a small bbox around it (approx +/- 0.2 deg)
	minLat, maxLat := center.Lat-0.2, center.Lat+0.2
	minLon, maxLon := center.Lon-0.2, center.Lon+0.2

	labels := h.manager.SelectLabels(minLat, minLon, maxLat, maxLon, req.ACLat, req.ACLon, req.Heading, nil)

	shadow := false
	for _, l := range labels {
		// If we found a label ahead with high importance...
		// Threshold: Population > 50k ? Or Score?
		// Let's use Score > 2000 (roughly "Small Name, High Pop" logic)
		if l.FinalScore > 2000 {
			shadow = true
			break
		}
	}

	if err := json.NewEncoder(w).Encode(CheckShadowResponse{Shadow: shadow}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
