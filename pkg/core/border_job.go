package core

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// BorderJob checks for border crossings (country/region change) every 15s.
type BorderJob struct {
	BaseJob
	narrator     Borderrer
	geo          LocationProvider
	lastLocation model.LocationInfo
	cooldown     time.Duration
	lastCheck    time.Time
}

// NewBorderJob creates a new BorderJob.
func NewBorderJob(n Borderrer, g LocationProvider) *BorderJob {
	return &BorderJob{
		BaseJob:  NewBaseJob("BorderJob"),
		narrator: n,
		geo:      g,
		cooldown: 15 * time.Second,
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
	// Trigger if the name actually changed. Note: we allow one to be empty to handle
	// transitions between labeled states and unlabeled regions/waters.
	if curr.Admin1Name != j.lastLocation.Admin1Name {
		from := j.lastLocation.Admin1Name
		to := curr.Admin1Name

		// Map empty names to something readable for the narrator if needed,
		// but typically the narrator context handles names.
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
	slog.Info("BorderJob: Border crossing detected!", "from", from, "to", to)
	j.narrator.PlayBorder(ctx, from, to, t)
}

// SessionResettable implementation
func (j *BorderJob) ResetSession(ctx context.Context) {
	j.lastLocation = model.LocationInfo{}
	j.lastCheck = time.Time{}
	slog.Debug("BorderJob: Session reset")
}
