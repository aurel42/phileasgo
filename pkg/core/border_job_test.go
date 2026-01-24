package core

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"testing"
	"time"
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

func (m *mockBorderNarrator) ReorderFeatures(lat, lon float64) {
	// no-op
}

type mockBorderGeo struct {
	loc model.LocationInfo
}

func (m *mockBorderGeo) GetLocation(lat, lon float64) model.LocationInfo {
	return m.loc
}

func (m *mockBorderGeo) ReorderFeatures(lat, lon float64) {
	// no-op
}

func TestBorderJob_InternationalWatersTranslation(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(config.DefaultConfig(), narrator, geo)

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
	job.lastAnnouncementTime = time.Time{}
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
	job := NewBorderJob(config.DefaultConfig(), narrator, geo)

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
	job := NewBorderJob(config.DefaultConfig(), narrator, geo)

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

func TestBorderJob_BootstrapWithoutCity(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(config.DefaultConfig(), narrator, geo)

	// 1. Initial location has NO city, but YES country/state
	geo.loc = model.LocationInfo{CountryCode: "DE", Admin1Name: "Bavaria", CityName: ""}
	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.calls != 0 {
		t.Fatal("Should not trigger on first run")
	}
	if job.lastLocation.CountryCode != "DE" {
		t.Fatal("Initial location should be stored")
	}

	// 2. Move to different state STILL with no city
	geo.loc = model.LocationInfo{CountryCode: "DE", Admin1Name: "Hesse", CityName: ""}
	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.calls != 1 {
		t.Errorf("Expected 1 call when state changed, even without city. Got %d", narrator.calls)
	}
	if narrator.lastFrom != "Bavaria" {
		t.Errorf("Expected from 'Bavaria', got '%s'", narrator.lastFrom)
	}
	if narrator.lastTo != "Hesse" {
		t.Errorf("Expected to 'Hesse', got '%s'", narrator.lastTo)
	}
}

func TestBorderJob_EmptyToNamedState(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(config.DefaultConfig(), narrator, geo)

	// 1. Initial location in a country where Admin1 is unknown/empty (e.g. crossing from waters)
	geo.loc = model.LocationInfo{CountryCode: "XZ", Admin1Name: "", CityName: ""}
	job.Run(context.Background(), &sim.Telemetry{})

	// 2. Cross into a named state
	geo.loc = model.LocationInfo{CountryCode: "US", Admin1Name: "New York", CityName: "NYC"}
	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.calls != 1 {
		t.Fatalf("Expected 1 call (country change), got %d", narrator.calls)
	}
	if narrator.lastTo != "US" {
		t.Errorf("Expected to 'US', got '%s'", narrator.lastTo)
	}

	// 3. Cross into a named state in same country (if prior was unknown)
	job.lastAnnouncementTime = time.Time{}
	job.lastLocation = model.LocationInfo{CountryCode: "US", Admin1Name: "", CityName: ""}
	geo.loc = model.LocationInfo{CountryCode: "US", Admin1Name: "New Jersey", CityName: ""}
	job.Run(context.Background(), &sim.Telemetry{})

	if narrator.calls != 2 {
		t.Errorf("Expected 2nd call for state change from empty to 'New Jersey', got %d", narrator.calls)
	}
}

func TestBorderJob_Cooldowns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Border.CooldownAny = config.Duration(1 * time.Minute)
	cfg.Narrator.Border.CooldownRepeat = config.Duration(5 * time.Minute)

	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(cfg, narrator, geo)

	ctx := context.Background()
	tel := &sim.Telemetry{}

	// 1. First crossing (Global)
	job.lastLocation = model.LocationInfo{CountryCode: "A"}
	geo.loc = model.LocationInfo{CountryCode: "B"}
	job.Run(ctx, tel)
	if narrator.calls != 1 {
		t.Fatalf("Expected 1st call, got %d", narrator.calls)
	}

	// 2. Immediate crossing elsewhere (Should be suppressed by GLOBAL cooldown)
	job.lastLocation = geo.loc
	geo.loc = model.LocationInfo{CountryCode: "C"}
	job.Run(ctx, tel)
	if narrator.calls != 1 {
		t.Errorf("Expected 1 call (suppressed by global), got %d", narrator.calls)
	}

	// 3. Wait for global cooldown, then cross back to A (REPETITIVE crossing)
	job.lastAnnouncementTime = time.Now().Add(-2 * time.Minute)
	job.lastLocation = geo.loc
	geo.loc = model.LocationInfo{CountryCode: "B"}
	job.Run(ctx, tel)
	if narrator.calls != 2 {
		t.Errorf("Expected 2 calls (global passed), got %d", narrator.calls)
	}

	// 4. Repeated crossing B -> C (Should be suppressed by REPEAT cooldown)
	job.lastAnnouncementTime = time.Now().Add(-10 * time.Minute) // Bypass global
	job.lastLocation = geo.loc
	geo.loc = model.LocationInfo{CountryCode: "C"}
	// Let's make it actually trigger first.
	job.lastAnnouncementTime = time.Now().Add(-10 * time.Minute) // Long ago
	job.repeatCooldowns = make(map[string]time.Time)             // Clear
	job.lastLocation = model.LocationInfo{CountryCode: "A"}
	geo.loc = model.LocationInfo{CountryCode: "B"}
	job.Run(ctx, tel) // Call 3: A -> B
	if narrator.calls != 3 {
		t.Errorf("Expected 3 calls, got %d", narrator.calls)
	}

	// 5. Cross back B -> A
	job.lastAnnouncementTime = time.Now().Add(-10 * time.Minute)
	job.lastLocation = geo.loc
	geo.loc = model.LocationInfo{CountryCode: "A"}
	job.Run(ctx, tel) // Call 4: B -> A
	if narrator.calls != 4 {
		t.Errorf("Expected 4 calls, got %d", narrator.calls)
	}

	// 6. Immediate repeat A -> B (Suppressed by REPEAT cooldown even if global passes)
	job.lastAnnouncementTime = time.Now().Add(-10 * time.Minute)
	job.lastLocation = geo.loc
	geo.loc = model.LocationInfo{CountryCode: "B"}
	job.Run(ctx, tel)
	if narrator.calls != 4 {
		t.Errorf("Expected 4 calls (suppressed by repeat), got %d", narrator.calls)
	}
}

