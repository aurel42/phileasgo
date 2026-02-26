package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"phileasgo/pkg/geo"
)

// FeaturesHandler exposes the currently active spatial features.
type FeaturesHandler struct {
	featureSvc *geo.FeatureService
	telemetry  *TelemetryHandler
}

// NewFeaturesHandler creates a new handler. Returns nil if dependencies are missing.
func NewFeaturesHandler(featureSvc *geo.FeatureService, tel *TelemetryHandler) *FeaturesHandler {
	if featureSvc == nil || tel == nil {
		return nil
	}
	return &FeaturesHandler{
		featureSvc: featureSvc,
		telemetry:  tel,
	}
}

// FeatureResponse represents a single matched spatial feature.
type FeatureResponse struct {
	Name     string `json:"name"`
	QID      string `json:"qid"`
	Category string `json:"category"`
}

// HandleGet returns the spatial features covering the current position.
func (h *FeaturesHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	var lat, lon float64
	var found bool

	// 1. Try query parameters if provided
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	if latStr != "" && lonStr != "" {
		var err1, err2 error
		lat, err1 = strconv.ParseFloat(latStr, 64)
		lon, err2 = strconv.ParseFloat(lonStr, 64)
		if err1 == nil && err2 == nil {
			found = true
		}
	}

	// 2. Fallback to active telemetry
	if !found {
		tel, ok := h.telemetry.GetTelemetry()
		if !ok {
			// No telemetry yet, return empty list
			h.writeEmpty(w)
			return
		}
		lat = tel.Latitude
		lon = tel.Longitude
	}

	if h.featureSvc == nil {
		h.writeEmpty(w)
		return
	}

	features := h.featureSvc.GetFeaturesAtPoint(lat, lon)

	// Convert to response format
	resp := make([]FeatureResponse, len(features))
	for i, f := range features {
		resp[i] = FeatureResponse{
			Name:     f.Name,
			QID:      f.QID,
			Category: f.Category,
		}
	}

	// Sort by Category, then by Name
	sort.Slice(resp, func(i, j int) bool {
		if resp[i].Category != resp[j].Category {
			return resp[i].Category < resp[j].Category
		}
		return resp[i].Name < resp[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode features response", "error", err)
	}
}

func (h *FeaturesHandler) writeEmpty(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("[]\n"))
}
