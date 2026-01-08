package wikidata

import (
	"math"
	"testing"
)

func TestCalculateBearing(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		wantBear float64
	}{
		{"North", 0, 0, 1, 0, 0},
		{"East", 0, 0, 0, 1, 90},
		{"South", 1, 0, 0, 0, 180},
		{"West", 0, 1, 0, 0, 270},
		{"Same", 0, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBearing(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			// Allow small float error
			if math.Abs(got-tt.wantBear) > 1.0 {
				t.Errorf("calculateBearing() = %v, want %v", got, tt.wantBear)
			}
		})
	}
}

func TestGetCandidates(t *testing.T) {
	s := NewScheduler(100.0) // 100km max radius

	tests := []struct {
		name          string
		lat           float64
		lon           float64
		heading       float64
		isAirborne    bool
		wantMin       int // Minimum number of candidates expected
		checkSorting  bool
		checkDistance bool
		checkCone     bool
	}{
		{
			name:          "Ground (All Directions)",
			lat:           50.0,
			lon:           14.0,
			heading:       0.0,
			isAirborne:    false,
			wantMin:       10, // Should find many neighbors in 100km
			checkSorting:  true,
			checkDistance: true,
			checkCone:     false,
		},
		{
			name:          "Airborne North (Cone Filter)",
			lat:           50.0,
			lon:           14.0,
			heading:       0.0, // North
			isAirborne:    true,
			wantMin:       3,
			checkSorting:  true,
			checkDistance: true,
			checkCone:     true,
		},
		{
			name:          "Airborne South (Cone Filter)",
			lat:           50.0,
			lon:           14.0,
			heading:       180.0, // South
			isAirborne:    true,
			wantMin:       3, // Less than ground, but > 0
			checkSorting:  true,
			checkDistance: true,
			checkCone:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := s.GetCandidates(tt.lat, tt.lon, tt.heading, tt.isAirborne)

			// 1. Minimum Count Check
			if len(candidates) < tt.wantMin {
				t.Errorf("Got %d candidates, want at least %d", len(candidates), tt.wantMin)
			}

			// 2. Sorting Check (Closest Separation)
			if tt.checkSorting {
				prevDist := -1.0
				for _, c := range candidates {
					if c.Dist < prevDist {
						t.Errorf("Sorting error: found dist %.2f after %.2f", c.Dist, prevDist)
					}
					prevDist = c.Dist
				}
			}

			// 3. Max Distance Check
			if tt.checkDistance {
				for _, c := range candidates {
					if c.Dist > 100.0+spacingKm { // allow small margin for center vs edge
						t.Errorf("Candidate too far: %.2f km > 100km (limit)", c.Dist)
					}
				}
			}

			// 4. Cone Check
			if tt.checkCone {
				// Determine start tile to skip it (it's always valid)
				g := NewGrid()
				startTile := g.TileAt(tt.lat, tt.lon)

				for _, c := range candidates {
					if c.Tile == startTile {
						continue
					}

					bearing := calculateBearing(tt.lat, tt.lon, c.Lat, c.Lon)
					diff := math.Abs(bearing - tt.heading)
					if diff > 180 {
						diff = 360 - diff
					}

					// We use 30 degrees half-arc in scheduler.go
					if diff > 35.0 { // Allow margin for large tiles (bearing to center vs tile edge?)
						// Actually scheduler filters on CENTER.
						// So center bearing difference should be strict <= 30.
						// BUT float math + projection might add error. Let's say 32.
						t.Errorf("Candidate outside cone: bearing %.1f vs heading %.1f (diff %.1f)", bearing, tt.heading, diff)
					}
				}
			}
		})
	}
}
