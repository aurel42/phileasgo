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
	store   store.Store
	cfgProv config.Provider
	appCfg  *config.Config
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(st store.Store, cfg config.Provider) *ConfigHandler {
	return &ConfigHandler{
		store:   st,
		cfgProv: cfg,
		appCfg:  cfg.AppConfig(),
	}
}

// ConfigResponse represents the config API response.
type ConfigResponse struct {
	SimSource           string   `json:"sim_source"`
	Units               string   `json:"units"`
	TTSEngine           string   `json:"tts_engine"`
	ShowCacheLayer      bool     `json:"show_cache_layer"`
	ShowVisibilityLayer bool     `json:"show_visibility_layer"`
	MinPOIScore         float64  `json:"min_poi_score"`
	Volume              float64  `json:"volume"`
	FilterMode          string   `json:"filter_mode"`
	TargetPOICount      int      `json:"target_poi_count"`
	NarrationFrequency  int      `json:"narration_frequency"`
	TextLength          int      `json:"text_length"`
	ShowMapBox          bool     `json:"show_map_box"`
	ShowPOIInfo         bool     `json:"show_poi_info"`
	ShowInfoBar         bool     `json:"show_info_bar"`
	ShowLogLine         bool     `json:"show_log_line"`
	LLMProvider         string   `json:"llm_provider"`
	MockStartLat        float64  `json:"mock_start_lat"`
	MockStartLon        float64  `json:"mock_start_lon"`
	MockStartAlt        float64  `json:"mock_start_alt"`
	MockStartHeading    *float64 `json:"mock_start_heading"`
	MockDurationParked  string   `json:"mock_duration_parked"`
	MockDurationTaxi    string   `json:"mock_duration_taxi"`
	MockDurationHold    string   `json:"mock_duration_hold"`
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
	MockStartLat        *float64 `json:"mock_start_lat,omitempty"`
	MockStartLon        *float64 `json:"mock_start_lon,omitempty"`
	MockStartAlt        *float64 `json:"mock_start_alt,omitempty"`
	MockStartHeading    *float64 `json:"mock_start_heading,omitempty"`
	MockDurationParked  string   `json:"mock_duration_parked,omitempty"`
	MockDurationTaxi    string   `json:"mock_duration_taxi,omitempty"`
	MockDurationHold    string   `json:"mock_duration_hold,omitempty"`
}

// HandleConfig is a unified handler for all config-related methods, facilitating CORS/OPTIONS.
func (h *ConfigHandler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.HandleGetConfig(w, r)
	case http.MethodPut, http.MethodPost:
		h.HandleSetConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleGetConfig returns the current configuration.
func (h *ConfigHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := h.getConfigResponse(ctx)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode config response", "error", err)
	}
}

