package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"phileasgo/pkg/store"
)

// ConfigHandler handles configuration API requests.
type ConfigHandler struct {
	store store.Store
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(st store.Store) *ConfigHandler {
	return &ConfigHandler{store: st}
}

// ConfigResponse represents the config API response.
type ConfigResponse struct {
	SimSource           string  `json:"sim_source"`
	Units               string  `json:"units"`
	ShowCacheLayer      bool    `json:"show_cache_layer"`
	ShowVisibilityLayer bool    `json:"show_visibility_layer"`
	MinPOIScore         float64 `json:"min_poi_score"`
	Volume              float64 `json:"volume"`
}

// ConfigRequest represents the config API request for updates.
type ConfigRequest struct {
	SimSource           string   `json:"sim_source,omitempty"`
	Units               string   `json:"units,omitempty"`
	ShowCacheLayer      *bool    `json:"show_cache_layer,omitempty"`      // Pointer to detect false vs missing
	ShowVisibilityLayer *bool    `json:"show_visibility_layer,omitempty"` // Pointer to detect false vs missing
	MinPOIScore         *float64 `json:"min_poi_score,omitempty"`
}

// HandleGetConfig returns the current configuration.
func (h *ConfigHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	simSource, _ := h.store.GetState(ctx, "sim_source")
	if simSource == "" {
		simSource = "simconnect" // Default
	}

	units, _ := h.store.GetState(ctx, "units")
	if units == "" {
		units = "km" // Default
	}

	showCache, _ := h.store.GetState(ctx, "show_cache_layer")
	showCacheBool := showCache == "true"

	showVis, _ := h.store.GetState(ctx, "show_visibility_layer")
	showVisBool := showVis == "true"

	minScoreStr, _ := h.store.GetState(ctx, "min_poi_score")
	minScore := 0.5 // Default
	if minScoreStr != "" {
		// Basic parsing
		var val float64
		if _, err := fmt.Sscanf(minScoreStr, "%f", &val); err == nil {
			minScore = val
		}
	}

	volStr, _ := h.store.GetState(ctx, "volume")
	volume := 1.0 // Default
	if volStr != "" {
		var val float64
		if _, err := fmt.Sscanf(volStr, "%f", &val); err == nil {
			volume = val
		}
	}

	resp := ConfigResponse{
		SimSource:           simSource,
		Units:               units,
		ShowCacheLayer:      showCacheBool,
		ShowVisibilityLayer: showVisBool,
		MinPOIScore:         minScore,
		Volume:              volume,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode config response", "error", err)
	}
}

// HandleSetConfig updates the configuration.
func (h *ConfigHandler) HandleSetConfig(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req ConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Update Fields
	if req.SimSource != "" {
		if err := h.updateSimSource(ctx, req.SimSource); err != nil {
			slog.Error("Failed to save sim_source", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.Units != "" {
		if err := h.updateUnits(ctx, req.Units); err != nil {
			slog.Error("Failed to save units", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.ShowCacheLayer != nil {
		h.updateBoolState(ctx, "show_cache_layer", *req.ShowCacheLayer)
	}

	if req.ShowVisibilityLayer != nil {
		h.updateBoolState(ctx, "show_visibility_layer", *req.ShowVisibilityLayer)
	}

	if req.MinPOIScore != nil {
		score := *req.MinPOIScore
		// Validation removed to allow full range

		strVal := fmt.Sprintf("%.2f", score)
		if err := h.store.SetState(ctx, "min_poi_score", strVal); err != nil {
			slog.Error("Failed to save state", "key", "min_poi_score", "error", err)
		} else {
			slog.Info("Config updated", "min_poi_score", strVal)
		}
	}

	// Return updated config
	h.HandleGetConfig(w, r)
}

func (h *ConfigHandler) updateSimSource(ctx context.Context, val string) error {
	if val != "mock" && val != "simconnect" {
		return io.ErrUnexpectedEOF // Hacky error reuse or create custom
	}
	if err := h.store.SetState(ctx, "sim_source", val); err != nil {
		return err
	}
	slog.Info("Config updated", "sim_source", val)
	return nil
}

func (h *ConfigHandler) updateUnits(ctx context.Context, val string) error {
	if val != "km" && val != "nm" {
		return io.ErrUnexpectedEOF
	}
	if err := h.store.SetState(ctx, "units", val); err != nil {
		return err
	}
	slog.Info("Config updated", "units", val)
	return nil
}

func (h *ConfigHandler) updateBoolState(ctx context.Context, key string, val bool) {
	strVal := "false"
	if val {
		strVal = "true"
	}
	if err := h.store.SetState(ctx, key, strVal); err != nil {
		slog.Error("Failed to save state", "key", key, "error", err)
	} else {
		slog.Info("Config updated", key, strVal)
	}
}
