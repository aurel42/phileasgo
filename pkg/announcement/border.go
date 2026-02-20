package announcement

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
)

// LocationProvider interface to avoid core dependency
type LocationProvider interface {
	GetLocation(lat, lon float64) model.LocationInfo
}

type Border struct {
	*Base
	geo      LocationProvider
	provider DataProvider
	cfg      *config.Config

	lastLocation    model.LocationInfo
	lastCheck       time.Time
	lastAnnounce    time.Time
	repeatCooldowns map[string]time.Time
	checkCooldown   time.Duration

	// Transient state for the current generation
	pendingFrom string
	pendingTo   string
}

func NewBorder(cfg *config.Config, geo LocationProvider, dp DataProvider, events EventRecorder) *Border {
	b := &Border{
		Base:            NewBase("border", model.NarrativeTypeBorder, true, dp, events), // BY DESIGN: repeatable: true
		geo:             geo,
		provider:        dp,
		cfg:             cfg,
		checkCooldown:   10 * time.Second, // Check every 10s (similar to old 15s)
		repeatCooldowns: make(map[string]time.Time),
	}
	b.SetUIMetadata("Border Crossing", "", "")
	return b
}

func (b *Border) ShouldGenerate(t *sim.Telemetry) bool {
	// 1. Throttle checks
	if time.Since(b.lastCheck) < b.checkCooldown {
		return false
	}
	b.lastCheck = time.Now()

	// 2. Get Location
	curr := b.geo.GetLocation(t.Latitude, t.Longitude)

	// Refinement: Skip logic if initializing
	if b.lastLocation.CountryCode == "" {
		slog.Debug("Border: Initialized location", "country", curr.CountryCode, "region", curr.Admin1Name)
		b.lastLocation = curr
		return false
	}

	from, to, triggered := b.checkCrossing(&curr)

	if !triggered {
		b.lastLocation = curr
		return false
	}

	// 5. Trigger Logic & Cooldowns
	if from == "XZ" {
		from = "International Waters"
	}
	if to == "XZ" {
		to = "International Waters"
	}

	if b.isCooldownActive(from, to) {
		return false
	}

	// Success!
	slog.Info("Border: Crossing detected", "from", from, "to", to)
	b.pendingFrom = from
	b.pendingTo = to
	b.lastAnnounce = time.Now()
	b.repeatCooldowns[fmt.Sprintf("%s->%s", from, to)] = time.Now()

	// Direct log to event history (Phase 3)
	if b.Events != nil {
		b.Events.AddEvent(&model.TripEvent{
			Timestamp: time.Now(),
			Type:      "activity",
			Title:     "Border Crossing",
			Summary:   fmt.Sprintf("Moved from %s to %s", from, to),
		})
	}

	// [NEW] If user is paused, we ONLY log. We don't queue or create a script/audio.
	if b.provider.IsUserPaused() {
		slog.Debug("Border: Skipping narrative generation (User Paused)", "from", from, "to", to)
		b.lastLocation = curr // Mark as processed
		b.Reset()             // Clear any internal state
		return false
	}

	b.lastLocation = curr // Update location only after success or handled logic

	// Reset Base state to ensure we can generate fresh
	b.Reset()

	return true
}

func (b *Border) checkCrossing(curr *model.LocationInfo) (from, to string, triggered bool) {
	// 3. Check Country Change
	if curr.CountryCode != b.lastLocation.CountryCode {
		return b.lastLocation.CountryCode, curr.CountryCode, true
	}

	// 4. Check Region Change
	if curr.Admin1Name != b.lastLocation.Admin1Name {
		from = b.lastLocation.Admin1Name
		to = curr.Admin1Name

		// Suppress region transit if either side is over water
		if curr.Zone == "territorial" || curr.Zone == "eez" {
			slog.Debug("Border: Admin1 change suppressed (over water)", "from", from, "to", to, "zone", curr.Zone)
			return "", "", false
		}

		// Suppress region transit if no city nearby or blank names
		if curr.CityName == "" || b.lastLocation.CityName == "" || to == "" || from == "" {
			slog.Debug("Border: Region change suppressed (wilderness/no city)", "from", from, "to", to)
			return "", "", false
		}

		return from, to, true
	}

	return "", "", false
}

func (b *Border) isCooldownActive(from, to string) bool {
	// Global Cooldown
	cooldownAny := time.Duration(b.cfg.Narrator.Border.CooldownAny)
	if time.Since(b.lastAnnounce) < cooldownAny {
		slog.Debug("Border: Global cooldown active", "remain", cooldownAny-time.Since(b.lastAnnounce))
		return true
	}

	// Repeat Cooldown
	pairKey := fmt.Sprintf("%s->%s", from, to)
	cooldownRepeat := time.Duration(b.cfg.Narrator.Border.CooldownRepeat)
	if lastRepeat, ok := b.repeatCooldowns[pairKey]; ok {
		if time.Since(lastRepeat) < cooldownRepeat {
			slog.Debug("Border: Repeat cooldown active", "pair", pairKey, "remain", cooldownRepeat-time.Since(lastRepeat))
			return true
		}
	}
	return false
}

func (b *Border) GetPromptData(t *sim.Telemetry) (any, error) {
	// Use the generic assembler provided by infra
	pd := b.provider.AssembleGeneric(context.Background(), t)
	if pd == nil {
		pd = make(prompt.Data)
	}

	pd["From"] = b.pendingFrom
	pd["To"] = b.pendingTo
	pd["Type"] = "border"
	pd["MaxWords"] = 30 // Narrative should be concise

	return pd, nil
}

func (b *Border) ShouldPlay(t *sim.Telemetry) bool {
	return true
}

func (b *Border) ResetSession(ctx context.Context) {
	b.Base.Reset()
	b.lastLocation = model.LocationInfo{}
	b.lastCheck = time.Time{}
	b.lastAnnounce = time.Time{}
	b.repeatCooldowns = make(map[string]time.Time)
}
