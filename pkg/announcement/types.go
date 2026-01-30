package announcement

import (
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

type Status string

const (
	StatusIdle       Status = "Idle"       // Waiting for trigger
	StatusGenerating Status = "Generating" // AI is working
	StatusHeld       Status = "Held"       // Narrative ready, waiting for play window
	StatusTriggered  Status = "Triggered"  // Sent to playback queue
	StatusDone       Status = "Done"       // Completed (only for non-repeatable)
	StatusMissed     Status = "Missed"     // Play window passed before generation
)

// Announcement defines the generic interface for timed flight narrations.
type Announcement interface {
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

	// Pipeline Data (Managed by the infrastructure)
	GetHeldNarrative() *model.Narrative
	SetHeldNarrative(n *model.Narrative)

	// Reset state (for session resets/teleports)
	Reset()
}
