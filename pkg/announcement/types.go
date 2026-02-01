package announcement

import (
	"context"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
)

type Status string

const (
	StatusIdle       Status = "Idle"       // Waiting for trigger
	StatusGenerating Status = "Generating" // AI is working
	StatusHeld       Status = "Held"       // Narrative ready, waiting for play window
	StatusTriggered  Status = "Triggered"  // Sent to playback queue
	StatusDone       Status = "Done"       // Completed (only for non-repeatable)
)

// Item defines the generic interface for timed flight narrations (previously Announcement).
type Item interface {
	ID() string
	Type() model.NarrativeType
	IsRepeatable() bool
	Status() Status
	SetStatus(s Status)

	// Decision Logic
	ShouldGenerate(t *sim.Telemetry) bool
	ShouldPlay(t *sim.Telemetry) bool

	// Content
	GetPromptData(t *sim.Telemetry) (any, error)

	// UI Metadata
	Title() string
	Summary() string
	ImagePath() string
	POI() *model.POI

	// Pipeline Data (Managed by the infrastructure)
	GetHeldNarrative() *model.Narrative
	SetHeldNarrative(n *model.Narrative)

	// Reset state (for session resets/teleports)
	Reset()
}

// EventRecorder defines the interface for logging trip events.
type EventRecorder interface {
	AddEvent(event *model.TripEvent)
}

// DataProvider defines the infrastructure services required by announcements.
type DataProvider interface {
	// Basic Context
	GetLocation(lat, lon float64) model.LocationInfo

	// Proximity & Knowledge
	GetPOIsNear(lat, lon, radius float64) []*model.POI
	GetRepeatTTL() time.Duration
	GetTripSummary() string
	GetLastTransition(stage string) time.Time

	// Prompt Data Assembly
	AssemblePOI(ctx context.Context, p *model.POI, t *sim.Telemetry, strategy string) prompt.Data
	AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data
}
