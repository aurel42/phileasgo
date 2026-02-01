package core

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
)

// SessionPersistenceJob manages the periodic saving of session state.
type SessionPersistenceJob struct {
	st      store.Store
	sessMgr *session.Manager
	sim     sim.Client

	lastSavedState []byte
}

// NewSessionPersistenceJob creates a new persistence job.
func NewSessionPersistenceJob(st store.Store, sm *session.Manager, s sim.Client) *SessionPersistenceJob {
	return &SessionPersistenceJob{
		st:      st,
		sessMgr: sm,
		sim:     s,
	}
}

// Start begins the persistence loop.
func (j *SessionPersistenceJob) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)

	slog.Info("Persistence: Session persistence loop started")

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.checkAndSave(ctx)
			}
		}
	}()
}

func (j *SessionPersistenceJob) checkAndSave(ctx context.Context) {
	// 1. Update Session with latest Stage Data
	stageState := j.sim.GetStageState()
	j.sessMgr.SetStageData(stageState)

	// 2. Get Telemetry for location
	tel, err := j.sim.GetTelemetry(ctx)
	if err != nil {
		// Can't save without location context
		return
	}

	// 3. Serialize
	data, err := j.sessMgr.GetPersistentState(tel.Latitude, tel.Longitude)
	if err != nil {
		slog.Error("Persistence: Failed to serialize session state", "error", err)
		return
	}

	// 4. Dirty Check
	if bytes.Equal(data, j.lastSavedState) {
		return // No change
	}

	// 5. Save
	if err := j.st.SetState(ctx, "session_context", string(data)); err != nil {
		slog.Error("Persistence: Failed to save session state", "error", err)
	} else {
		j.lastSavedState = data
		slog.Debug("Persistence: Session saved", "size", len(data))
	}
}
