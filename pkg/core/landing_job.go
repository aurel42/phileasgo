package core

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/sim"
)

// LandingJob detects when the aircraft lands and triggers a debrief.
type LandingJob struct {
	BaseJob
	debriefer   Debriefer
	wasAirborne bool
	cooldown    time.Time
}

// NewLandingJob creates a new LandingJob.
func NewLandingJob(debriefer Debriefer) *LandingJob {
	return &LandingJob{
		BaseJob:   NewBaseJob("LandingJob"),
		debriefer: debriefer,
	}
}

func (j *LandingJob) ShouldFire(t *sim.Telemetry) bool {
	// If recently fired, wait
	if time.Now().Before(j.cooldown) {
		return false
	}

	// State machine:
	// We need to have been airborne (!OnGround) significantly
	// And now we are OnGround and slow (< 15 kts)

	if !t.IsOnGround {
		// Mark as airborne. We don't require a specific altitude, just "not on ground".
		// To avoid "bouncing" on takeoff roll, maybe wait?
		// But IsOnGround from SimConnect covers the wheels state.
		if !j.wasAirborne {
			slog.Debug("LandingJob: Aircraft is airborne")
			j.wasAirborne = true
		}
		return false
	}

	// We are ON GROUND.
	// Were we airborne before?
	if !j.wasAirborne {
		return false // Just taxiing around or started on ground
	}

	// We landed. Are we stopped/slow?
	if t.GroundSpeed > 15.0 {
		return false // rolling out
	}

	// Landed and Slow. Fire!
	return true
}

func (j *LandingJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	slog.Info("LandingJob: Landing detected! Triggering debrief.")

	if j.debriefer.PlayDebrief(ctx, t) {
		// Reset state only if successfully triggered (or busy but acknowledged)
		j.wasAirborne = false
		j.cooldown = time.Now().Add(5 * time.Minute) // Prevent double trigger
	} else {
		// If failed/skipped, maybe retry? For now, we reset to avoid spam loop if it's permanently failing
		// But if it returned false because "Busy", we might want to retry.
		// For PlayDebrief, false usually adds to queue or means "disabled".
		j.wasAirborne = false
	}
}
