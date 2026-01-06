package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"phileasgo/pkg/audio"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
)

// NarratorHandler handles narrator control endpoints.
type NarratorHandler struct {
	audio    audio.Service
	narrator narrator.Service
}

// NewNarratorHandler creates a new NarratorHandler.
func NewNarratorHandler(audioMgr audio.Service, narratorSvc narrator.Service) *NarratorHandler {
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

	var req PlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Reset pause state if user paused
	if h.audio.IsUserPaused() {
		h.audio.ResetUserPause()
		h.audio.Resume()
	}

	// Trigger narration
	h.narrator.PlayPOI(r.Context(), req.POIID, true, nil)

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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}
