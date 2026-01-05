package sim

import "testing"

func TestUpdateState(t *testing.T) {
	tests := []struct {
		name        string
		cameraState int32
		wantState   *State
	}{
		// Active states
		{"camera 2 -> active", 2, ptr(StateActive)},
		{"camera 3 -> active", 3, ptr(StateActive)},
		{"camera 4 -> active", 4, ptr(StateActive)},
		{"camera 30 -> active", 30, ptr(StateActive)},
		{"camera 34 -> active", 34, ptr(StateActive)},
		// Inactive states
		{"camera 12 -> inactive", 12, ptr(StateInactive)},
		{"camera 15 -> inactive", 15, ptr(StateInactive)},
		{"camera 32 -> inactive", 32, ptr(StateInactive)},
		// Ignored states (return nil)
		{"camera 0 -> ignored", 0, nil},
		{"camera 1 -> ignored", 1, nil},
		{"camera 5 -> ignored", 5, nil},
		{"camera 100 -> ignored", 100, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpdateState(tt.cameraState)
			if tt.wantState == nil {
				if got != nil {
					t.Errorf("UpdateState(%d) = %v, want nil", tt.cameraState, *got)
				}
			} else {
				if got == nil {
					t.Errorf("UpdateState(%d) = nil, want %v", tt.cameraState, *tt.wantState)
				} else if *got != *tt.wantState {
					t.Errorf("UpdateState(%d) = %v, want %v", tt.cameraState, *got, *tt.wantState)
				}
			}
		})
	}
}

func ptr(s State) *State {
	return &s
}
