package core

import (
	"context"
	"log/slog"
	"phileasgo/pkg/config"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/visibility"
	"strconv"
)

// TransponderWatcherJob monitors the aircraft transponder for squawk and ident changes.
// It maps squawk codes starting with 7 into configuration/visibility boosts.
// It also triggers the configured action on IDENT button press.
type TransponderWatcherJob struct {
	BaseJob
	cfg        config.Provider
	narrator   narrator.Service
	st         store.Store
	vis        *visibility.Calculator
	lastSquawk int
	lastIdent  bool
}

// NewTransponderWatcherJob creates a new transponder watcher job.
func NewTransponderWatcherJob(cfg config.Provider, n narrator.Service, st store.Store, vis *visibility.Calculator) *TransponderWatcherJob {
	return &TransponderWatcherJob{
		BaseJob:   NewBaseJob("TransponderWatcherJob"),
		cfg:       cfg,
		narrator:  n,
		st:        st,
		vis:       vis,
		lastIdent: false, // Default: Assume off at start
	}
}

// ShouldFire returns true if transponder data has changed.
func (j *TransponderWatcherJob) ShouldFire(t *sim.Telemetry) bool {
	// We always fire to check for transitions, but the Run method will handle logic.
	// Actually, we can optimize to only fire on change.
	return t.Squawk != j.lastSquawk || t.Ident != j.lastIdent
}

// Run executes the transponder logic.
func (j *TransponderWatcherJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	// 1. Check Squawk for 7XXX prefix (where XXX are 1-5)
	if isControlSquawk(t.Squawk) {
		if t.Squawk != j.lastSquawk {
			j.handleSquawkChange(t.Squawk)
		}
	}

	// 2. Check Ident for Rising Edge (Edge-Triggered)
	if t.Ident && !j.lastIdent {
		j.handleIdentTrigger()
	}

	j.lastSquawk = t.Squawk
	j.lastIdent = t.Ident
}

func (j *TransponderWatcherJob) handleSquawkChange(squawk int) {
	s := strconv.Itoa(squawk)
	// Squawk is always 4 digits, but for safety:
	if len(s) != 4 {
		return
	}

	// Digits are at index 1, 2, 3
	d1, _ := strconv.Atoi(string(s[1]))
	d2, _ := strconv.Atoi(string(s[2]))
	d3, _ := strconv.Atoi(string(s[3]))

	slog.Info("Transponder: Control Squawk detected", "squawk", squawk, "freq", d1, "len", d2, "boost", d3)

	// Digit 1: Frequency (0-4)
	if d1 == 0 {
		slog.Info("Transponder: Frequency 0 detected, pausing narration")
		j.narrator.Pause()
	} else if d1 >= 1 && d1 <= 4 {
		if j.st != nil {
			_ = j.st.SetState(context.Background(), "narration_frequency", strconv.Itoa(d1))
		}
		// Auto-resume if frequency is moved from 0 to 1-4
		j.narrator.Resume()
	}

	// Digit 2: Length (1-5)
	if d2 >= 1 && d2 <= 5 {
		if j.st != nil {
			_ = j.st.SetState(context.Background(), "text_length", strconv.Itoa(d2))
		}
	}

	// Digit 3: Visibility Boost (1-5)
	// 1: 1.0x, 2: 1.25x, 3: 1.5x, 4: 1.75x, 5: 2.0x
	if d3 >= 1 && d3 <= 5 {
		boost := 1.0 + (float64(d3-1) * 0.25)
		// Persist boost to state store so other components (Overlay/Scorer) see it
		if j.st != nil {
			err := j.st.SetState(context.Background(), "visibility_boost", strconv.FormatFloat(boost, 'f', 2, 64))
			if err != nil {
				slog.Error("Transponder: Failed to persist visibility boost", "error", err)
			}
		}
	}
}

func (j *TransponderWatcherJob) handleIdentTrigger() {
	action := j.cfg.AppConfig().Transponder.IdentAction
	slog.Info("Transponder: IDENT triggered (rising edge)", "action", action)

	switch action {
	case "pause_toggle":
		if j.narrator.IsPaused() {
			j.narrator.Resume()
		} else {
			j.narrator.Pause()
		}
	case "stop":
		j.narrator.Stop()
	case "skip":
		j.narrator.Skip()
	default:
		slog.Warn("Transponder: unknown ident action", "action", action)
	}
}

// isControlSquawk returns true if squawk matches 7[0-5][1-5][1-5]
func isControlSquawk(s int) bool {
	if s < 7000 || s > 7777 {
		return false
	}
	str := strconv.Itoa(s)
	if len(str) != 4 {
		return false
	}
	if str[0] != '7' {
		return false
	}
	// Digit 1: 0-4
	d1 := str[1]
	if d1 < '0' || d1 > '4' {
		return false
	}
	// Digits 2 & 3: 1-5
	for i := 2; i < 4; i++ {
		d := str[i]
		if d < '1' || d > '5' {
			return false
		}
	}
	return true
}
