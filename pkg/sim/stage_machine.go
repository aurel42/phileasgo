package sim

import (
	"log/slog"
	"strings"
	"time"
)

const (
	StageOnGround = "on_the_ground"
	StageParked   = "parked"
	StageTaxi     = "taxi"
	StageHold     = "hold"
	StageTakeOff  = "take-off"
	StageAirborne = "airborne"
	StageClimb    = "climb"
	StageCruise   = "cruise"
	StageDescend  = "descend"
	StageLanded   = "landed"
)

// StageMachine tracks the flight phase state across telemetry ticks.
type StageMachine struct {
	current         string
	lastGroundSpeed float64
	isAccelerating  bool
	isDecelerating  bool
	lastTransition  map[string]time.Time

	// Robustness fields
	transitionStart time.Time // When the potential state change was first detected
	lockedUntil     time.Time // Time until which the state is frozen (transition hold)

	// Time source for testing
	now func() time.Time

	// Event Recording
	recorder EventRecorder
}

// SetRecorder injects an event recorder.
func (m *StageMachine) SetRecorder(r EventRecorder) {
	m.recorder = r
}

// StageState represents the persistent state of the machine.
type StageState struct {
	Current        string               `json:"current"`
	LastTransition map[string]time.Time `json:"last_transition"`
}

// NewStageMachine creates a stage machine in an uninitialized state.
func NewStageMachine(timeSource ...func() time.Time) *StageMachine {
	clock := time.Now
	if len(timeSource) > 0 {
		clock = timeSource[0]
	}
	return &StageMachine{
		current:        "",
		lastTransition: make(map[string]time.Time),
		now:            clock,
	}
}

// Update evaluates telemetry and returns the current refined stage.
func (m *StageMachine) Update(t *Telemetry) string {
	now := m.now()
	previous := m.current

	// 1. Initial State (First Tick)
	if m.current == "" {
		return m.initializeState(t, now)
	}

	// 2. Event Hold (Lock)
	if now.Before(m.lockedUntil) {
		return m.current
	}

	// Trend Tracking
	m.updateTrends(t)

	// 3. Validation Logic (Frozen State)
	if stage, handled := m.checkTransitions(t, now); handled {
		return stage
	}

	// 4. Normal Sub-state Logic
	if t.IsOnGround {
		m.current = m.updateGroundState(t, m.current)
	} else {
		m.current = m.updateAirborneState(t, m.current)
	}

	// Log silent transitions
	if m.current != "" && m.current != previous {
		m.recordTransition(m.current, now, t.Latitude, t.Longitude)
		slog.Debug("StageMachine: State transition", "from", previous, "to", m.current)
	}

	return m.current
}

func (m *StageMachine) initializeState(t *Telemetry, now time.Time) string {
	if t.IsOnGround {
		m.current = StageOnGround
	} else {
		// Mid-air start: Trigger synthetic Take-off
		m.current = StageTakeOff
		m.recordTransition(StageTakeOff, now, t.Latitude, t.Longitude)
		slog.Info("StageMachine: Mid-air start detected", "stage", m.current)
	}
	return m.current
}

func (m *StageMachine) updateTrends(t *Telemetry) {
	m.isAccelerating = t.GroundSpeed > m.lastGroundSpeed+1
	m.isDecelerating = t.GroundSpeed < m.lastGroundSpeed-1
	m.lastGroundSpeed = t.GroundSpeed
}

func (m *StageMachine) checkTransitions(t *Telemetry, now time.Time) (string, bool) {
	isAirborneState := isAirborne(m.current)
	isGroundState := !isAirborneState

	// Case A: Ground -> Air (Potential Take-off)
	if isGroundState && !t.IsOnGround {
		return m.handleTakeoffCandidate(t, now)
	}

	// Case B: Air -> Ground (Potential Landing)
	if isAirborneState && t.IsOnGround {
		return m.handleLandingCandidate(t, now)
	}

	// Reset validation if state matches physical reality
	if (isGroundState && t.IsOnGround) || (isAirborneState && !t.IsOnGround) {
		m.transitionStart = time.Time{}
	}

	return "", false
}

func (m *StageMachine) handleTakeoffCandidate(t *Telemetry, now time.Time) (string, bool) {
	if m.transitionStart.IsZero() {
		m.transitionStart = now
	}

	// Wait 4 seconds for valid take-off
	if now.Sub(m.transitionStart) > 4*time.Second {
		// Check One-Time Confirmation
		if !t.IsOnGround {
			// Confirmed Take-off
			m.current = StageTakeOff
			m.recordTransition(StageTakeOff, now, t.Latitude, t.Longitude)
			m.lockedUntil = now.Add(4 * time.Second)
			m.transitionStart = time.Time{} // Reset
			slog.Info("StageMachine: Take-off Confirmed", "stage", m.current)
			return m.current, true
		}
		// Failed Validation (Bounce/Glitch)
		m.transitionStart = time.Time{}
		return "", false // Fall through to ground state update
	}
	// FROZEN: Return current ground state while validating
	return m.current, true
}

