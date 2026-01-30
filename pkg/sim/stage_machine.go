package sim

import (
	"strings"
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
	current       string
	candidate     string
	confirmations int
	wasOnGround   bool
	wasAirborne   bool
}

// NewStageMachine creates a stage machine in an uninitialized state.
func NewStageMachine() *StageMachine {
	return &StageMachine{
		current: "",
	}
}

// Update evaluates telemetry and returns the current refined stage.
func (m *StageMachine) Update(t *Telemetry) string {
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

func (m *StageMachine) detectCandidate(t *Telemetry) string {
	if t.IsOnGround {
		return m.detectGroundCandidate(t)
	}
	return m.detectAirborneCandidate(t)
}

func (m *StageMachine) detectGroundCandidate(t *Telemetry) string {
	// Ground Sub-States
	if !t.EngineOn && t.GroundSpeed < 1 {
		return StageParked
	}

	// Takeoff Roll
	if t.GroundSpeed > 40 {
		return StageTakeOff
	}

	if t.EngineOn {
		if t.GroundSpeed >= 5 && t.GroundSpeed <= 25 {
			return StageTaxi
		}
		if t.GroundSpeed < 1 {
			return StageHold
		}
	}

	// Landmark: Landed (if was airborne and haven't hit other states yet)
	if m.wasAirborne {
		return StageLanded
	}

	return StageOnGround
}

func (m *StageMachine) detectAirborneCandidate(t *Telemetry) string {
	// Landmark: Initial Takeoff (Airborne but low and was on ground)
	if m.wasOnGround && t.AltitudeAGL < 500 {
		return StageTakeOff
	}

	// Performance States
	if t.VerticalSpeed > 300 {
		return StageClimb
	}
	if t.VerticalSpeed < -300 {
		return StageDescend
	}
	if t.VerticalSpeed > -200 && t.VerticalSpeed < 200 {
		return StageCruise
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
