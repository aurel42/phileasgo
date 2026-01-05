package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"phileasgo/pkg/audio"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/store"
)

// AudioHandler handles audio control endpoints.
type AudioHandler struct {
	audio    audio.Service
	narrator narrator.Service
	store    store.Store
}

// NewAudioHandler creates a new AudioHandler.
func NewAudioHandler(audioMgr audio.Service, narratorSvc narrator.Service, st store.Store) *AudioHandler {
	return &AudioHandler{
		audio:    audioMgr,
		narrator: narratorSvc,
		store:    st,
	}
}

// AudioControlRequest represents an audio control command.
type AudioControlRequest struct {
	Action string `json:"action"` // "pause", "resume", "stop", "skip", "replay"
}

// AudioVolumeRequest represents a volume change request.
type AudioVolumeRequest struct {
	Volume float64 `json:"volume"`
}

// AudioStatusResponse represents the audio status.
type AudioStatusResponse struct {
	IsPlaying    bool    `json:"is_playing"`
	IsPaused     bool    `json:"is_paused"`
	IsUserPaused bool    `json:"is_user_paused"`
	Volume       float64 `json:"volume"`
}

// HandleControl handles POST /api/audio/control
func (h *AudioHandler) HandleControl(w http.ResponseWriter, r *http.Request) {
	var req AudioControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var state string
	switch req.Action {
	case "pause":
		h.audio.Pause()
		h.audio.SetUserPaused(true)
		state = "paused"
	case "resume":
		h.audio.Resume()
		h.audio.ResetUserPause()
		state = "playing"
	case "stop":
		h.audio.Stop()
		state = "stopped"
	case "skip":
		h.audio.Stop()
		h.audio.ResetUserPause()
		h.narrator.SkipCooldown()
		state = "skipped"
	case "replay":
		if !h.narrator.ReplayLast(r.Context()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "No previous narration to replay",
			}); err != nil {
				slog.Error("Failed to encode response", "error", err)
			}
			return
		}
		state = "replaying"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	slog.Debug("Audio control", "action", req.Action, "state", state)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"state":  state,
	}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// HandleVolume handles POST /api/audio/volume
func (h *AudioHandler) HandleVolume(w http.ResponseWriter, r *http.Request) {
	var req AudioVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	h.audio.SetVolume(req.Volume)

	// Persist volume
	if h.store != nil {
		strVal := fmt.Sprintf("%.2f", req.Volume)
		if err := h.store.SetState(r.Context(), "volume", strVal); err != nil {
			slog.Error("Failed to persist volume", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"volume": h.audio.Volume(),
	}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// HandleStatus handles GET /api/audio/status
func (h *AudioHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	resp := AudioStatusResponse{
		IsPlaying:    h.audio.IsPlaying(),
		IsPaused:     h.audio.IsPaused(),
		IsUserPaused: h.audio.IsUserPaused(),
		Volume:       h.audio.Volume(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}
