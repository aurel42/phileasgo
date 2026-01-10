package core

import (
	"context"
)

// SessionResettable is an interface for components that maintain session-specific state
// (e.g. narrated POIs, trip summaries, caches) and need to be reset when the
// aircraft "teleports" (starts a new flight).
type SessionResettable interface {
	ResetSession(ctx context.Context)
}
