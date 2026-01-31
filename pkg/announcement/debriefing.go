package announcement

import (
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"time"
)

type Debriefing struct {
	*Base
	dp DataProvider
}

func NewDebriefing(dp DataProvider) *Debriefing {
	return &Debriefing{
		Base: NewBase("debriefing", model.NarrativeTypeDebrief, false),
		dp:   dp,
	}
}

func (a *Debriefing) Title() string {
	return "Flight Debriefing"
}

// ShouldGenerate returns true if we are on the ground (landed, taxi, hold)
// AND we have been airborne for at least 5 minutes in this session.
func (a *Debriefing) ShouldGenerate(t *sim.Telemetry) bool {
	if a.Status() != StatusIdle {
		return false
	}

	// 1. Check if we are in a "post-flight" stage
	isLanded := t.FlightStage == sim.StageLanded ||
		t.FlightStage == sim.StageTaxi ||
		t.FlightStage == sim.StageHold

	if !isLanded {
		return false
	}

	// 2. Check if we actually flew (TakeOff or Climb started > 5 mins ago)
	takeOffTime := a.dp.GetLastTransition(sim.StageTakeOff)
	climbTime := a.dp.GetLastTransition(sim.StageClimb)

	var startTime time.Time
	if !takeOffTime.IsZero() {
		startTime = takeOffTime
	} else if !climbTime.IsZero() {
		startTime = climbTime
	}

	if startTime.IsZero() {
		return false
	}

	// Must have been at least 5 minutes since we started flying
	if time.Since(startTime) < 5*time.Minute {
		return false
	}

	// 3. Must have a decent trip summary (at least 50 chars)
	return len(a.dp.GetTripSummary()) >= 50
}

// ShouldPlay returns true once we are settled (Taxi or Hold)
// We don't want to play right during the high-workload Landed stage.
func (a *Debriefing) ShouldPlay(t *sim.Telemetry) bool {
	return t.FlightStage == sim.StageTaxi || t.FlightStage == sim.StageHold
}

func (a *Debriefing) GetPromptData(t *sim.Telemetry) (any, error) {
	return struct {
		TripSummary string
	}{
		TripSummary: a.dp.GetTripSummary(),
	}, nil
}
