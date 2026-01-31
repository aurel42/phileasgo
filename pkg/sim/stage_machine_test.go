package sim

import (
	"testing"
	"time"
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
			name: "Landed Detection: High Speed Roll",
			sequence: []Telemetry{
				{IsOnGround: false, AltitudeAGL: 5000, VerticalSpeed: 0}, // Start airborne
				{IsOnGround: true, GroundSpeed: 80},                      // Touchdown at high speed
				{IsOnGround: true, GroundSpeed: 75},                      // Decelerating (detected as Landed)
				{IsOnGround: true, GroundSpeed: 60},                      // Still decelerating (confirmed Landed)
			},
			expected: StageLanded,
		},
		{
			name: "TakeOff: Must be Accelerating",
			sequence: []Telemetry{
				{IsOnGround: true, EngineOn: true, GroundSpeed: 0},  // 1. Initial (StageOnGround)
				{IsOnGround: true, EngineOn: true, GroundSpeed: 0},  // 2. Hold candidate
				{IsOnGround: true, EngineOn: true, GroundSpeed: 0},  // 3. Hold confirmed (lastTransition[Hold] set)
				{IsOnGround: true, EngineOn: true, GroundSpeed: 20}, // 4. Taxi candidate
				{IsOnGround: true, EngineOn: true, GroundSpeed: 20}, // 5. Taxi confirmed (lastTransition[Taxi] set)
				{IsOnGround: true, EngineOn: true, GroundSpeed: 45}, // 6. TakeOff candidate
				{IsOnGround: true, EngineOn: true, GroundSpeed: 55}, // 7. TakeOff confirmed
			},
			expected: StageTakeOff,
		},
		{
			name: "Abort TakeOff: Decelerating",
			sequence: []Telemetry{
				{IsOnGround: true, EngineOn: true, GroundSpeed: 50}, // TakeOff Roll (was accelerating)
				{IsOnGround: true, EngineOn: true, GroundSpeed: 60}, // TakeOff Confirmed
				{IsOnGround: true, EngineOn: true, GroundSpeed: 55}, // Abort! Decelerating
				{IsOnGround: true, EngineOn: true, GroundSpeed: 40}, // Decelerating further -> Landed/Taxi fallback
			},
			expected: StageOnGround, // Since we were "TakeOff" and now decelerating on ground, but never was airborne
		},
		{
			name: "Touch and Go (Now fails TakeOff without Taxi - Expected behavior change)",
			sequence: []Telemetry{
				{IsOnGround: true, EngineOn: true, GroundSpeed: 10},        // 1. Initial (OnGround)
				{IsOnGround: true, EngineOn: true, GroundSpeed: 10},        // 2. Taxi candidate
				{IsOnGround: true, EngineOn: true, GroundSpeed: 10},        // 3. Taxi confirmed (lastTransition set)
				{IsOnGround: false, AltitudeAGL: 100, VerticalSpeed: -500}, // 4. TakeOff candidate (detected via wasOnGround)
				{IsOnGround: false, AltitudeAGL: 500, VerticalSpeed: 800},  // 5. Climb candidate
				{IsOnGround: false, AltitudeAGL: 1000, VerticalSpeed: 800}, // 6. Climb confirmed
				{IsOnGround: true, GroundSpeed: 70},                        // 7. Landed candidate (wasAirborne=true)
				{IsOnGround: true, GroundSpeed: 65},                        // 8. Landed confirmed
				{IsOnGround: true, GroundSpeed: 75},                        // 9. Take-off roll candidate (wasLanded/RecentlyTaxiied)
				{IsOnGround: true, GroundSpeed: 85},                        // 10. Take-off Confirmed
			},
			expected: StageTakeOff,
		},
		{
			name: "Spurious TakeOff: Skipping Taxi/Hold",
			sequence: []Telemetry{
				{IsOnGround: true, EngineOn: true, GroundSpeed: 45}, // Above 40, accelerating
				{IsOnGround: true, EngineOn: true, GroundSpeed: 55}, // Should NOT be TakeOff
			},
			expected: StageOnGround,
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

func TestStageMachine_FlightDuration(t *testing.T) {
	sm := NewStageMachine()

	// 1. Initial state
	if dur := sm.FlightDuration(); dur != 0 {
		t.Errorf("Expected 0 flight duration initially, got %f", dur)
	}

	// 2. Set up a takeoff
	sm.Update(&Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 10}) // Initial (OnGround)
	sm.Update(&Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 10}) // Taxi (Candidate)
	sm.Update(&Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 10}) // Taxi Confirmed
	sm.Update(&Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 50}) // TakeOff (Candidate)
	sm.Update(&Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 60}) // TakeOff Confirmed

	time.Sleep(10 * time.Millisecond) // Ensure duration is measurable

	dur := sm.FlightDuration()
	if dur <= 0 {
		t.Errorf("Expected positive flight duration after takeoff, got %f", dur)
	}
}
