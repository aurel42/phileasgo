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
	if m == nil {
		return nil
	}
	return &MapLabelsHandler{manager: m}
}

// SyncRequest represents the payload for synchronizing labels.
type SyncRequest struct {
	BBox     [4]float64  `json:"bbox"` // [minLat, minLon, maxLat, maxLon]
	ACLat    float64     `json:"ac_lat"`
	ACLon    float64     `json:"ac_lon"`
	Heading  float64     `json:"heading"`
	Existing []geo.Point `json:"existing"`
	Zoom     float64     `json:"zoom"`
}

// SyncResponse represents the label set for the viewport.
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
		req.Zoom,
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
