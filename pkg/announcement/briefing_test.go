package announcement

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

type mockDP struct {
	GetPOIsNearFunc       func(lat, lon, radius float64) []*model.POI
	GetRepeatTTLFunc      func() time.Duration
	AssemblePOIFunc       func(ctx context.Context, p *model.POI, t *sim.Telemetry, strategy string) prompt.Data
	AssembleGenericFunc   func(ctx context.Context, t *sim.Telemetry) prompt.Data
	GetLocationFunc       func(lat, lon float64) model.LocationInfo
	GetTripSummaryFunc    func() string
	GetLastTransitionFunc func(stage string) time.Time
	NewContextFunc        func() map[string]any
}

func (m *mockDP) GetPOIsNear(lat, lon, radius float64) []*model.POI {
	return m.GetPOIsNearFunc(lat, lon, radius)
}
func (m *mockDP) GetRepeatTTL() time.Duration { return m.GetRepeatTTLFunc() }
func (m *mockDP) AssemblePOI(ctx context.Context, p *model.POI, t *sim.Telemetry, strategy string) prompt.Data {
	return m.AssemblePOIFunc(ctx, p, t, strategy)
}
func (m *mockDP) AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data {
	return m.AssembleGenericFunc(ctx, t)
}
func (m *mockDP) GetLocation(lat, lon float64) model.LocationInfo {
	if m.GetLocationFunc != nil {
		return m.GetLocationFunc(lat, lon)
	}
	return model.LocationInfo{}
}
func (m *mockDP) GetTripSummary() string {
	if m.GetTripSummaryFunc != nil {
		return m.GetTripSummaryFunc()
	}
	return ""
}
func (m *mockDP) GetLastTransition(stage string) time.Time {
	if m.GetLastTransitionFunc != nil {
		return m.GetLastTransitionFunc(stage)
	}
	return time.Time{}
}
func (m *mockDP) NewContext() map[string]any {
	if m.NewContextFunc != nil {
		return m.NewContextFunc()
	}
	return nil
}

func TestBriefing_Triggers(t *testing.T) {
	dp := &mockDP{
		GetPOIsNearFunc: func(lat, lon, radius float64) []*model.POI {
			return []*model.POI{
				{WikidataID: "Q1", Category: "airport", Lat: 10.0, Lon: 20.0},
			}
		},
	}
	a := NewBriefing(dp)

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
	a := NewBriefing(dp)

	tel := &sim.Telemetry{FlightStage: sim.StageParked, Latitude: 10.0, Longitude: 20.0}
	if a.ShouldGenerate(tel) {
		t.Error("ShouldGenerate should be false when no airport nearby")
	}
}
