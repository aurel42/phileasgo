package announcement

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"time"
)

type Debriefing struct {
	*Base
	cfg *config.Config
	dp  DataProvider
}

func NewDebriefing(cfg *config.Config, dp DataProvider, events EventRecorder) *Debriefing {
	return &Debriefing{
		Base: NewBase("debriefing", model.NarrativeTypeDebriefing, false, dp, events),
		cfg:  cfg,
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

	// 3. No longer check for trip summary length (per user request)
	return true
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
