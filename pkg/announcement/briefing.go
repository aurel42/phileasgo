package announcement

import (
	"context"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"time"
)

type Briefing struct {
	*Base
	provider DataProvider
}

func NewBriefing(provider DataProvider) *Briefing {
	return &Briefing{
		Base:     NewBase("briefing", model.NarrativeType("briefing"), false),
		provider: provider,
	}
}

func (a *Briefing) ShouldGenerate(t *sim.Telemetry) bool {
	// Generate during Parked, Taxi, or Hold if we have an airport nearby
	if t.FlightStage != sim.StageParked && t.FlightStage != sim.StageTaxi && t.FlightStage != sim.StageHold {
		return false
	}

	// Find nearest airport within 5km
	airport := a.findNearestAirport(t)
	return airport != nil
}

func (a *Briefing) ShouldPlay(t *sim.Telemetry) bool {
	return t.FlightStage == sim.StageTaxi || t.FlightStage == sim.StageHold
}

func (a *Briefing) GetPromptData(t *sim.Telemetry) (any, error) {
	airport := a.findNearestAirport(t)
	if airport == nil {
		return nil, nil
	}

	// Determine strategy based on LastPlayed
	strategy := prompt.StrategyMaxSkew
	if !airport.LastPlayed.IsZero() && time.Since(airport.LastPlayed) < a.provider.GetRepeatTTL() {
		strategy = prompt.StrategyMinSkew
	}

	// Use unified data builder for full POI context
	pd := a.provider.AssemblePOI(context.Background(), airport, t, strategy)
	pd["IsBriefing"] = true

	// Set POI and Metadata for UI signaling
	a.SetPOI(airport)
	a.SetUIMetadata("Briefing: "+airport.DisplayName(), airport.Category, airport.ThumbnailURL)

	return pd, nil
}

func (a *Briefing) findNearestAirport(t *sim.Telemetry) *model.POI {
	pois := a.provider.GetPOIsNear(t.Latitude, t.Longitude, 5000)
	var best *model.POI
	minDist := 5001.0

	for _, p := range pois {
		if p.Category == "airport" || p.Category == "aerodrome" {
			dist := geo.Distance(geo.Point{Lat: t.Latitude, Lon: t.Longitude}, geo.Point{Lat: p.Lat, Lon: p.Lon})
			if dist < minDist {
				minDist = dist
				best = p
			}
		}
	}
	return best
}
