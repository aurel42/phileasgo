package announcement

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"testing"
)

func TestBriefing_Triggers(t *testing.T) {
	dp := &mockDP{
		GetPOIsNearFunc: func(lat, lon, radius float64) []*model.POI {
			return []*model.POI{
				{WikidataID: "Q1", Category: "airport", Lat: 10.0, Lon: 20.0},
			}
		},
	}
	a := NewBriefing(config.DefaultConfig(), dp, dp)

	tests := []struct {
		name         string
		stage        string
		expectedGen  bool
		expectedPlay bool
	}{
		{"Parked", sim.StageParked, true, false},
		{"Taxi", sim.StageTaxi, true, true},
		{"Hold", sim.StageHold, true, true},
		{"TakeOff", sim.StageTakeOff, false, false},
		{"Climb", sim.StageClimb, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tel := &sim.Telemetry{FlightStage: tt.stage, Latitude: 10.0, Longitude: 20.0}
			if a.ShouldGenerate(tel) != tt.expectedGen {
				t.Errorf("ShouldGenerate for %s: expected %v", tt.stage, tt.expectedGen)
			}
			if a.ShouldPlay(tel) != tt.expectedPlay {
				t.Errorf("ShouldPlay for %s: expected %v", tt.stage, tt.expectedPlay)
			}
		})
	}
}

func TestBriefing_NoAirportNearby(t *testing.T) {
	dp := &mockDP{
		GetPOIsNearFunc: func(lat, lon, radius float64) []*model.POI {
			return []*model.POI{}
		},
	}
	a := NewBriefing(config.DefaultConfig(), dp, dp)

	tel := &sim.Telemetry{FlightStage: sim.StageParked, Latitude: 10.0, Longitude: 20.0}
	if a.ShouldGenerate(tel) {
		t.Error("ShouldGenerate should be false when no airport nearby")
	}
}

func TestBriefing_ShouldPlay(t *testing.T) {
	a := NewBriefing(config.DefaultConfig(), nil, nil)
	if !a.ShouldPlay(&sim.Telemetry{FlightStage: sim.StageTaxi}) {
		t.Error("ShouldPlay should be true in Taxi")
	}
}

func TestBriefing_GetPromptData(t *testing.T) {
	dp := &mockDP{
		AssemblePOIFunc: func(ctx context.Context, p *model.POI, t *sim.Telemetry, s string) prompt.Data {
			return prompt.Data{"Airport": p.NameEn, "Language": "en"}
		},
		GetPOIsNearFunc: func(lat, lon, radius float64) []*model.POI {
			return []*model.POI{{NameEn: "Test Arpt", Category: "airport"}}
		},
	}
	a := NewBriefing(config.DefaultConfig(), dp, dp)
	data, err := a.GetPromptData(&sim.Telemetry{})
	if err != nil {
		t.Fatal(err)
	}
	d := data.(prompt.Data)
	if d["Airport"] != "Test Arpt" {
		t.Errorf("Expected Airport Test Arpt, got %v", d["Airport"])
	}
}
