package mocksim

import (
	"math"
	"math/rand"
	"phileasgo/pkg/geo"
	"time"
)

func (m *MockClient) updateAirborne(dt float64, now time.Time) {
	m.tel.IsOnGround = false
	m.tel.GroundSpeed = 120.0
	// Wander logic
	if now.Sub(m.lastTurnTime) > 60*time.Second {
		change := (rand.Float64() * 20) - 10 // -10 to +10 degrees
		m.tel.Heading = math.Mod(m.tel.Heading+change, 360.0)
		if m.tel.Heading < 0 {
			m.tel.Heading += 360.0
		}
		m.lastTurnTime = now
	}

	m.updateScenario(dt, now)

	// Move using Geodesic Math
	distMeters := m.tel.GroundSpeed * 0.514444 * dt // knots to m/s
	if distMeters > 0 {
		nextPos := geo.DestinationPoint(
			geo.Point{Lat: m.tel.Latitude, Lon: m.tel.Longitude},
			distMeters,
			m.tel.Heading,
		)
		m.tel.Latitude = nextPos.Lat
		m.tel.Longitude = nextPos.Lon
	}

	// TERRAIN FOLLOWING: Ensure we don't crash into mountains.
	// Enforce min 500ft AGL if airborne, BUT only after we've naturally reached it to avoid takeoff snap.
	minAlt := m.groundAlt + 500.0

	// Check if we have reached safe altitude
	if !m.safeAltReached {
		if m.tel.AltitudeMSL >= minAlt {
			m.safeAltReached = true
		}
	}

	// Only clamp if we have reached safe alt previously
	if m.safeAltReached {
		if m.tel.AltitudeMSL < minAlt {
			m.tel.AltitudeMSL = minAlt
		}
	}
}
