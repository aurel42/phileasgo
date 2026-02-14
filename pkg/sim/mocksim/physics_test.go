package mocksim

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/geo"
)

func TestMockPhysics_Accuracy(t *testing.T) {
	// 1. Setup Mock Client at a specific latitude (London: 51.5)
	cfg := Config{
		StartLat: 51.5,
		StartLon: -0.12,
		StartAlt: 0,
	}
	m := NewClient(cfg)
	defer m.Close()

	// 2. Teleport to Airborne state at 120 kts heading 090 (East)
	m.mu.Lock()
	m.state = StageAirborne
	m.tel.Latitude = 51.5
	m.tel.Longitude = -0.12
	m.tel.Heading = 90.0
	m.tel.GroundSpeed = 120.0
	m.lastUpdate = time.Now().Add(-60 * time.Second) // Simulate 1 minute delay
	m.mu.Unlock()

	startPos := geo.Point{Lat: 51.5, Lon: -0.12}

	// 3. Trigger one update (should simulate 1 minute of travel)
	m.update()

	tel, _ := m.GetTelemetry(context.Background())
	endPos := geo.Point{Lat: tel.Latitude, Lon: tel.Longitude}

	// 4. At 120 kts, 1 minute should cover exactly 2.0 NM
	distMeters := geo.Distance(startPos, endPos)
	distNM := distMeters / 1852.0

	t.Logf("Distance covered in 1 minute at 120 kts: %.4f NM", distNM)

	// Tolerance of 0.1% for floating point / geodesic math jitter
	if distNM < 1.99 || distNM > 2.01 {
		t.Errorf("Inaccurate physics: expected ~2.0 NM, got %.4f NM", distNM)
	}

	// 5. Test Latitude Bias (Repeat at high latitude: 80 degrees)
	m.mu.Lock()
	m.tel.Latitude = 80.0
	m.tel.Longitude = 0.0
	m.lastUpdate = time.Now().Add(-60 * time.Second)
	m.mu.Unlock()

	startPosHigh := geo.Point{Lat: 80.0, Lon: 0.0}
	m.update()

	telHigh, _ := m.GetTelemetry(context.Background())
	endPosHigh := geo.Point{Lat: telHigh.Latitude, Lon: telHigh.Longitude}

	distHighNM := geo.Distance(startPosHigh, endPosHigh) / 1852.0
	t.Logf("Distance covered at 80 deg Lat: %.4f NM", distHighNM)

	if distHighNM < 1.99 || distHighNM > 2.01 {
		t.Errorf("Inaccurate physics at high latitude: expected ~2.0 NM, got %.4f NM", distHighNM)
	}
}

func TestMockPhysics_TickIndependance(t *testing.T) {
	// Verify that speed is independent of tick frequency
	cfg := Config{StartLat: 0, StartLon: 0}
	m := NewClient(cfg)
	defer m.Close()

	m.mu.Lock()
	m.state = StageAirborne
	m.tel.GroundSpeed = 120.0
	m.tel.Heading = 0
	m.mu.Unlock()

	startPos := geo.Point{Lat: 0, Lon: 0}

	// A: 10 small ticks of 0.1s
	for i := 0; i < 10; i++ {
		m.mu.Lock()
		m.lastUpdate = time.Now().Add(-100 * time.Millisecond)
		m.mu.Unlock()
		m.update()
	}

	tel, _ := m.GetTelemetry(context.Background())
	distA := geo.Distance(startPos, geo.Point{Lat: tel.Latitude, Lon: tel.Longitude})

	// B: 1 big tick of 1.0s
	m.mu.Lock()
	m.tel.Latitude = 0
	m.tel.Longitude = 0
	m.lastUpdate = time.Now().Add(-1000 * time.Millisecond)
	m.mu.Unlock()
	m.update()

	telB, _ := m.GetTelemetry(context.Background())
	distB := geo.Distance(startPos, geo.Point{Lat: telB.Latitude, Lon: telB.Longitude})

	if mathAbs(distA-distB) > 0.1 {
		t.Errorf("Physics depend on tick frequency: distA=%.4f, distB=%.4f", distA, distB)
	}
}

func mathAbs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func TestMock_ExecuteCommand(t *testing.T) {
	m := NewClient(Config{})
	defer m.Close()

	ctx := context.Background()

	// 1. Unknown command
	if err := m.ExecuteCommand(ctx, "jump", nil); err == nil {
		t.Error("Expected error for unknown command")
	}

	// 2. land on ground (error)
	if err := m.ExecuteCommand(ctx, "land", nil); err == nil {
		t.Error("Expected error for landing while on ground")
	}

	// 3. land while airborne (success)
	m.mu.Lock()
	m.state = StageAirborne
	m.mu.Unlock()

	if err := m.ExecuteCommand(ctx, "land", nil); err != nil {
		t.Errorf("Unexpected error for landing: %v", err)
	}

	if !m.isLanding {
		t.Error("isLanding flag not set")
	}
}

func TestMock_AirborneTurningAndLanding(t *testing.T) {
	m := NewClient(Config{StartAlt: 1000})
	defer m.Close()

	m.mu.Lock()
	m.state = StageAirborne
	m.tel.AltitudeMSL = 2000
	m.groundAlt = 1000
	m.tel.Heading = 100
	m.lastTurnTime = time.Now().Add(-70 * time.Second)
	m.turnCount = 3 // Next turn is 4th (dramatic)
	m.mu.Unlock()

	// 1. Test Dramatic Turn (4th turn)
	m.updateAirborne(0.1, time.Now())

	if m.turnCount != 4 {
		t.Errorf("Expected turnCount 4, got %d", m.turnCount)
	}
	// We can't easily check exact heading due to randomness, but we know it updated.

	// 2. Test Landing Sequence
	m.mu.Lock()
	m.isLanding = true
	m.mu.Unlock()

	m.updateAirborne(2.0, time.Now()) // 2 seconds tick

	if m.tel.GroundSpeed != 70.0 {
		t.Errorf("Expected ground speed 70.0 during landing, got %.1f", m.tel.GroundSpeed)
	}
	if m.tel.VerticalSpeed != -500.0 {
		t.Errorf("Expected vertical speed -500.0 during landing, got %.1f", m.tel.VerticalSpeed)
	}

	// Should have descended: -500fpm = -8.33 fps. 2s = -16.66 ft.
	expectedAlt := 2000.0 - (500.0/60.0)*2.0
	if mathAbs(m.tel.AltitudeMSL-expectedAlt) > 0.1 {
		t.Errorf("Expected altitude %.2f, got %.2f", expectedAlt, m.tel.AltitudeMSL)
	}

	// 3. Test Touchdown
	m.mu.Lock()
	m.tel.AltitudeMSL = 1005 // Almost there
	m.mu.Unlock()

	m.updateAirborne(2.0, time.Now())

	if m.state != StageTaxiing {
		t.Errorf("Expected state StageTaxiing after touchdown, got %s", m.state)
	}
	if !m.tel.IsOnGround {
		t.Error("Expected IsOnGround to be true after touchdown")
	}
	if m.isLanding {
		t.Error("Expected isLanding to be false after touchdown")
	}
}
