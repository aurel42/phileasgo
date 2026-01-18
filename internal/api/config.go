package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"phileasgo/pkg/config"
	"phileasgo/pkg/store"
)

// ConfigHandler handles configuration API requests.
type ConfigHandler struct {
	store  store.Store
	appCfg *config.Config
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(st store.Store, appCfg *config.Config) *ConfigHandler {
	return &ConfigHandler{store: st, appCfg: appCfg}
}

// ConfigResponse represents the config API response.
type ConfigResponse struct {
	SimSource           string  `json:"sim_source"`
	Units               string  `json:"units"`
	TTSEngine           string  `json:"tts_engine"`
	ShowCacheLayer      bool    `json:"show_cache_layer"`
	ShowVisibilityLayer bool    `json:"show_visibility_layer"`
	MinPOIScore         float64 `json:"min_poi_score"`
	Volume              float64 `json:"volume"`
	FilterMode          string  `json:"filter_mode"`
	TargetPOICount      int     `json:"target_poi_count"`
	NarrationFrequency  int     `json:"narration_frequency"`
	TextLength          int     `json:"text_length"`
	ShowMapBox          bool    `json:"show_map_box"`
	ShowPOIInfo         bool    `json:"show_poi_info"`
	ShowInfoBar         bool    `json:"show_info_bar"`
	LLMProvider         string  `json:"llm_provider"`
}

// ConfigRequest represents the config API request for updates.
type ConfigRequest struct {
	SimSource           string   `json:"sim_source,omitempty"`
	Units               string   `json:"units,omitempty"`
	ShowCacheLayer      *bool    `json:"show_cache_layer,omitempty"`      // Pointer to detect false vs missing
	ShowVisibilityLayer *bool    `json:"show_visibility_layer,omitempty"` // Pointer to detect false vs missing
	MinPOIScore         *float64 `json:"min_poi_score,omitempty"`
	FilterMode          string   `json:"filter_mode,omitempty"`
	TargetPOICount      *int     `json:"target_poi_count,omitempty"`
	NarrationFrequency  *int     `json:"narration_frequency,omitempty"`
	TextLength          *int     `json:"text_length,omitempty"`
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

	filterMode, _ := h.store.GetState(ctx, "filter_mode")
	if filterMode == "" {
		filterMode = "fixed"
	}

	targetCountStr, _ := h.store.GetState(ctx, "target_poi_count")
	targetCount := 20 // Default
	if targetCountStr != "" {
		var val int
		if _, err := fmt.Sscanf(targetCountStr, "%d", &val); err == nil {
			targetCount = val
		}
	}

	freqStr, _ := h.store.GetState(ctx, "narration_frequency")
	frequency := 3 // Default (Active)
	if freqStr != "" {
		var val int
		if _, err := fmt.Sscanf(freqStr, "%d", &val); err == nil {
			frequency = val
		}
	}

	textLenStr, _ := h.store.GetState(ctx, "text_length")
	textLength := 3 // Default (Normal = x1.5)
	if textLenStr != "" {
		var val int
		if _, err := fmt.Sscanf(textLenStr, "%d", &val); err == nil {
			textLength = val
		}
	}

	resp := ConfigResponse{
		SimSource:           simSource,
		Units:               units,
		TTSEngine:           h.appCfg.TTS.Engine,
		ShowCacheLayer:      showCacheBool,
		ShowVisibilityLayer: showVisBool,
		MinPOIScore:         minScore,
		Volume:              volume,
		FilterMode:          filterMode,
		TargetPOICount:      targetCount,
		NarrationFrequency:  frequency,
		TextLength:          textLength,
		ShowMapBox:          h.appCfg.Overlay.MapBox,
		ShowPOIInfo:         h.appCfg.Overlay.POIInfo,
		ShowInfoBar:         h.appCfg.Overlay.InfoBar,
		LLMProvider:         h.appCfg.LLM.Provider,
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
		h.updateFloatState(ctx, "min_poi_score", *req.MinPOIScore)
	}
	if req.FilterMode != "" {
		h.updateFilterMode(ctx, req.FilterMode)
	}
	if req.TargetPOICount != nil {
		h.updateIntState(ctx, "target_poi_count", *req.TargetPOICount)
	}
	if req.NarrationFrequency != nil {
		h.updateIntState(ctx, "narration_frequency", *req.NarrationFrequency)
	}
	if req.TextLength != nil {
		h.updateIntState(ctx, "text_length", *req.TextLength)
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

func (h *ConfigHandler) updateFloatState(ctx context.Context, key string, val float64) {
	strVal := fmt.Sprintf("%.2f", val)
	if err := h.store.SetState(ctx, key, strVal); err != nil {
		slog.Error("Failed to save state", "key", key, "error", err)
	} else {
		slog.Info("Config updated", key, strVal)
	}
}

func (h *ConfigHandler) updateIntState(ctx context.Context, key string, val int) {
	strVal := fmt.Sprintf("%d", val)
	if err := h.store.SetState(ctx, key, strVal); err != nil {
		slog.Error("Failed to save state", "key", key, "error", err)
	} else {
		slog.Info("Config updated", key, strVal)
	}
}

func (h *ConfigHandler) updateFilterMode(ctx context.Context, val string) {
	if val == "fixed" || val == "adaptive" {
		if err := h.store.SetState(ctx, "filter_mode", val); err != nil {
			slog.Error("Failed to save state", "key", "filter_mode", "error", err)
		} else {
			slog.Info("Config updated", "filter_mode", val)
		}
	}
}
