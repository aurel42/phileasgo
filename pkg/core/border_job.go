package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// BorderJob checks for border crossings (country/region change) every 15s.
type BorderJob struct {
	BaseJob
	narrator             Borderrer
	geo                  LocationProvider
	cfg                  *config.Config
	lastLocation         model.LocationInfo
	cooldown             time.Duration
	lastCheck            time.Time
	lastAnnouncementTime time.Time
	repeatCooldowns      map[string]time.Time // Key: "from->to", Value: Last trigger time
}

// NewBorderJob creates a new BorderJob.
func NewBorderJob(cfg *config.Config, n Borderrer, g LocationProvider) *BorderJob {
	return &BorderJob{
		BaseJob:         NewBaseJob("BorderJob"),
		narrator:        n,
		geo:             g,
		cfg:             cfg,
		cooldown:        15 * time.Second,
		repeatCooldowns: make(map[string]time.Time),
	}
}

func (j *BorderJob) ShouldFire(t *sim.Telemetry) bool {
	// Periodic check every 15s
	return time.Since(j.lastCheck) >= j.cooldown
}

func (j *BorderJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	j.lastCheck = time.Now()

	// 1. Get current location
	curr := j.geo.GetLocation(t.Latitude, t.Longitude)

	// Detect Change
	if j.lastLocation.CountryCode == "" {
		// Initial setup, no change detected
		slog.Debug("BorderJob: Initialized location", "country", curr.CountryCode, "region", curr.Admin1Name)
		j.lastLocation = curr
		return
	}

	// Check for Country Change
	if curr.CountryCode != j.lastLocation.CountryCode {
		j.trigger(ctx, j.lastLocation.CountryCode, curr.CountryCode, t)
		j.lastLocation = curr
		return
	}

	// Check for State/Region Change (Admin1)
	if curr.Admin1Name != j.lastLocation.Admin1Name {
		from := j.lastLocation.Admin1Name
		to := curr.Admin1Name

		// Suppress region transit if either side is over water (EEZ/Territorial)
		// We only care about Admin1 changes on LAND.
		if curr.Zone == "territorial" || curr.Zone == "eez" {
			slog.Debug("BorderJob: Admin1 change suppressed (over water)", "from", from, "to", to, "zone", curr.Zone)
			// Update lastLocation so we don't re-check or get stuck in a "changed" state
			j.lastLocation = curr
			return
		}

		// Suppress region transit if either side has no city nearby OR if names are blank.
		// This avoids noise when moving between "Unknown" regions in wilderness.
		if curr.CityName == "" || j.lastLocation.CityName == "" || to == "" || from == "" {
			slog.Debug("BorderJob: Region change suppressed (wilderness/no city)", "from", from, "to", to)
			j.lastLocation = curr
			return
		}

		j.trigger(ctx, from, to, t)
		j.lastLocation = curr
		return
	}

	j.lastLocation = curr
}

func (j *BorderJob) trigger(ctx context.Context, from, to string, t *sim.Telemetry) {
	if from == "XZ" {
		from = "International Waters"
	}
	if to == "XZ" {
		to = "International Waters"
	}

	// 1. Check Global Cooldown
	cooldownAny := time.Duration(j.cfg.Narrator.Border.CooldownAny)
	if time.Since(j.lastAnnouncementTime) < cooldownAny {
		slog.Debug("BorderJob: Global cooldown active, suppressing announcement", "from", from, "to", to, "remain", cooldownAny-time.Since(j.lastAnnouncementTime))
		return
	}

	// 2. Check Repeat Cooldown
	pairKey := fmt.Sprintf("%s->%s", from, to)
	cooldownRepeat := time.Duration(j.cfg.Narrator.Border.CooldownRepeat)
	if lastRepeat, ok := j.repeatCooldowns[pairKey]; ok {
		if time.Since(lastRepeat) < cooldownRepeat {
			slog.Debug("BorderJob: Repeat cooldown active for pair, suppressing announcement", "pair", pairKey, "remain", cooldownRepeat-time.Since(lastRepeat))
			return
		}
	}

	slog.Info("BorderJob: Border crossing detected!", "from", from, "to", to, "lat", t.Latitude, "lon", t.Longitude)
	if j.narrator.PlayBorder(ctx, from, to, t) {
		j.lastAnnouncementTime = time.Now()
		j.repeatCooldowns[pairKey] = time.Now()
	}
}

// SessionResettable implementation
func (j *BorderJob) ResetSession(ctx context.Context) {
	j.lastLocation = model.LocationInfo{}
	j.lastCheck = time.Time{}
	j.lastAnnouncementTime = time.Time{}
	j.repeatCooldowns = make(map[string]time.Time)
	slog.Debug("BorderJob: Session reset")
}
