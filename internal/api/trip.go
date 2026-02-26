package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"phileasgo/pkg/model"
	"phileasgo/pkg/store"
)

// SessionProvider provides access to session state.
type SessionProvider interface {
	GetEvents() []model.TripEvent
}

// persistedSession mirrors the session.PersistentState for JSON unmarshalling.
type persistedSession struct {
	Events []model.TripEvent `json:"events"`
}

// TripHandler handles trip-related API endpoints.
type TripHandler struct {
	session SessionProvider
	store   store.Store
}

// NewTripHandler creates a new TripHandler. Returns nil if dependencies are missing.
func NewTripHandler(session SessionProvider, st store.Store) *TripHandler {
	if session == nil || st == nil {
		return nil
	}
	return &TripHandler{session: session, store: st}
}

// HandleEvents returns the trip events as JSON.
// It first checks the in-memory session, then falls back to the persisted session_context.
// GET /api/trip/events
func (h *TripHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	// First, try in-memory session (active flight)
	events := h.session.GetEvents()

	// If empty, try loading from persisted session_context (for replay mode)
	if len(events) == 0 && h.store != nil {
		if val, found := h.store.GetState(r.Context(), "session_context"); found && val != "" {
			var ps persistedSession
			if err := json.Unmarshal([]byte(val), &ps); err != nil {
				slog.Warn("TripHandler: failed to unmarshal persisted session", "error", err)
			} else {
				events = ps.Events
			}
		}
	}

	if events == nil {
		events = []model.TripEvent{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		slog.Error("Failed to encode trip events", "error", err)
	}
}
