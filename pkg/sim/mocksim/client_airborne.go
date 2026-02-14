package mocksim

import (
	"math"
	"math/rand"
	"phileasgo/pkg/geo"
	"time"
)

func (m *MockClient) updateAirborne(dt float64, now time.Time) {
	m.tel.IsOnGround = false
	// Wander logic - Every fourth turn is a drastic 45 degree turn
	if now.Sub(m.lastTurnTime) > 60*time.Second {
		m.turnCount++
		maxChange := 10.0
		if m.turnCount%4 == 0 {
			maxChange = 45.0
		}

		change := (rand.Float64() * (maxChange * 2)) - maxChange // -maxChange to +maxChange
		m.tel.Heading = math.Mod(m.tel.Heading+change, 360.0)
		if m.tel.Heading < 0 {
			m.tel.Heading += 360.0
		}
		m.lastTurnTime = now
	}

	if m.isLanding {
		// Landing sequence: 500fpm descent and 70kts speed
		m.tel.GroundSpeed = 70.0
		m.tel.VerticalSpeed = -500.0

		delta := (m.tel.VerticalSpeed / 60.0) * dt
		m.tel.AltitudeMSL += delta

		if m.tel.AltitudeMSL <= m.groundAlt {
			m.tel.AltitudeMSL = m.groundAlt
			m.tel.VerticalSpeed = 0
			m.tel.IsOnGround = true
			m.tel.GroundSpeed = 10.0 // Slow taxi
			m.isLanding = false
			m.state = StageTaxiing
			m.stateStart = now
		}
	} else {
		m.tel.GroundSpeed = 120.0
		m.updateScenario(dt, now)
	}

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
	// Don't enforce if we are landing.
	if !m.isLanding {
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
}
