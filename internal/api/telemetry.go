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
	SimState       string  `json:"SimState"`
	ValleyAltitude float64 `json:"ValleyAltitude,omitempty"`
	Valid          bool    `json:"Valid"`
}

type TelemetryHandler struct {
	mu             sync.RWMutex
	telemetry      sim.Telemetry
	simState       sim.State
	valleyAltitude float64
	hasReceived    bool
}

func NewTelemetryHandler() *TelemetryHandler {
	return &TelemetryHandler{simState: sim.StateDisconnected}
}

// Update implements core.TelemetrySink.
func (h *TelemetryHandler) Update(t *sim.Telemetry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.telemetry = *t
	h.hasReceived = true
}

// UpdateState updates the simulator state.
func (h *TelemetryHandler) UpdateState(s sim.State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.simState = s
	// If disconnected, we might want to invalidate?
	// But user says: last known good data.
	// However, if we disconnect, usually specific UI logic takes over.
	// Let's leave hasReceived true if we had data, so we can show "Last known" if we wanted.
}

// SetValleyAltitude updates the cached valley altitude.
func (h *TelemetryHandler) SetValleyAltitude(altMeters float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.valleyAltitude = altMeters
}

func (h *TelemetryHandler) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	resp := TelemetryResponse{
		Telemetry:      h.telemetry,
		SimState:       string(h.simState),
		ValleyAltitude: h.valleyAltitude,
		Valid:          h.hasReceived,
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode telemetry response", "error", err)
	}
}
