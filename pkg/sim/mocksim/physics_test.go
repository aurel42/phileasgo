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
