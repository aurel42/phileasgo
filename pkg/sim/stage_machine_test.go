package sim

import (
	"log/slog"
	"testing"
	"time"
)

// MockClock for deterministic time testing
type MockClock struct {
	current time.Time
}

func (m *MockClock) Now() time.Time {
	return m.current
}

func (m *MockClock) Advance(d time.Duration) {
	m.current = m.current.Add(d)
}

func TestStageMachine_Robustness(t *testing.T) {
	// Silence logs during tests
	slog.SetLogLoggerLevel(slog.LevelError)

	tests := []struct {
		name  string
		steps []struct {
			telemetry Telemetry
			advance   time.Duration
			wantStage string
		}
	}{
		{
			name: "Mid-Air Start",
			steps: []struct {
				telemetry Telemetry
				advance   time.Duration
				wantStage string
			}{
				// 1. Initial State: !OnGround -> Immediate TakeOff
				{Telemetry{IsOnGround: false, AltitudeAGL: 5000}, 0, StageTakeOff},
				// 2. Next Tick: Transition to Cruise (No lock for mid-air start)
				{Telemetry{IsOnGround: false, AltitudeAGL: 5000}, 1 * time.Second, StageCruise},
			},
		},
		{
			name: "Normal Take-off (Ground -> Air -> 4s -> TakeOff)",
			steps: []struct {
				telemetry Telemetry
				advance   time.Duration
				wantStage string
			}{
				// 1. Initial: OnGround (First tick)
				{Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 10}, 0, StageOnGround},
				// 2. Next tick: Refine to Taxi
				{Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 10}, 1 * time.Second, StageTaxi},
				// 3. Lift off! (T=0) -> Validation Starts, State FROZEN as Taxi
				{Telemetry{IsOnGround: false, AltitudeAGL: 10}, 0, StageTaxi},
				// 4. T+3s: Still validating, State FROZEN as Taxi
				{Telemetry{IsOnGround: false, AltitudeAGL: 50}, 3 * time.Second, StageTaxi},
				// 5. T+4.1s: Validation Complete -> Transition to TakeOff
				{Telemetry{IsOnGround: false, AltitudeAGL: 100}, 1100 * time.Millisecond, StageTakeOff},
				// 6. T+6s: Event Lock Active (Even if we touch ground momentarily?)
				// Let's test normal climb first
				{Telemetry{IsOnGround: false, AltitudeAGL: 200}, 2 * time.Second, StageTakeOff},
				// 7. After Lock expires (4s lock), should go to Climb/Airborne
				{Telemetry{IsOnGround: false, AltitudeAGL: 500, VerticalSpeed: 1000}, 5 * time.Second, StageClimb},
			},
		},
		{
			name: "Rejected Take-off (Jump/Bounce)",
			steps: []struct {
				telemetry Telemetry
				advance   time.Duration
				wantStage string
			}{
				// 1. Initial: OnGround (First tick)
				{Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 20}, 0, StageOnGround},
				// 2. Next tick: Refine to Taxi
				{Telemetry{IsOnGround: true, EngineOn: true, GroundSpeed: 20}, 1 * time.Second, StageTaxi},
				// 3. Hop! (T=0)
				{Telemetry{IsOnGround: false, AltitudeAGL: 5}, 0, StageTaxi},
				// 4. T+2s: Still validating
				{Telemetry{IsOnGround: false, AltitudeAGL: 10}, 2 * time.Second, StageTaxi},
				// 5. T+3s: Touch back down (Bounced)
				// Note: Logic checks condition AT TIMEOUT. But if we touch down *before* timeout,
				// the loop says: `if isGroundState && !t.IsOnGround`.
				// If `t.IsOnGround` is TRUE, we fall through to "reset validation timer" logic.
				{Telemetry{IsOnGround: true, GroundSpeed: 20}, 1 * time.Second, StageTaxi},
				// 6. T+5s: Check if validation cleared? Next tick is Ground, so current logic records Taxi.
				{Telemetry{IsOnGround: true, GroundSpeed: 20}, 2 * time.Second, StageTaxi},
			},
		},
		{
			name: "Normal Landing (Air -> Ground -> 15s -> Landed)",
			steps: []struct {
				telemetry Telemetry
				advance   time.Duration
				wantStage string
			}{
				// 1. Initial: Approach detection (Mid-Air Start -> TakeOff)
				{Telemetry{IsOnGround: false, AltitudeAGL: 500, VerticalSpeed: -500}, 0, StageTakeOff},
				// 2. Next tick: Transition to Descend (Sub-state logic)
				{Telemetry{IsOnGround: false, AltitudeAGL: 500, VerticalSpeed: -500}, 1 * time.Second, StageDescend},
				// 3. Touchdown! (T=0) -> Validation Starts, State FROZEN as Descend
				{Telemetry{IsOnGround: true, GroundSpeed: 80}, 0, StageDescend},
				// 4. T+10s: Still validating (bridges TNG)
				{Telemetry{IsOnGround: true, GroundSpeed: 60}, 10 * time.Second, StageDescend},
				// 5. T+14s: Still validating
				{Telemetry{IsOnGround: true, GroundSpeed: 40}, 4 * time.Second, StageDescend},
				// 6. T+15.1s: Validation Complete -> Landed
				{Telemetry{IsOnGround: true, GroundSpeed: 30}, 1100 * time.Millisecond, StageLanded},
				// 7. T+18s: Event Lock Active (4s total).
				// Even if stopped, returns Landed.
				{Telemetry{IsOnGround: true, GroundSpeed: 0}, 3 * time.Second, StageLanded},
				// 8. T+20s: Lock Expired -> Normal Ground Logic (e.g. Parked/Taxi)
				{Telemetry{IsOnGround: true, EngineOn: false, GroundSpeed: 0}, 2 * time.Second, StageParked},
			},
		},
		{
			name: "Touch and Go (Air -> Ground -> Air)",
			steps: []struct {
				telemetry Telemetry
				advance   time.Duration
				wantStage string
			}{
				// 1. Initial: Approach detection (Mid-Air Start -> TakeOff)
				{Telemetry{IsOnGround: false, AltitudeAGL: 100, VerticalSpeed: -500}, 0, StageTakeOff},
				// 2. Next tick: Transition to Descend
				{Telemetry{IsOnGround: false, AltitudeAGL: 100, VerticalSpeed: -500}, 1 * time.Second, StageDescend},
				// 3. Touchdown! (T=0)
				{Telemetry{IsOnGround: true, GroundSpeed: 80}, 0, StageDescend},
				// 4. T+10s: Rolling on ground
				{Telemetry{IsOnGround: true, GroundSpeed: 80}, 10 * time.Second, StageDescend},
				// 5. T+12s: Lift off again! (IsOnGround = False)
				// Detection Logic: `if isAirborneState && t.IsOnGround`.
				// Since `t.IsOnGround` is False, we fall through to reset.
				// State should update to Climb (VS=800)
				{Telemetry{IsOnGround: false, AltitudeAGL: 10, VerticalSpeed: 800}, 2 * time.Second, StageClimb},
				// 6. T+16s: Still airborne -> Should trigger Climb or Cruise?
				// Validation was reset. Normal logic applies.
				{Telemetry{IsOnGround: false, AltitudeAGL: 500, VerticalSpeed: 1000}, 4 * time.Second, StageClimb},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &MockClock{current: time.Now()} // Start at arbitrary time
			sm := NewStageMachine(clock.Now)

			for i, step := range tt.steps {
				clock.Advance(step.advance)
				got := sm.Update(&step.telemetry)
				if got != step.wantStage {
					t.Errorf("Step %d: wanted %s, got %s (Elapsed: %v)",
						i, step.wantStage, got, step.advance)
				}
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
