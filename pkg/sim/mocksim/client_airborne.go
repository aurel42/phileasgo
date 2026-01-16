package mocksim

import (
	"math"
	"math/rand"
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

	// Move
	distNm := m.tel.GroundSpeed * (dt / 3600.0)
	distDeg := distNm / 60.0
	radHeading := m.tel.Heading * (math.Pi / 180.0)
	m.tel.Latitude += distDeg * math.Cos(radHeading)
	m.tel.Longitude += distDeg * math.Sin(radHeading)

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