func (h *ConfigHandler) getConfigResponse(ctx context.Context) ConfigResponse {
	// Volume is not yet migrated to cfgProv, read directly
	volStr, _ := h.store.GetState(ctx, "volume")
	volume := 1.0 // Default
	if volStr != "" {
		var val float64
		if _, err := fmt.Sscanf(volStr, "%f", &val); err == nil {
			volume = val
		}
	}

	return ConfigResponse{
		SimSource:           h.cfgProv.SimProvider(ctx),
		Units:               h.cfgProv.Units(ctx),
		TTSEngine:           h.appCfg.TTS.Engine,
		ShowCacheLayer:      h.cfgProv.ShowCacheLayer(ctx),
		ShowVisibilityLayer: h.cfgProv.ShowVisibilityLayer(ctx),
		MinPOIScore:         h.cfgProv.MinScoreThreshold(ctx),
		Volume:              volume, // Volume is not migrated to cfgProv
		FilterMode:          h.cfgProv.FilterMode(ctx),
		TargetPOICount:      h.cfgProv.TargetPOICount(ctx),
		NarrationFrequency:  h.cfgProv.NarrationFrequency(ctx),
		TextLength:          h.cfgProv.TextLengthScale(ctx),
		ShowMapBox:          h.appCfg.Overlay.MapBox,
		ShowPOIInfo:         h.appCfg.Overlay.POIInfo,
		ShowInfoBar:         h.appCfg.Overlay.InfoBar,
		ShowLogLine:         h.appCfg.Overlay.LogLine,
		LLMProvider:         h.getPrimaryLLMProvider(),
		MockStartLat:        h.cfgProv.MockStartLat(ctx),
		MockStartLon:        h.cfgProv.MockStartLon(ctx),
		MockStartAlt:        h.cfgProv.MockStartAlt(ctx),
		MockStartHeading:    h.cfgProv.MockStartHeading(ctx),
		MockDurationParked:  h.cfgProv.MockDurationParked(ctx).String(),
		MockDurationTaxi:    h.cfgProv.MockDurationTaxi(ctx).String(),
		MockDurationHold:    h.cfgProv.MockDurationHold(ctx).String(),
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
	if req.MockStartLat != nil {
		h.updateFloatState(ctx, "mock_start_lat", *req.MockStartLat)
	}
	if req.MockStartLon != nil {
		h.updateFloatState(ctx, "mock_start_lon", *req.MockStartLon)
	}
	if req.MockStartAlt != nil {
		h.updateFloatState(ctx, "mock_start_alt", *req.MockStartAlt)
	}
	if req.MockStartHeading != nil {
		h.updateFloatState(ctx, "mock_start_heading", *req.MockStartHeading)
	} else if req.MockStartHeading == nil && containsJSONKey(body, "mock_start_heading") {
		// Explicit null means random (heading removed)
		_ = h.store.DeleteState(ctx, "mock_start_heading")
	}

	if req.MockDurationParked != "" {
		_ = h.store.SetState(ctx, "mock_duration_parked", req.MockDurationParked)
	}
	if req.MockDurationTaxi != "" {
		_ = h.store.SetState(ctx, "mock_duration_taxi", req.MockDurationTaxi)
	}
	if req.MockDurationHold != "" {
		_ = h.store.SetState(ctx, "mock_duration_hold", req.MockDurationHold)
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
	slog.Debug("Config updated", "sim_source", val)
	return nil
}

func (h *ConfigHandler) updateUnits(ctx context.Context, val string) error {
	if val != "km" && val != "nm" {
		return io.ErrUnexpectedEOF
	}
	if err := h.store.SetState(ctx, "units", val); err != nil {
		return err
	}
	slog.Debug("Config updated", "units", val)
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
		slog.Debug("Config updated", key, strVal)
	}
}

func (h *ConfigHandler) updateFloatState(ctx context.Context, key string, val float64) {
	strVal := fmt.Sprintf("%.2f", val)
	if err := h.store.SetState(ctx, key, strVal); err != nil {
		slog.Error("Failed to save state", "key", key, "error", err)
	} else {
		slog.Debug("Config updated", key, strVal)
	}
}

func (h *ConfigHandler) updateIntState(ctx context.Context, key string, val int) {
	strVal := fmt.Sprintf("%d", val)
	if err := h.store.SetState(ctx, key, strVal); err != nil {
		slog.Error("Failed to save state", "key", key, "error", err)
	} else {
		slog.Debug("Config updated", key, strVal)
	}
}

func (h *ConfigHandler) updateFilterMode(ctx context.Context, val string) {
	if val == "fixed" || val == "adaptive" {
		if err := h.store.SetState(ctx, "filter_mode", val); err != nil {
			slog.Error("Failed to save state", "key", "filter_mode", "error", err)
		} else {
			slog.Debug("Config updated", "filter_mode", val)
		}
	}
}

func containsJSONKey(body []byte, key string) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func (h *ConfigHandler) getPrimaryLLMProvider() string {
	if len(h.appCfg.LLM.Fallback) > 0 {
		return h.appCfg.LLM.Fallback[0]
	}
	return "none"
}
