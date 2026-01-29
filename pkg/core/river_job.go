package core

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/sim"
)

// RiverJob detects nearby rivers and hydrates POIs periodically.
type RiverJob struct {
	BaseJob
	lastTime  time.Time
	threshold time.Duration
	manager   *poi.Manager
	logger    *slog.Logger
}

// NewRiverJob creates a new RiverJob with a 15s interval.
func NewRiverJob(manager *poi.Manager) *RiverJob {
	return &RiverJob{
		BaseJob:   NewBaseJob("RiverJob"),
		threshold: 15 * time.Second,
		manager:   manager,
		logger:    slog.With("component", "river_job"),
	}
}

func (j *RiverJob) ShouldFire(t *sim.Telemetry) bool {
	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}

	// Only fire when airborne
	if t.IsOnGround {
		return false
	}

	// Time-based interval
	if j.lastTime.IsZero() {
		return true
	}

	return time.Since(j.lastTime) >= j.threshold
}

func (j *RiverJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	j.lastTime = time.Now()

	// Call Manager.UpdateRivers
	_, err := j.manager.UpdateRivers(ctx, t.Latitude, t.Longitude, t.Heading)
	if err != nil {
		j.logger.Warn("UpdateRivers failed", "error", err)
		return
	}

	// If poi != nil, it means discovery or update happened.
	// Logging is now handled inside Manager.UpdateRivers at appropriate levels (DEBUG for updates, INFO for discovery).
}

// GetLastRiverPOI returns the last detected river POI (for testing/debugging).
func (j *RiverJob) GetLastRiverPOI() *model.POI {
	// This could be extended to cache the last result if needed
	return nil
}
