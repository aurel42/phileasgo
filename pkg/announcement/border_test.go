package announcement

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

type mockBorderGeo struct {
	loc model.LocationInfo
}

func (m *mockBorderGeo) GetLocation(lat, lon float64) model.LocationInfo {
	return m.loc
}

func TestBorder_MaritimeRestrictions(t *testing.T) {
	geo := &mockBorderGeo{}
	dp := &mockDP{}
	b := NewBorder(config.DefaultConfig(), geo, dp, dp)
	// Override cooldown for testing
	b.checkCooldown = 0

	// 1. Initial Setup: Land (FR)
	b.lastLocation = model.LocationInfo{CountryCode: "FR", Admin1Name: "Normandy", Zone: "land"}

	// 2. Move to Territorial (FR) -> Should be ignored (Admin1 change suppressed)
	geo.loc = model.LocationInfo{CountryCode: "FR", Admin1Name: "", Zone: "territorial"}
	if b.ShouldGenerate(&sim.Telemetry{}) {
		t.Error("Expected no generation for territorial waters")
	}
	// Check internal state update
	if b.lastLocation.Zone != "territorial" {
		t.Errorf("Expected lastLocation to be updated to territorial, got %s", b.lastLocation.Zone)
	}

	// 3. Move to International -> Should trigger FR -> International
	geo.loc = model.LocationInfo{CountryCode: "XZ", Admin1Name: "", Zone: "international"}
	if !b.ShouldGenerate(&sim.Telemetry{}) {
		t.Fatal("Expected generation when entering international waters")
	}
	if b.pendingFrom != "FR" {
		t.Errorf("Expected from 'FR', got '%s'", b.pendingFrom)
	}
	if b.pendingTo != "International Waters" {
		t.Errorf("Expected to 'International Waters', got '%s'", b.pendingTo)
	}

	// 4. Move to EEZ (UK) -> Should TRIGGER (Country Change allowed over water)
	b.lastAnnounce = time.Time{} // reset cooldown
	geo.loc = model.LocationInfo{CountryCode: "UK", Admin1Name: "", Zone: "eez"}
	if !b.ShouldGenerate(&sim.Telemetry{}) {
		t.Error("Expected generation when entering EEZ (Country Change)")
	}

	// 5. Move to Land (UK) -> Should NOT trigger (Country same, Admin1 change from empty suppressed)
	geo.loc = model.LocationInfo{CountryCode: "UK", Admin1Name: "Kent", Zone: "land"}
	if b.ShouldGenerate(&sim.Telemetry{}) {
		t.Error("Expected no new generation when hitting land (already in UK)")
	}
}

func TestBorder_Cooldowns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Border.CooldownAny = config.Duration(1 * time.Minute)
	cfg.Narrator.Border.CooldownRepeat = config.Duration(5 * time.Minute)

	geo := &mockBorderGeo{}
	dp := &mockDP{}
	b := NewBorder(cfg, geo, dp, dp)
	b.checkCooldown = 0

	ctx := context.Background()
	_ = ctx

	// 1. First crossing (Global)
	b.lastLocation = model.LocationInfo{CountryCode: "A"}
	geo.loc = model.LocationInfo{CountryCode: "B"}

	if !b.ShouldGenerate(&sim.Telemetry{}) {
		t.Fatal("Expected 1st generation")
	}

	// 2. Immediate crossing elsewhere (Should be suppressed by GLOBAL cooldown)

	geo.loc = model.LocationInfo{CountryCode: "C"}
	if b.ShouldGenerate(&sim.Telemetry{}) {
		t.Fatal("Expected suppression by global cooldown")
	}

	// 3. Wait for global cooldown, then cross back to A (REPETITIVE crossing)
	b.lastAnnounce = time.Now().Add(-2 * time.Minute)
	// We are currently at C (from step 2, lastLocation was updated even if suppressed?
	// Wait, internal logci in ShouldGenerate:
	// If suppressed by cooldown, does it update lastLocation?
	// The code: "Trigger Logic & Cooldowns" is AFTER "Triggered = true".
	// But where is lastLocation updated?
	// Steps 3 & 4 in code: "b.lastLocation = curr".
	// So YES, lastLocation updates even if cooldown suppresses the announcement.

	// So we are at C. Moving to A.
	geo.loc = model.LocationInfo{CountryCode: "A"}

	// However, we want to test REPEAT cooldown A->B.
	// Let's reset locations to control the test better.
	b.lastLocation = model.LocationInfo{CountryCode: "A"}
	geo.loc = model.LocationInfo{CountryCode: "B"}

	// Mock previous trigger time for A->B
	b.repeatCooldowns["A->B"] = time.Now()

	if b.ShouldGenerate(&sim.Telemetry{}) {
		t.Error("Expected suppression by repeat cooldown")
	}
}
