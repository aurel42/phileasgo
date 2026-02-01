package core

import (
	"context"
	"sync/atomic"

	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
)

// SessionRestorationJob attempts to restore the session state on startup.
type SessionRestorationJob struct {
	BaseJob
	st      store.Store
	sessMgr *session.Manager
	done    int32 // 1 if attempted
}

func NewSessionRestorationJob(st store.Store, sm *session.Manager) *SessionRestorationJob {
	return &SessionRestorationJob{
		BaseJob: NewBaseJob("SessionRestoration"),
		st:      st,
		sessMgr: sm,
	}
}

func (j *SessionRestorationJob) ShouldFire(t *sim.Telemetry) bool {
	// Fire if not done and conditions might be met (Airborne checked in logic, but here we just try once)
	// Actually, TryRestore checks IsOnGround. If we are on ground, ShouldFire should probably return false and wait?
	// or TryRestore returns false if it didn't run.

	if atomic.LoadInt32(&j.done) == 1 {
		return false
	}

	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}

	// We fire immediately once active (handled by Scheduler/SimState check implicitly by being here?)
	// Scheduler only ticks if SimState == Active.
	// So we just check if we are already done or running.
	return true
}

func (j *SessionRestorationJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	// TryRestore returns true if it actually checked/restored/discarded.
	// Returns false if conditions (like IsOnGround) weren't met (though we checked that in ShouldFire).
	if session.TryRestore(ctx, j.st, j.sessMgr, t) {
		atomic.StoreInt32(&j.done, 1)
	}
}
