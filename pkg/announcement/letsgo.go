package announcement

import (
	"context"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
)

type Letsgo struct {
	*Base
	cfg      *config.Config
	provider DataProvider
}

func NewLetsgo(cfg *config.Config, dp DataProvider, events EventRecorder) *Letsgo {
	return &Letsgo{
		Base:     NewBase("letsgo", model.NarrativeTypeLetsgo, false, dp, events),
		cfg:      cfg,
		provider: dp,
	}
}

func (a *Letsgo) ShouldGenerate(t *sim.Telemetry) bool {
	// Trigger when we move from ground to air (StageClimb or StageTakeOff Confirm/Airborne)
	// Or simply if we were on ground and now GroundSpeed > 40 and we are confirm airborne
	matchesState := (t.FlightStage == sim.StageTakeOff || t.FlightStage == sim.StageClimb) && t.GroundSpeed > 40
	if !matchesState {
		return false
	}

	// Suppression: If we have been flying for > 5 minutes, assume this is a session restore
	// and we shouldn't welcome the pilot again.
	takeOffTime := a.provider.GetLastTransition(sim.StageTakeOff)
	if !takeOffTime.IsZero() && time.Since(takeOffTime) > 5*time.Minute {
		// Log debug if strictly necessary, or just return false
		// slog.Debug("LetsGo: Suppressed due to flight duration", "duration", time.Since(takeOffTime))
		return false
	}

	return true
}

func (a *Letsgo) ShouldPlay(t *sim.Telemetry) bool {
	// Play once we are fairly stable in climb
	return t.FlightStage == sim.StageClimb && t.AltitudeAGL > 500
}

func (a *Letsgo) GetPromptData(t *sim.Telemetry) (any, error) {
	// Use the generic assembler provided by infra
	pd := a.provider.AssembleGeneric(context.Background(), t)

	// Aircraft situation
	loc := a.provider.GetLocation(t.Latitude, t.Longitude)

	pd["City"] = loc.CityName
	pd["Region"] = loc.Admin1Name
	pd["Country"] = loc.CountryCode
	pd["Lat"] = t.Latitude
	pd["Lon"] = t.Longitude
	pd["AltitudeAGL"] = t.AltitudeAGL
	pd["GroundSpeed"] = t.GroundSpeed
	pd["FlightStage"] = sim.FormatStage(t.FlightStage)
	pd["FlightStatusSentence"] = prompt.GenerateFlightStatusSentence(t)

	return pd, nil
}
