package core

import (
	"context"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"testing"
)

type mockBorderNarrator struct {
	lastFrom string
	lastTo   string
	calls    int
}

func (m *mockBorderNarrator) PlayBorder(ctx context.Context, from, to string, tel *sim.Telemetry) bool {
	m.lastFrom = from
	m.lastTo = to
	m.calls++
	return true
}

type mockBorderGeo struct {
	loc model.LocationInfo
}

func (m *mockBorderGeo) GetLocation(lat, lon float64) model.LocationInfo {
	return m.loc
}

func TestBorderJob_InternationalWatersTranslation(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(narrator, geo)

	// 1. Land to Sea
	job.lastLocation = model.LocationInfo{CountryCode: "FR", CityName: "Paris"}
	geo.loc = model.LocationInfo{CountryCode: "XZ", CityName: "International Waters"}

	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.lastFrom != "FR" {
		t.Errorf("Expected from 'FR', got '%s'", narrator.lastFrom)
	}
	if narrator.lastTo != "International Waters" {
		t.Errorf("Expected to 'International Waters', got '%s'", narrator.lastTo)
	}

	// 2. Sea to Land
	job.lastLocation = model.LocationInfo{CountryCode: "XZ", CityName: "International Waters"}
	geo.loc = model.LocationInfo{CountryCode: "UK", CityName: "London"}

	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.lastFrom != "International Waters" {
		t.Errorf("Expected from 'International Waters', got '%s'", narrator.lastFrom)
	}
	if narrator.lastTo != "UK" {
		t.Errorf("Expected to 'UK', got '%s'", narrator.lastTo)
	}
}

func TestBorderJob_Admin1Change(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(narrator, geo)

	// 1. Same Country, Different State
	job.lastLocation = model.LocationInfo{CountryCode: "US", Admin1Name: "California", CityName: "SF"}
	geo.loc = model.LocationInfo{CountryCode: "US", Admin1Name: "Nevada", CityName: "Reno"}

	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.lastFrom != "California" {
		t.Errorf("Expected from 'California', got '%s'", narrator.lastFrom)
	}
	if narrator.lastTo != "Nevada" {
		t.Errorf("Expected to 'Nevada', got '%s'", narrator.lastTo)
	}
}

func TestBorderJob_Concurrency(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(narrator, geo)

	// Lock the job manually
	if !job.TryLock() {
		t.Fatal("Failed to lock job")
	}

	// Try running
	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.calls != 0 {
		t.Error("Job should not have run while locked")
	}

	job.Unlock()

	// Now it should run
	job.lastLocation = model.LocationInfo{CountryCode: "A", CityName: "A"}
	geo.loc = model.LocationInfo{CountryCode: "B", CityName: "B"}
	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.calls != 1 {
		t.Errorf("Expected 1 call after unlock, got %d", narrator.calls)
	}
}
