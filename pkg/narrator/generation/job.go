package generation

import (
	"time"

	"phileasgo/pkg/announcement"
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

	// For Announcements (Phases 2 & 3)
	Announcement announcement.Announcement

	// Callback handles the generated narrative.
	// If nil, the dispatcher must decide on a default (e.g. playback).
	OnComplete func(*model.Narrative)
}
