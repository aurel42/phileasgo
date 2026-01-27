package generation

import (
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// Job represents a queued request (priority queue).
type Job struct {
	Type      model.NarrativeType
	POIID     string
	ImagePath string
	Manual    bool
	Strategy  string // e.g., "funny", "historic"
	CreatedAt time.Time
	Telemetry *sim.Telemetry

	// For Border Crossings
	From string
	To   string
}
