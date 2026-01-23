package core

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// SessionResettable is an interface for components that maintain session-specific state
// (e.g. narrated POIs, trip summaries, caches) and need to be reset when the
// aircraft "teleports" (starts a new flight).
type SessionResettable interface {
	ResetSession(ctx context.Context)
}

// Debriefer interface for triggering the debrief.
type Debriefer interface {
	PlayDebrief(ctx context.Context, tel *sim.Telemetry) bool
}

// Borderrer interface for triggering border announcements.
type Borderrer interface {
	PlayBorder(ctx context.Context, from, to string, tel *sim.Telemetry) bool
}

// LocationProvider interface for reverse geocoding.
type LocationProvider interface {
	GetLocation(lat, lon float64) model.LocationInfo
}
