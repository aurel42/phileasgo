// Package sim provides simulator client interfaces and types.
package sim

// State represents the connection and activity state of the simulator.
type State string

const (
	// StateDisconnected indicates no connection to the simulator.
	StateDisconnected State = "disconnected"
	// StateInactive indicates connected but not in active flight (menu/pause).
	StateInactive State = "inactive"
	// StateActive indicates connected and in active flight.
	StateActive State = "active"
)

// Camera states that force ACTIVE state (cockpit/chase views).
var ActiveCameraStates = map[int32]bool{
	2:  true, // Cockpit
	3:  true, // Chase
	4:  true, // Drone
	16: true, // Cinematic
	30: true, // Cockpit VR
	34: true, // Chase VR
}

// Camera states that force INACTIVE state (menu/pause views).
var InactiveCameraStates = map[int32]bool{
	12: true, // Menu
	15: true, // Pause
	32: true, // Loading
}

// UpdateState returns the new state based on camera value.
// Returns nil if the camera state should be ignored (keep current state).
func UpdateState(cameraState int32) *State {
	if ActiveCameraStates[cameraState] {
		s := StateActive
		return &s
	}
	if InactiveCameraStates[cameraState] {
		s := StateInactive
		return &s
	}
	return nil // Ignore unknown camera states
}
