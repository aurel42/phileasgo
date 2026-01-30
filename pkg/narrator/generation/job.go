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

	// Callback handles the generated narrative.
	// If nil, the dispatcher must decide on a default (e.g. playback).
	OnComplete func(*model.Narrative)
}