func (m *StageMachine) handleLandingCandidate(t *Telemetry, now time.Time) (string, bool) {
	if m.transitionStart.IsZero() {
		m.transitionStart = now
	}

	// Wait 15 seconds for valid landing (bridges TNGs)
	if now.Sub(m.transitionStart) > 15*time.Second {
		// Check One-Time Confirmation
		if t.IsOnGround {
			// Confirmed Landing
			m.current = StageLanded
			m.recordTransition(StageLanded, now, t.Latitude, t.Longitude)
			m.lockedUntil = now.Add(4 * time.Second)
			m.transitionStart = time.Time{} // Reset
			slog.Info("StageMachine: Landing Confirmed", "stage", m.current)
			return m.current, true
		}
		// Failed Validation (Touch and Go / Bounce)
		m.transitionStart = time.Time{}
		return "", false // Fall through to airborne state update
	}
	// FROZEN: Return current airborne state while validating
	return m.current, true
}

func (m *StageMachine) recordTransition(stage string, t time.Time, lat, lon float64) {
	m.lastTransition[stage] = t

	// Trigger System Event
	if m.recorder != nil {
		if stage == StageTakeOff {
			m.recorder.RecordSystemEvent("Take-off", "flight_stage", lat, lon, map[string]string{
				"flight_stage": stage,
			})
		} else if stage == StageLanded {
			m.recorder.RecordSystemEvent("Landed", "flight_stage", lat, lon, map[string]string{
				"flight_stage": stage,
			})
		}
	}
}

// GetState returns a snapshot of the current state.
func (m *StageMachine) GetState() StageState {
	// Deep copy the map
	transitions := make(map[string]time.Time, len(m.lastTransition))
	for k, v := range m.lastTransition {
		transitions[k] = v
	}

	return StageState{
		Current:        m.current,
		LastTransition: transitions,
	}
}

// RestoreState restores the machine state from a snapshot.
func (m *StageMachine) RestoreState(s StageState) {
	m.current = s.Current
	// Deep copy the map
	m.lastTransition = make(map[string]time.Time, len(s.LastTransition))
	for k, v := range s.LastTransition {
		m.lastTransition[k] = v
	}
	slog.Info("StageMachine: State restored", "stage", m.current)
}

func isAirborne(stage string) bool {
	switch stage {
	case StageTakeOff, StageAirborne, StageClimb, StageCruise, StageDescend:
		return true
	}
	return false
}

func (m *StageMachine) updateGroundState(t *Telemetry, current string) string {
	// If we just landed and lock expired, we might still be 'Landed'.
	// Transition to Taxi/Hold/Parked based on speed/engine.

	// Parked
	if !t.EngineOn && t.GroundSpeed < 1 {
		return StageParked
	}

	// Engine On
	if t.EngineOn {
		if t.GroundSpeed >= 5 {
			return StageTaxi
		}
		// If very slow, Hold
		if t.GroundSpeed < 1 {
			return StageHold
		}
	}

	// Fallback: If we are already in a valid ground state, keep it.
	// E.g. Waiting between 1 and 5 knots.
	switch current {
	case StageParked, StageTaxi, StageHold, StageLanded, StageOnGround:
		return current
	}

	return StageOnGround
}

func (m *StageMachine) updateAirborneState(t *Telemetry, current string) string {
	// Simple performance-based states
	if t.VerticalSpeed > 300 {
		return StageClimb
	}
	if t.VerticalSpeed < -300 {
		return StageDescend
	}
	// Stable
	return StageCruise
}

func (m *StageMachine) Current() string {
	return m.current
}

// GetLastTransition returns the timestamp of the last transition to the given stage.
func (m *StageMachine) GetLastTransition(stage string) time.Time {
	if m.lastTransition == nil {
		return time.Time{}
	}
	return m.lastTransition[stage]
}

// FormatStage returns a human-readable title for the stage.
func FormatStage(s string) string {
	if s == "" {
		return "Unknown"
	}
	// on_the_ground -> On the Ground
	// take-off -> Take-off
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[0:1]) + p[1:]
		}
	}
	res := strings.Join(parts, " ")

	// Handle take-off hyphen
	if strings.Contains(res, "-") {
		sub := strings.Split(res, "-")
		for i, p := range sub {
			if p != "" {
				sub[i] = strings.ToUpper(p[0:1]) + p[1:]
			}
		}
		res = strings.Join(sub, "-")
	}

	return res
}

// FlightDuration returns the duration of the flight in seconds since take-off,
// or 0 if a take-off timestamp is not available.
func (m *StageMachine) FlightDuration() float64 {
	takeOffTime, ok := m.lastTransition[StageTakeOff]
	if !ok || takeOffTime.IsZero() {
		return 0
	}
	return time.Since(takeOffTime).Seconds()
}
