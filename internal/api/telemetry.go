package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"phileasgo/pkg/sim"
)

// TelemetryResponse is the API response structure.
type TelemetryResponse struct {
	sim.Telemetry
	SimState string `json:"SimState"`
}

type TelemetryHandler struct {
	mu        sync.RWMutex
	telemetry sim.Telemetry
	simState  sim.State
}

func NewTelemetryHandler() *TelemetryHandler {
	return &TelemetryHandler{simState: sim.StateDisconnected}
}

// Update implements core.TelemetrySink.
func (h *TelemetryHandler) Update(t *sim.Telemetry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.telemetry = *t
}

// UpdateState updates the simulator state.
func (h *TelemetryHandler) UpdateState(s sim.State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.simState = s
}

func (h *TelemetryHandler) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	resp := TelemetryResponse{
		Telemetry: h.telemetry,
		SimState:  string(h.simState),
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode telemetry response", "error", err)
	}
}