func TestBorderJob_MaritimeRestrictions(t *testing.T) {
	narrator := &mockBorderNarrator{}
	geo := &mockBorderGeo{}
	job := NewBorderJob(config.DefaultConfig(), narrator, geo)

	// 1. Land (FR) to Territorial (FR) -> Should be ignored
	job.lastLocation = model.LocationInfo{CountryCode: "FR", Admin1Name: "Normandy", Zone: "land"}
	geo.loc = model.LocationInfo{CountryCode: "FR", Admin1Name: "", Zone: "territorial"}
	job.Run(context.Background(), &sim.Telemetry{})
	if narrator.calls != 0 {
		t.Errorf("Expected 0 calls for territorial waters, got %d", narrator.calls)
	}
	if job.lastLocation.Admin1Name != "Normandy" {
		t.Errorf("Expected lastLocation to remain Normandy, got %s", job.lastLocation.Admin1Name)
	}

	// 2. Territorial (FR) to International -> Should trigger FR -> International
	geo.loc = model.LocationInfo{CountryCode: "XZ", Admin1Name: "", Zone: "international"}
	job.Run(context.Background(), &sim.Telemetry{})
	if narrator.calls != 1 {
		t.Fatalf("Expected 1 call when entering international waters, got %d", narrator.calls)
	}
	if narrator.lastFrom != "FR" {
		t.Errorf("Expected from 'FR', got '%s'", narrator.lastFrom)
	}
	if narrator.lastTo != "International Waters" {
		t.Errorf("Expected to 'International Waters', got '%s'", narrator.lastTo)
	}

	// 3. International to EEZ (UK) -> Should be ignored
	job.lastAnnouncementTime = time.Time{} // reset cooldown
	geo.loc = model.LocationInfo{CountryCode: "UK", Admin1Name: "", Zone: "eez"}
	job.Run(context.Background(), &sim.Telemetry{})
	if narrator.calls != 1 {
		t.Errorf("Expected no new calls when entering EEZ, got %d", narrator.calls)
	}

	// 4. EEZ (UK) to Land (UK) -> Should trigger International -> UK
	geo.loc = model.LocationInfo{CountryCode: "UK", Admin1Name: "Kent", Zone: "land"}
	job.Run(context.Background(), &sim.Telemetry{})
	if narrator.calls != 2 {
		t.Errorf("Expected 2nd call when hitting land, got %d", narrator.calls)
	}
	if narrator.lastFrom != "International Waters" {
		t.Errorf("Expected from 'International Waters', got '%s'", narrator.lastFrom)
	}
	if narrator.lastTo != "UK" {
		t.Errorf("Expected to 'UK', got '%s'", narrator.lastTo)
	}
}
