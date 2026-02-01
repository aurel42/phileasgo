package session

import (
	"context"
	"encoding/json"
	"log/slog"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
)

// persistentState is a local mirror of Manager's private PersistentState for unmarshalling.
// We only need Lat/Lon for the distance check.
type persistentState struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// TryRestore attempts to restore a previous session if airborne and within range.
// It returns true if restoration was attempted (success or failure), false if conditions were not met.
func TryRestore(ctx context.Context, st store.Store, mgr *Manager, tel *sim.Telemetry) bool {
	// 1. If we are on the ground, we assume a fresh flight start.
	// We do NOT restore previous session. We return true to mark the job as done.
	if tel.IsOnGround {
		slog.Info("Session: Aircraft is on the ground. Starting fresh session.")
		return true
	}

	val, found := st.GetState(ctx, "session_context")
	if !found || val == "" {
		return true // Nothing to restore, but we "checked"
	}

	// Check distance
	var ps persistentState
	if err := json.Unmarshal([]byte(val), &ps); err != nil {
		slog.Error("Session: Failed to unmarshal persistent session location", "error", err)
		return true
	}

	dist := geo.Distance(geo.Point{Lat: ps.Lat, Lon: ps.Lon}, geo.Point{Lat: tel.Latitude, Lon: tel.Longitude})
	// 50 nautical miles ~= 92.6 km => 92600 meters
	if dist > 92600 {
		slog.Info("Session: Persistent session too far away, ignoring", "dist_m", dist)
		return true
	}

	if err := mgr.Restore([]byte(val)); err != nil {
		slog.Error("Session: Failed to restore persisted session state", "error", err)
	} else {
		slog.Info("Session: Successfully restored persisted session state", "dist_m", dist)
	}

	return true
}
