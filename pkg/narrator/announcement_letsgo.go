package narrator

import (
	"phileasgo/pkg/announcement"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

type LetsgooAnnouncement struct {
	*announcement.BaseAnnouncement
	provider *AIService
}

func NewLetsgooAnnouncement(s *AIService) *LetsgooAnnouncement {
	return &LetsgooAnnouncement{
		BaseAnnouncement: announcement.NewBaseAnnouncement("letsgo", model.NarrativeTypeLetsgo, false),
		provider:         s,
	}
}

func (a *LetsgooAnnouncement) ShouldGenerate(t *sim.Telemetry) bool {
	return t.FlightStage == sim.StageTaxi
}

func (a *LetsgooAnnouncement) ShouldPlay(t *sim.Telemetry) bool {
	return t.FlightStage == sim.StageTakeOff
}

func (a *LetsgooAnnouncement) GetPromptData(t *sim.Telemetry) (any, error) {
	pd := a.provider.getCommonPromptData()

	// Aircraft situation
	loc := a.provider.geoSvc.GetLocation(t.Latitude, t.Longitude)

	pd["City"] = loc.CityName
	pd["Region"] = loc.Admin1Name
	pd["Country"] = loc.CountryCode
	pd["Lat"] = t.Latitude
	pd["Lon"] = t.Longitude
	pd["AltitudeAGL"] = t.AltitudeAGL
	pd["GroundSpeed"] = t.GroundSpeed
	pd["FlightStage"] = sim.FormatStage(t.FlightStage)
	pd["FlightStatusSentence"] = generateFlightStatusSentence(t)

	return pd, nil
}
