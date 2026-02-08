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
	SimSource                   string   `json:"sim_source"`
	Units                       string   `json:"units"`            // Prompt template units (imperial/hybrid/metric) - not used by frontend
	RangeRingUnits              string   `json:"range_ring_units"` // Map display units (km/nm) - used by frontend
	TTSEngine                   string   `json:"tts_engine"`
	ShowCacheLayer              bool     `json:"show_cache_layer"`
	ShowVisibilityLayer         bool     `json:"show_visibility_layer"`
	RenderVisibilityAsMap       bool     `json:"render_visibility_as_map"`
	MinPOIScore                 float64  `json:"min_poi_score"`
	Volume                      float64  `json:"volume"`
	FilterMode                  string   `json:"filter_mode"`
	TargetPOICount              int      `json:"target_poi_count"`
	NarrationFrequency          int      `json:"narration_frequency"`
	TextLength                  int      `json:"text_length"`
	ShowMapBox                  bool     `json:"show_map_box"`
	ShowPOIInfo                 bool     `json:"show_poi_info"`
	ShowInfoBar                 bool     `json:"show_info_bar"`
	ShowLogLine                 bool     `json:"show_log_line"`
	LLMProvider                 string   `json:"llm_provider"`
	TeleportDistance            float64  `json:"teleport_distance"`
	MockStartLat                float64  `json:"mock_start_lat"`
	MockStartLon                float64  `json:"mock_start_lon"`
	MockStartAlt                float64  `json:"mock_start_alt"`
	MockStartHeading            *float64 `json:"mock_start_heading"`
	MockDurationParked          string   `json:"mock_duration_parked"`
	MockDurationTaxi            string   `json:"mock_duration_taxi"`
	MockDurationHold            string   `json:"mock_duration_hold"`
	StyleLibrary                []string `json:"style_library"`
	ActiveStyle                 string   `json:"active_style"`
	SecretWordLibrary           []string `json:"secret_word_library"`
	ActiveSecretWord            string   `json:"active_secret_word"`
	TargetLanguageLibrary       []string `json:"target_language_library"`
	ActiveTargetLanguage        string   `json:"active_target_language"`
	DeferralThreshold           float64  `json:"deferral_threshold"`
	DeferralProximityBoostPower float64  `json:"deferral_proximity_boost_power"`
	TwoPassScriptGeneration     bool     `json:"two_pass_script_generation"`
	// Beacon
	BeaconEnabled           bool    `json:"beacon_enabled"`
	BeaconFormationEnabled  bool    `json:"beacon_formation_enabled"`
	BeaconFormationDistance float64 `json:"beacon_formation_distance"`
	BeaconFormationCount    int     `json:"beacon_formation_count"`
	BeaconMinSpawnAltitude  float64 `json:"beacon_min_spawn_altitude"`
	BeaconAltitudeFloor     float64 `json:"beacon_altitude_floor"`
	BeaconSinkDistanceFar   float64 `json:"beacon_sink_distance_far"`
	BeaconSinkDistanceClose float64 `json:"beacon_sink_distance_close"`
	BeaconMaxTargets        int     `json:"beacon_max_targets"`
	AutoNarrate             bool    `json:"auto_narrate"`
	PauseBetweenNarrations  float64 `json:"pause_between_narrations"`
	RepeatTTL               string  `json:"repeat_ttl"`
	NarrationLengthShort    int     `json:"narration_length_short_words"`
	NarrationLengthLong     int     `json:"narration_length_long_words"`
}

