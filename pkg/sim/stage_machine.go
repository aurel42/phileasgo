package sim

import (
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
	candidate       string
	confirmations   int
	wasOnGround     bool
	wasAirborne     bool
	lastGroundSpeed float64
	isAccelerating  bool
	isDecelerating  bool
	lastTransition  map[string]time.Time
}

// NewStageMachine creates a stage machine in an uninitialized state.
func NewStageMachine() *StageMachine {
	return &StageMachine{
		current:        "",
		lastTransition: make(map[string]time.Time),
	}
}

// Update evaluates telemetry and returns the current refined stage.
func (m *StageMachine) Update(t *Telemetry) string {
	// Trend Tracking
	if m.current != "" {
		m.isAccelerating = t.GroundSpeed > m.lastGroundSpeed+1
		m.isDecelerating = t.GroundSpeed < m.lastGroundSpeed-1
	}
	m.lastGroundSpeed = t.GroundSpeed

	// First-tick Initialization: determine fallback from actual ground status
	if m.current == "" {
		if t.IsOnGround {
			m.current = StageOnGround
			m.wasOnGround = true
		} else {
			m.current = StageAirborne
			m.wasAirborne = true
		}
		// Skip hysteresis for initial state
		return m.current
	}

	candidate := m.detectCandidate(t)

	// Hysteresis: Require 2 ticks to confirm state change
	switch {
	case candidate == m.current:
		m.candidate = ""
		m.confirmations = 0
	case candidate == m.candidate:
		m.confirmations++
		if m.confirmations >= 1 { // 0+1 = 2 ticks total (first detect + 1 confirmation)
			m.current = candidate
			m.lastTransition[m.current] = time.Now()
			m.candidate = ""
			m.confirmations = 0
		}
	default:
		m.candidate = candidate
		m.confirmations = 0
	}

	// Persistent State Management
	if t.IsOnGround {
		m.wasOnGround = true
	}
	if !t.IsOnGround {
		m.wasAirborne = true
	}

	// Resets
	switch m.current {
	case StageClimb, StageCruise:
		m.wasOnGround = false
	case StageTaxi, StageHold, StageParked:
		m.wasAirborne = false
	}

	return m.current
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

func (m *StageMachine) detectCandidate(t *Telemetry) string {
	if t.IsOnGround {
		return m.detectGroundCandidate(t)
	}
	return m.detectAirborneCandidate(t)
}

func (m *StageMachine) detectGroundCandidate(t *Telemetry) string {
	// 1. Landed (Priority: Airborne -> Ground + Decelerating or stable after touchdown)
	// We check wasAirborne to ensure we don't trigger Landed while already on ground.
	if m.wasAirborne {
		if m.isDecelerating || t.GroundSpeed < 40 {
			return StageLanded
		}
	}

	// 2. Takeoff Roll (Priority: Needs to be accelerating)
	// User Requirement: trigger take-off only if we were taxiing or holding in the past 10 mins
	if t.GroundSpeed > 40 && m.isAccelerating {
		lastTaxi := m.lastTransition[StageTaxi]
		lastHold := m.lastTransition[StageHold]
		if time.Since(lastTaxi) < 10*time.Minute || time.Since(lastHold) < 10*time.Minute {
			return StageTakeOff
		}
	}

	// 3. Ground Sub-States
	if !t.EngineOn && t.GroundSpeed < 1 {
		return StageParked
	}

	if t.EngineOn {
		if t.GroundSpeed >= 5 && t.GroundSpeed <= 25 {
			return StageTaxi
		}
		if t.GroundSpeed < 1 {
			return StageHold
		}
	}

	// Fallback: Maintain current state if it's already a ground state
	switch m.current {
	case StageParked, StageTaxi, StageHold, StageTakeOff, StageLanded:
		return m.current
	}

	return StageOnGround
}

func (m *StageMachine) detectAirborneCandidate(t *Telemetry) string {
	// 1. Performance States (Trend established)
	if t.VerticalSpeed > 300 {
		return StageClimb
	}
	if t.VerticalSpeed < -300 {
		return StageDescend
	}
	if t.VerticalSpeed > -200 && t.VerticalSpeed < 200 {
		return StageCruise
	}

	// 2. Initial Takeoff (Airborne but was just on ground and no performance trend yet)
	// User Requirement: trigger take-off only if we were taxiing or holding in the past 10 mins
	if m.wasOnGround {
		lastTaxi := m.lastTransition[StageTaxi]
		lastHold := m.lastTransition[StageHold]
		if time.Since(lastTaxi) < 10*time.Minute || time.Since(lastHold) < 10*time.Minute {
			return StageTakeOff
		}
	}

	// Fallback: Maintain current state if it's already an airborne state
	switch m.current {
	case StageAirborne, StageClimb, StageCruise, StageDescend, StageTakeOff:
		return m.current
	}

	return StageAirborne
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
