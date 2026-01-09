package api

import (
	"context" // Added
	"encoding/json"
	"log/slog"

	"net/http"
	"reflect"
	"sync"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// AudioController defines methods for controlling audio playback.
type AudioController interface {
	IsPlaying() bool
	IsBusy() bool
	ResetUserPause()
	Resume()
	IsUserPaused() bool // Added
}

// NarratorController defines methods for controlling and viewing narration state.
type NarratorController interface {
	IsActive() bool
	IsGenerating() bool
	PlayPOI(ctx context.Context, id string, manual bool, tel *sim.Telemetry, strategy string)
	CurrentPOI() *model.POI
	CurrentTitle() string
	NarratedCount() int
	Stats() map[string]any
}

// NarratorHandler handles narrator control endpoints.
type NarratorHandler struct {
	audio    AudioController
	narrator NarratorController

	statusMu           sync.Mutex
	lastStatusResponse *NarratorStatusResponse
}

// NewNarratorHandler creates a new NarratorHandler.
func NewNarratorHandler(audioMgr AudioController, narratorSvc NarratorController) *NarratorHandler {
	return &NarratorHandler{
		audio:    audioMgr,
		narrator: narratorSvc,
	}
}

// PlayRequest represents a manual narration play request.
type PlayRequest struct {
	POIID string `json:"poi_id"`
}

// NarratorStatusResponse represents the narrator status.
type NarratorStatusResponse struct {
	Active         bool           `json:"active"`
	PlaybackStatus string         `json:"playback_status"` // idle, preparing, playing, paused
	CurrentPOI     *model.POI     `json:"current_poi,omitempty"`
	CurrentTitle   string         `json:"current_title"`
	NarratedCount  int            `json:"narrated_count"`
	Stats          map[string]any `json:"stats"`
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

	slog.Info("API: HandlePlay received POI request", "poi_id", req.POIID)

	// Reset pause state if user paused
	if h.audio.IsUserPaused() {
		h.audio.ResetUserPause()
		h.audio.Resume()
	}

	// Trigger narration (Manual play -> uniform strategy, or pass explicitly if needed)
	h.narrator.PlayPOI(r.Context(), req.POIID, true, nil, "uniform")

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

	if isActive {
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

	resp := NarratorStatusResponse{
		Active:         isActive,
		PlaybackStatus: status,
		CurrentPOI:     h.narrator.CurrentPOI(),
		CurrentTitle:   h.narrator.CurrentTitle(),
		NarratedCount:  h.narrator.NarratedCount(),
		Stats:          h.narrator.Stats(),
	}

	// Check if state changed
	h.statusMu.Lock()
	if !reflect.DeepEqual(h.lastStatusResponse, &resp) {
		slog.Debug("Narrator state changed", "old", h.lastStatusResponse, "new", resp)
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