// ConfigRequest represents the config API request for updates.
type ConfigRequest struct {
	SimSource                   string   `json:"sim_source,omitempty"`
	Units                       string   `json:"units,omitempty"`                 // Prompt template units (imperial/hybrid/metric)
	RangeRingUnits              string   `json:"range_ring_units,omitempty"`      // Map display units (km/nm)
	ShowCacheLayer              *bool    `json:"show_cache_layer,omitempty"`      // Pointer to detect false vs missing
	ShowVisibilityLayer         *bool    `json:"show_visibility_layer,omitempty"` // Pointer to detect false vs missing
	RenderVisibilityAsMap       *bool    `json:"render_visibility_as_map,omitempty"`
	MinPOIScore                 *float64 `json:"min_poi_score,omitempty"`
	FilterMode                  string   `json:"filter_mode,omitempty"`
	TargetPOICount              *int     `json:"target_poi_count,omitempty"`
	NarrationFrequency          *int     `json:"narration_frequency,omitempty"`
	TextLength                  *int     `json:"text_length,omitempty"`
	TeleportDistance            *float64 `json:"teleport_distance,omitempty"`
	MockStartLat                *float64 `json:"mock_start_lat,omitempty"`
	MockStartLon                *float64 `json:"mock_start_lon,omitempty"`
	MockStartAlt                *float64 `json:"mock_start_alt,omitempty"`
	MockStartHeading            *float64 `json:"mock_start_heading,omitempty"`
	MockDurationParked          string   `json:"mock_duration_parked,omitempty"`
	MockDurationTaxi            string   `json:"mock_duration_taxi,omitempty"`
	MockDurationHold            string   `json:"mock_duration_hold,omitempty"`
	StyleLibrary                []string `json:"style_library,omitempty"`
	ActiveStyle                 *string  `json:"active_style,omitempty"` // Pointer to detect empty string vs missing
	SecretWordLibrary           []string `json:"secret_word_library,omitempty"`
	ActiveSecretWord            *string  `json:"active_secret_word,omitempty"` // Pointer to detect empty string vs missing
	TargetLanguageLibrary       []string `json:"target_language_library,omitempty"`
	ActiveTargetLanguage        *string  `json:"active_target_language,omitempty"`
	DeferralThreshold           *float64 `json:"deferral_threshold,omitempty"`
	DeferralProximityBoostPower *float64 `json:"deferral_proximity_boost_power,omitempty"`
	TwoPassScriptGeneration     *bool    `json:"two_pass_script_generation,omitempty"`
	// Beacon
	BeaconEnabled           *bool    `json:"beacon_enabled,omitempty"`
	BeaconFormationEnabled  *bool    `json:"beacon_formation_enabled,omitempty"`
	BeaconFormationDistance *float64 `json:"beacon_formation_distance,omitempty"`
	BeaconFormationCount    *int     `json:"beacon_formation_count,omitempty"`
	BeaconMinSpawnAltitude  *float64 `json:"beacon_min_spawn_altitude,omitempty"`
	BeaconAltitudeFloor     *float64 `json:"beacon_altitude_floor,omitempty"`
	BeaconSinkDistanceFar   *float64 `json:"beacon_sink_distance_far,omitempty"`
	BeaconSinkDistanceClose *float64 `json:"beacon_sink_distance_close,omitempty"`
	BeaconMaxTargets        *int     `json:"beacon_max_targets,omitempty"`
	AutoNarrate             *bool    `json:"auto_narrate,omitempty"`
	PauseBetweenNarrations  *float64 `json:"pause_between_narrations,omitempty"`
	RepeatTTL               *string  `json:"repeat_ttl,omitempty"`
	NarrationLengthShort    *int     `json:"narration_length_short_words,omitempty"`
	NarrationLengthLong     *int     `json:"narration_length_long_words,omitempty"`
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
		SimSource:                   h.cfgProv.SimProvider(ctx),
		Units:                       h.cfgProv.Units(ctx),          // Prompt template units (imperial/hybrid/metric) - backend only
		RangeRingUnits:              h.cfgProv.RangeRingUnits(ctx), // Map display units (km/nm) - frontend only
		TTSEngine:                   h.appCfg.TTS.Engine,
		ShowCacheLayer:              h.cfgProv.ShowCacheLayer(ctx),
		ShowVisibilityLayer:         h.cfgProv.ShowVisibilityLayer(ctx),
		RenderVisibilityAsMap:       h.cfgProv.RenderVisibilityAsMap(ctx),
		MinPOIScore:                 h.cfgProv.MinScoreThreshold(ctx),
		Volume:                      volume, // Volume is not migrated to cfgProv
		FilterMode:                  h.cfgProv.FilterMode(ctx),
		TargetPOICount:              h.cfgProv.TargetPOICount(ctx),
		NarrationFrequency:          h.cfgProv.NarrationFrequency(ctx),
		TextLength:                  h.cfgProv.TextLengthScale(ctx),
		ShowMapBox:                  h.appCfg.Overlay.MapBox,
		ShowPOIInfo:                 h.appCfg.Overlay.POIInfo,
		ShowInfoBar:                 h.appCfg.Overlay.InfoBar,
		ShowLogLine:                 h.appCfg.Overlay.LogLine,
		LLMProvider:                 h.getPrimaryLLMProvider(),
		TeleportDistance:            h.cfgProv.TeleportDistance(ctx),
		MockStartLat:                h.cfgProv.MockStartLat(ctx),
		MockStartLon:                h.cfgProv.MockStartLon(ctx),
		MockStartAlt:                h.cfgProv.MockStartAlt(ctx),
		MockStartHeading:            h.cfgProv.MockStartHeading(ctx),
		MockDurationParked:          h.cfgProv.MockDurationParked(ctx).String(),
		MockDurationTaxi:            h.cfgProv.MockDurationTaxi(ctx).String(),
		MockDurationHold:            h.cfgProv.MockDurationHold(ctx).String(),
		StyleLibrary:                h.cfgProv.StyleLibrary(ctx),
		ActiveStyle:                 h.cfgProv.ActiveStyle(ctx),
		SecretWordLibrary:           h.cfgProv.SecretWordLibrary(ctx),
		ActiveSecretWord:            h.cfgProv.ActiveSecretWord(ctx),
		TargetLanguageLibrary:       h.cfgProv.TargetLanguageLibrary(ctx),
		ActiveTargetLanguage:        h.cfgProv.ActiveTargetLanguage(ctx),
		DeferralThreshold:           h.cfgProv.DeferralThreshold(ctx),
		DeferralProximityBoostPower: h.cfgProv.DeferralProximityBoostPower(ctx),
		TwoPassScriptGeneration:     h.cfgProv.TwoPassScriptGeneration(ctx),
		BeaconEnabled:               h.cfgProv.BeaconEnabled(ctx),
		BeaconFormationEnabled:      h.cfgProv.BeaconFormationEnabled(ctx),
		BeaconFormationDistance:     float64(h.cfgProv.BeaconFormationDistance(ctx)),
		BeaconFormationCount:        h.cfgProv.BeaconFormationCount(ctx),
		BeaconMinSpawnAltitude:      float64(h.cfgProv.BeaconMinSpawnAltitude(ctx)),
		BeaconAltitudeFloor:         float64(h.cfgProv.BeaconAltitudeFloor(ctx)),
		BeaconSinkDistanceFar:       float64(h.cfgProv.BeaconSinkDistanceFar(ctx)),
		BeaconSinkDistanceClose:     float64(h.cfgProv.BeaconSinkDistanceClose(ctx)),
		BeaconMaxTargets:            h.cfgProv.BeaconMaxTargets(ctx),
		AutoNarrate:                 h.cfgProv.AutoNarrate(ctx),
		PauseBetweenNarrations:      h.cfgProv.PauseDuration(ctx).Seconds(),
		RepeatTTL:                   h.cfgProv.RepeatTTL(ctx).String(),
		NarrationLengthShort:        h.cfgProv.NarrationLengthShort(ctx),
		NarrationLengthLong:         h.cfgProv.NarrationLengthLong(ctx),
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

	// Core updates (return error to client if they fail)
	if err := h.applyCoreUpdates(ctx, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Other updates (logged but don't block)
	h.applyUIUpdates(ctx, &req)
	h.applyThresholdUpdates(ctx, &req)
	h.applyMockUpdates(ctx, &req, body)
	h.applyStyleUpdates(ctx, &req)
	h.applyLanguageUpdates(ctx, &req)
	h.applyBeaconUpdates(ctx, &req)

	// Return updated config
	h.HandleGetConfig(w, r)
}

func (h *ConfigHandler) applyCoreUpdates(ctx context.Context, req *ConfigRequest) error {
	if req.SimSource != "" {
		if err := h.updateSimSource(ctx, req.SimSource); err != nil {
			slog.Error("Failed to save sim_source", "error", err)
			return err
		}
	}

	if req.Units != "" {
		if err := h.updateUnits(ctx, req.Units); err != nil {
			slog.Error("Failed to save units", "error", err)
			return err
		}
	}

	if req.FilterMode != "" {
		h.updateFilterMode(ctx, req.FilterMode)
	}

	return nil
}

func (h *ConfigHandler) applyUIUpdates(ctx context.Context, req *ConfigRequest) {
	if req.ShowCacheLayer != nil {
		h.updateBoolState(ctx, config.KeyShowCacheLayer, *req.ShowCacheLayer)
	}

	if req.ShowVisibilityLayer != nil {
		h.updateBoolState(ctx, config.KeyShowVisibility, *req.ShowVisibilityLayer)
	}
	if req.RenderVisibilityAsMap != nil {
		h.updateBoolState(ctx, config.KeyRenderVisibilityAsMap, *req.RenderVisibilityAsMap)
	}

	if req.RangeRingUnits != "" && (req.RangeRingUnits == "km" || req.RangeRingUnits == "nm") {
		_ = h.store.SetState(ctx, config.KeyRangeRingUnits, req.RangeRingUnits)
		slog.Debug("Config updated", "range_ring_units", req.RangeRingUnits)
	}
}

func (h *ConfigHandler) applyThresholdUpdates(ctx context.Context, req *ConfigRequest) {
	if req.MinPOIScore != nil {
		h.updateFloatState(ctx, config.KeyMinPOIScore, *req.MinPOIScore)
	}
	if req.TargetPOICount != nil {
		h.updateIntState(ctx, config.KeyTargetPOICount, *req.TargetPOICount)
	}
	if req.NarrationFrequency != nil {
		h.updateIntState(ctx, config.KeyNarrationFrequency, *req.NarrationFrequency)
	}
	if req.TextLength != nil {
		h.updateIntState(ctx, config.KeyTextLength, *req.TextLength)
	}
	if req.TeleportDistance != nil {
		h.updateFloatState(ctx, config.KeyTeleportDistance, *req.TeleportDistance)
	}
	if req.DeferralThreshold != nil {
		h.updateFloatState(ctx, config.KeyDeferralThreshold, *req.DeferralThreshold)
	}
	if req.DeferralProximityBoostPower != nil {
		h.updateFloatState(ctx, config.KeyDeferralProximityBoostPower, *req.DeferralProximityBoostPower)
	}
	if req.TwoPassScriptGeneration != nil {
		h.updateBoolState(ctx, config.KeyTwoPassScriptGeneration, *req.TwoPassScriptGeneration)
	}
	if req.AutoNarrate != nil {
		h.updateBoolState(ctx, config.KeyAutoNarrate, *req.AutoNarrate)
	}
	if req.PauseBetweenNarrations != nil {
		strVal := fmt.Sprintf("%.0fs", *req.PauseBetweenNarrations)
		_ = h.store.SetState(ctx, config.KeyPauseDuration, strVal)
		slog.Debug("Config updated", config.KeyPauseDuration, strVal)
	}
	if req.RepeatTTL != nil {
		_ = h.store.SetState(ctx, config.KeyRepeatTTL, *req.RepeatTTL)
		slog.Debug("Config updated", config.KeyRepeatTTL, *req.RepeatTTL)
	}
	if req.NarrationLengthShort != nil {
		h.updateIntState(ctx, config.KeyNarrationLengthShort, *req.NarrationLengthShort)
	}
	if req.NarrationLengthLong != nil {
		h.updateIntState(ctx, config.KeyNarrationLengthLong, *req.NarrationLengthLong)
	}
}

func (h *ConfigHandler) applyMockUpdates(ctx context.Context, req *ConfigRequest, body []byte) {
	if req.MockStartLat != nil {
		h.updateFloatState(ctx, config.KeyMockLat, *req.MockStartLat)
	}
	if req.MockStartLon != nil {
		h.updateFloatState(ctx, config.KeyMockLon, *req.MockStartLon)
	}
	if req.MockStartAlt != nil {
		h.updateFloatState(ctx, config.KeyMockAlt, *req.MockStartAlt)
	}
	if req.MockStartHeading != nil {
		h.updateFloatState(ctx, config.KeyMockHeading, *req.MockStartHeading)
	} else if req.MockStartHeading == nil && containsJSONKey(body, "mock_start_heading") {
		// Explicit null means random (heading removed)
		_ = h.store.DeleteState(ctx, config.KeyMockHeading)
	}

	if req.MockDurationParked != "" {
		_ = h.store.SetState(ctx, config.KeyMockDurParked, req.MockDurationParked)
	}
	if req.MockDurationTaxi != "" {
		_ = h.store.SetState(ctx, config.KeyMockDurTaxi, req.MockDurationTaxi)
	}
	if req.MockDurationHold != "" {
		_ = h.store.SetState(ctx, config.KeyMockDurHold, req.MockDurationHold)
	}
}

func (h *ConfigHandler) applyStyleUpdates(ctx context.Context, req *ConfigRequest) {
	if req.StyleLibrary != nil {
		jsonBytes, err := json.Marshal(req.StyleLibrary)
		if err == nil {
			_ = h.store.SetState(ctx, config.KeyStyleLibrary, string(jsonBytes))
			slog.Debug("Config updated", config.KeyStyleLibrary, string(jsonBytes))
		}
	}
	if req.ActiveStyle != nil {
		_ = h.store.SetState(ctx, config.KeyActiveStyle, *req.ActiveStyle)
		slog.Debug("Config updated", config.KeyActiveStyle, *req.ActiveStyle)
	}
	if req.SecretWordLibrary != nil {
		jsonBytes, err := json.Marshal(req.SecretWordLibrary)
		if err == nil {
			_ = h.store.SetState(ctx, config.KeySecretWordLibrary, string(jsonBytes))
			slog.Debug("Config updated", config.KeySecretWordLibrary, string(jsonBytes))
		}
	}
	if req.ActiveSecretWord != nil {
		_ = h.store.SetState(ctx, config.KeyActiveSecretWord, *req.ActiveSecretWord)
		slog.Debug("Config updated", config.KeyActiveSecretWord, *req.ActiveSecretWord)
	}
}

func (h *ConfigHandler) applyLanguageUpdates(ctx context.Context, req *ConfigRequest) {
	if req.TargetLanguageLibrary != nil {
		jsonBytes, err := json.Marshal(req.TargetLanguageLibrary)
		if err == nil {
			_ = h.store.SetState(ctx, config.KeyTargetLanguageLibrary, string(jsonBytes))
			slog.Debug("Config updated", config.KeyTargetLanguageLibrary, string(jsonBytes))
		}
	}
	if req.ActiveTargetLanguage != nil {
		_ = h.store.SetState(ctx, config.KeyActiveTargetLanguage, *req.ActiveTargetLanguage)
		slog.Debug("Config updated", config.KeyActiveTargetLanguage, *req.ActiveTargetLanguage)
	}
}

func (h *ConfigHandler) applyBeaconUpdates(ctx context.Context, req *ConfigRequest) {
	if req.BeaconEnabled != nil {
		h.updateBoolState(ctx, config.KeyBeaconEnabled, *req.BeaconEnabled)
	}
	if req.BeaconFormationEnabled != nil {
		h.updateBoolState(ctx, config.KeyBeaconFormationEnabled, *req.BeaconFormationEnabled)
	}
	if req.BeaconFormationDistance != nil {
		h.updateFloatState(ctx, config.KeyBeaconFormationDistance, *req.BeaconFormationDistance)
	}
	if req.BeaconFormationCount != nil {
		h.updateIntState(ctx, config.KeyBeaconFormationCount, *req.BeaconFormationCount)
	}
	if req.BeaconMinSpawnAltitude != nil {
		h.updateFloatState(ctx, config.KeyBeaconMinSpawnAltitude, *req.BeaconMinSpawnAltitude)
	}
	if req.BeaconAltitudeFloor != nil {
		h.updateFloatState(ctx, config.KeyBeaconAltitudeFloor, *req.BeaconAltitudeFloor)
	}
	if req.BeaconSinkDistanceFar != nil {
		h.updateFloatState(ctx, config.KeyBeaconSinkDistanceFar, *req.BeaconSinkDistanceFar)
	}
	if req.BeaconSinkDistanceClose != nil {
		h.updateFloatState(ctx, config.KeyBeaconSinkDistanceClose, *req.BeaconSinkDistanceClose)
	}
	if req.BeaconMaxTargets != nil {
		h.updateIntState(ctx, config.KeyBeaconMaxTargets, *req.BeaconMaxTargets)
	}
}

func (h *ConfigHandler) updateSimSource(ctx context.Context, val string) error {
	if val != "mock" && val != "simconnect" {
		return io.ErrUnexpectedEOF // Hacky error reuse or create custom
	}
	if err := h.store.SetState(ctx, config.KeySimSource, val); err != nil {
		return err
	}
	slog.Debug("Config updated", "sim_source", val)
	return nil
}

func (h *ConfigHandler) updateUnits(ctx context.Context, val string) error {
	if val != "imperial" && val != "hybrid" && val != "metric" {
		return io.ErrUnexpectedEOF
	}
	if err := h.store.SetState(ctx, config.KeyUnits, val); err != nil {
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
		if err := h.store.SetState(ctx, config.KeyFilterMode, val); err != nil {
			slog.Error("Failed to save state", "key", config.KeyFilterMode, "error", err)
		} else {
			slog.Debug("Config updated", config.KeyFilterMode, val)
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
