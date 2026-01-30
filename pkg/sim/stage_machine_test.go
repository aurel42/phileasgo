package sim

import (
	"testing"
)

func TestStageMachine(t *testing.T) {

	tests := []struct {
		name     string
		sequence []Telemetry
		expected string
	}{
		{
			name: "Start Mid-Air (Initial)",
			sequence: []Telemetry{
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0},
			},
			expected: StageAirborne,
		},
		{
			name: "Start Mid-Air (Confirm Cruise)",
			sequence: []Telemetry{
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0},
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0},
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0},
			},
			expected: StageCruise,
		},
		{
			name: "Start On Ground (Initial)",
			sequence: []Telemetry{
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
			},
			expected: StageOnGround,
		},
		{
			name: "Start On Ground (Confirm Parked)",
			sequence: []Telemetry{
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
			},
			expected: StageParked,
		},
		{
			name: "Normal Flow: Parked -> Taxi -> TakeOff -> Climb -> Cruise",
			sequence: []Telemetry{
				// 1. Init
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
				// 2. Parked
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
				{IsOnGround: true, EngineOn: false, GroundSpeed: 0},
				// 3. Taxi
				{IsOnGround: true, EngineOn: true, GroundSpeed: 10},
				{IsOnGround: true, EngineOn: true, GroundSpeed: 10},
				// 4. TakeOff Roll
				{IsOnGround: true, EngineOn: true, GroundSpeed: 60},
				{IsOnGround: true, EngineOn: true, GroundSpeed: 80},
				// 5. Airborne TakeOff
				{IsOnGround: false, AltitudeAGL: 100, VerticalSpeed: 500},
				{IsOnGround: false, AltitudeAGL: 200, VerticalSpeed: 600},
				// 6. Climb
				{IsOnGround: false, AltitudeAGL: 1000, VerticalSpeed: 800},
				{IsOnGround: false, AltitudeAGL: 2000, VerticalSpeed: 800},
				// 7. Cruise
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0},
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0},
			},
			expected: StageCruise,
		},
		{
			name: "Mid-Air Start: No Spurious TakeOff",
			sequence: []Telemetry{
				{IsOnGround: false, AltitudeAGL: 200, VerticalSpeed: 0}, // App starts at 200ft
				{IsOnGround: false, AltitudeAGL: 200, VerticalSpeed: 0},
				{IsOnGround: false, AltitudeAGL: 200, VerticalSpeed: 0},
			},
			expected: StageCruise,
		},
		{
			name: "Landed Detection",
			sequence: []Telemetry{
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0}, // Start airborne
				{IsOnGround: true, GroundSpeed: 40},                      // Touchdown
				{IsOnGround: true, GroundSpeed: 20},                      // Slowing down
			},
			expected: StageLanded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStageMachine()
			var last string
			for _, tel := range tt.sequence {
				last = sm.Update(&tel)
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
