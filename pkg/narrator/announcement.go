package narrator

import (
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

type AnnouncementStatus string

const (
	StatusIdle       AnnouncementStatus = "idle"
	StatusGenerating AnnouncementStatus = "generating"
	StatusHeld       AnnouncementStatus = "held"
	StatusTriggered  AnnouncementStatus = "triggered"
	StatusDone       AnnouncementStatus = "done"
)

// Announcement defines the generic interface for timed flight narrations.
type Announcement interface {
	ID() string
	Type() model.NarrativeType
	Status() AnnouncementStatus
	SetStatus(s AnnouncementStatus)

	// Decision Logic
	ShouldGenerate(t *sim.Telemetry) bool
	ShouldPlay(t *sim.Telemetry) bool

	// Content
	GetPromptData(t *sim.Telemetry) (any, error)
	GetHeldNarrative() *model.Narrative
	SetHeldNarrative(n *model.Narrative)
}
