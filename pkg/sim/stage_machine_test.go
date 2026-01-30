package sim

import (
	"testing"
)

func TestStageMachine(t *testing.T) {

	tests := []struct {
		name        string
		tel         Telemetry
		ticks       int
		expected    string
		wasAirborne bool
	}{
		{
			name: "Initial Parked",
			tel: Telemetry{
				IsOnGround:  true,
				EngineOn:    false,
				GroundSpeed: 0,
			},
			ticks:    2,
			expected: StageParked,
		},
		{
			name: "Transition to Taxi",
			tel: Telemetry{
				IsOnGround:  true,
				EngineOn:    true,
				GroundSpeed: 10,
			},
			ticks:    2,
			expected: StageTaxi,
		},
		{
			name: "Transition to TakeOff Roll",
			tel: Telemetry{
				IsOnGround:  true,
				EngineOn:    true,
				GroundSpeed: 50,
			},
			ticks:    2,
			expected: StageTakeOff,
		},
		{
			name: "Transition to TakeOff Airborne",
			tel: Telemetry{
				IsOnGround:  false,
				AltitudeAGL: 100,
			},
			ticks:    2,
			expected: StageTakeOff,
		},
		{
			name: "Transition to Climb",
			tel: Telemetry{
				IsOnGround:    false,
				AltitudeAGL:   1000,
				VerticalSpeed: 500,
			},
			ticks:    2,
			expected: StageClimb,
		},
		{
			name: "Transition to Cruise",
			tel: Telemetry{
				IsOnGround:    false,
				AltitudeAGL:   5000,
				VerticalSpeed: 50,
			},
			ticks:    2,
			expected: StageCruise,
		},
		{
			name: "Transition to Descend",
			tel: Telemetry{
				IsOnGround:    false,
				AltitudeAGL:   3000,
				VerticalSpeed: -500,
			},
			ticks:    2,
			expected: StageDescend,
		},
		{
			name: "Transition to Landed",
			tel: Telemetry{
				IsOnGround:  true,
				GroundSpeed: 30,
			},
			wasAirborne: true,
			ticks:       2,
			expected:    StageLanded,
		},
		{
			name: "Hysteresis Check (1 tick no change)",
			tel: Telemetry{
				IsOnGround:  true,
				EngineOn:    false,
				GroundSpeed: 0,
			},
			ticks:    1,
			expected: StageOnGround, // Starts at OnGround, candidate is Parked
		},
		{
			name: "Hysteresis Check (2 ticks commit)",
			tel: Telemetry{
				IsOnGround:  true,
				EngineOn:    false,
				GroundSpeed: 0,
			},
			ticks:    2,
			expected: StageParked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStageMachine()
			if tt.wasAirborne {
				sm.wasAirborne = true
			}
			var last string
			for i := 0; i < tt.ticks; i++ {
				last = sm.Update(&tt.tel)
			}
			if last != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, last)
			}
		})
	}
}

func TestFormatStage(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"parked", "Parked"},
		{"on_the_ground", "On The Ground"},
		{"take-off", "Take-Off"},
		{"climb", "Climb"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		if got := FormatStage(tt.in); got != tt.want {
			t.Errorf("FormatStage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
