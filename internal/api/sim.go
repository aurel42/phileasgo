package api

import (
	"encoding/json"
	"net/http"
	"phileasgo/pkg/sim"
)

type SimCommandHandler struct {
	simClient sim.Client
}

func NewSimCommandHandler(simClient sim.Client) *SimCommandHandler {
	if simClient == nil {
		return nil
	}
	return &SimCommandHandler{simClient: simClient}
}

type SimCommandRequest struct {
	Command string         `json:"command"`
	Args    map[string]any `json:"args"`
}

func (h *SimCommandHandler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	var req SimCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.simClient.ExecuteCommand(r.Context(), req.Command, req.Args); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
