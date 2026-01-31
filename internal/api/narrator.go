package api

import (
	"context" // Added
	"encoding/json"
	"log/slog"

	"net/http"
	"reflect"
	"sync"

	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"strconv"
)

// AudioController defines methods for controlling audio playback.
type AudioController interface {
	IsPlaying() bool
	IsBusy() bool
	ResetUserPause()
	Resume()
	IsUserPaused() bool
}

// NarratorController defines methods for controlling and viewing narration state.
type NarratorController interface {
	IsActive() bool
	IsGenerating() bool
	PlayPOI(ctx context.Context, id string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string)
	CurrentPOI() *model.POI
	GetPreparedPOI() *model.POI
	CurrentTitle() string
	CurrentThumbnailURL() string // Added
	CurrentType() model.NarrativeType
	CurrentImagePath() string
	ClearCurrentImage() // Added
	NarratedCount() int
	Stats() map[string]any
}

// NarratorHandler handles narrator control endpoints.
type NarratorHandler struct {
	audio    AudioController
	narrator NarratorController
	store    store.Store

	statusMu           sync.Mutex
	lastStatusResponse *NarratorStatusResponse
}

// NewNarratorHandler creates a new NarratorHandler.
func NewNarratorHandler(audioMgr AudioController, narratorSvc NarratorController, st store.Store) *NarratorHandler {
	return &NarratorHandler{
		audio:    audioMgr,
		narrator: narratorSvc,
		store:    st,
	}
}

// PlayRequest represents a manual narration play request.
type PlayRequest struct {
	POIID    string `json:"poi_id"`
	Strategy string `json:"strategy"` // Optional: uniform, min_skew, max_skew
}

// NarratorStatusResponse represents the narrator status.
type NarratorStatusResponse struct {
	Active             bool           `json:"active"`
	PlaybackStatus     string         `json:"playback_status"` // idle, preparing, playing, paused
	CurrentPOI         *model.POI     `json:"current_poi,omitempty"`
	PreparingPOI       *model.POI     `json:"preparing_poi,omitempty"`
	CurrentTitle       string         `json:"current_title"`
	CurrentType        string         `json:"current_type"`
	CurrentImagePath   string         `json:"current_image_path,omitempty"`
	DisplayTitle       string         `json:"display_title"`     // Added
	DisplayThumbnail   string         `json:"display_thumbnail"` // Added
	NarratedCount      int            `json:"narrated_count"`
	Stats              map[string]any `json:"stats"`
	NarrationFrequency int            `json:"narration_frequency"`
	TextLength         int            `json:"text_length"`
}

// HandlePlay handles POST /api/narrator/play
func (h *NarratorHandler) HandlePlay(w http.ResponseWriter, r *http.Request) {
	slog.Info("API: HandlePlay called")

	var req PlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("API: HandlePlay decode error", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	slog.Info("API: HandlePlay received POI request", "poi_id", req.POIID, "strategy", req.Strategy)

	// Reset pause state if user paused
	if h.audio.IsUserPaused() {
		h.audio.ResetUserPause()
		h.audio.Resume()
	}

	// Trigger narration asynchronously (Manual play -> uniform strategy, or pass explicitly if needed)
	// We use background context because the HTTP request context will be canceled when this handler returns.
	// enqueueIfBusy = true to support queuing user requests if narration is ongoing
	go h.narrator.PlayPOI(context.Background(), req.POIID, true, true, nil, req.Strategy)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "upcoming",
		"message": "Queued " + req.POIID + " for narration",
	}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// HandleStatus handles GET /api/narrator/status
func (h *NarratorHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	status := "idle"
	isActive := h.narrator.IsActive()
	isUserPaused := h.audio.IsUserPaused()

	if isUserPaused {
		status = "paused"
	} else if isActive {
		// If narrator is active, check audio state
		switch {
		case h.audio.IsPlaying():
			status = "playing"
		case h.audio.IsBusy():
			// Busy but not playing = Paused
			status = "paused"
		case h.narrator.IsGenerating():
			// Active and generating = preparing
			status = "preparing"
		default:
			// Active but not playing/generating = Cooldown/Finishing
			status = "idle"
		}
	}

	// Fetch current config from store to allow synchronization
	ctx := r.Context()
	freq := 3
	if fStr, ok := h.store.GetState(ctx, "narration_frequency"); ok && fStr != "" {
		if val, err := strconv.Atoi(fStr); err == nil {
			freq = val
		}
	}
	textLen := 3
	if tStr, ok := h.store.GetState(ctx, "text_length"); ok && tStr != "" {
		if val, err := strconv.Atoi(tStr); err == nil {
			textLen = val
		}
	}

	resp := NarratorStatusResponse{
		Active:             isActive,
		PlaybackStatus:     status,
		CurrentPOI:         h.narrator.CurrentPOI(),
		PreparingPOI:       h.narrator.GetPreparedPOI(),
		CurrentTitle:       h.narrator.CurrentTitle(),
		CurrentType:        string(h.narrator.CurrentType()),
		CurrentImagePath:   h.narrator.CurrentImagePath(),
		DisplayTitle:       h.narrator.CurrentTitle(),
		DisplayThumbnail:   h.narrator.CurrentThumbnailURL(),
		NarratedCount:      h.narrator.NarratedCount(),
		Stats:              h.narrator.Stats(),
		NarrationFrequency: freq,
		TextLength:         textLen,
	}

	// Check if state changed
	h.statusMu.Lock()
	if !reflect.DeepEqual(h.lastStatusResponse, &resp) {
		logging.TraceDefault("Narrator state changed", "old", h.lastStatusResponse, "new", resp)
		// Create a copy to store
		stored := resp
		h.lastStatusResponse = &stored
	}
	h.statusMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// HandleClearImage handles POST /api/narrator/clear-image
func (h *NarratorHandler) HandleClearImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.narrator.ClearCurrentImage()

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}
