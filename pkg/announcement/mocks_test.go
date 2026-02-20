package announcement

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"time"
)

type mockDP struct {
	events                []model.TripEvent
	GetRepeatTTLFunc      func() time.Duration
	GetLastTransitionFunc func(string) time.Time
	AddEventFunc          func(*model.TripEvent)
	AssemblePOIFunc       func(context.Context, *model.POI, *sim.Telemetry, string) prompt.Data
	AssembleGenericFunc   func(context.Context, *sim.Telemetry) prompt.Data
	GetPOIsNearFunc       func(float64, float64, float64) []*model.POI
	UserPaused            bool
}

func (m *mockDP) GetRepeatTTL() time.Duration {
	if m.GetRepeatTTLFunc != nil {
		return m.GetRepeatTTLFunc()
	}
	return 0
}
func (m *mockDP) GetLastTransition(s string) time.Time {
	if m.GetLastTransitionFunc != nil {
		return m.GetLastTransitionFunc(s)
	}
	return time.Time{}
}
func (m *mockDP) AddEvent(e *model.TripEvent) {
	m.events = append(m.events, *e)
	if m.AddEventFunc != nil {
		m.AddEventFunc(e)
	}
}
func (m *mockDP) AssemblePOI(ctx context.Context, p *model.POI, t *sim.Telemetry, s string) prompt.Data {
	if m.AssemblePOIFunc != nil {
		return m.AssemblePOIFunc(ctx, p, t, s)
	}
	return nil
}
func (m *mockDP) AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data {
	if m.AssembleGenericFunc != nil {
		return m.AssembleGenericFunc(ctx, t)
	}
	return nil
}
func (m *mockDP) GetPOIsNear(lat, lon, radius float64) []*model.POI {
	if m.GetPOIsNearFunc != nil {
		return m.GetPOIsNearFunc(lat, lon, radius)
	}
	return nil
}

func (m *mockDP) GetLocation(lat, lon float64) model.LocationInfo {
	return model.LocationInfo{}
}

func (m *mockDP) IsUserPaused() bool {
	return m.UserPaused
}
