package wikidata

import (
	"math"
	"testing"
)

func TestGrid_TileAt(t *testing.T) {
	g := NewGrid()

	tests := []struct {
		name    string
		lat     float64
		lon     float64
		wantErr bool
	}{
		{
			name: "Seattle",
			lat:  47.6062,
			lon:  -122.3321,
		},
		{
			name: "London",
			lat:  51.5074,
			lon:  -0.1278,
		},
		{
			name: "Zero/Zero",
			lat:  0.0,
			lon:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tile := g.TileAt(tt.lat, tt.lon)
			if tile.Index == "" {
				t.Error("TileAt() returned empty index")
			}
			// Verify stability: encoding and decoding center should stay close
			cLat, cLon := g.TileCenter(tile)
			dist := DistKm(tt.lat, tt.lon, cLat, cLon)
			// H3 Res 5 edge length is ~8.5km, so center shouldn't be too far.
			// Max distance from center to vertex is "radius", usually < 12km.
			if dist > 6.0 {
				t.Errorf("TileAt/TileCenter roundtrip distance too large: %.2f km", dist)
			}
		})
	}
}

func TestGrid_Neighbors(t *testing.T) {
	g := NewGrid()

	tests := []struct {
		name      string
		lat       float64
		lon       float64
		wantCount int
	}{
		{"Pacific Ocean", 0.0, -160.0, 6},
		{"North Pole", 89.0, 0.0, 6}, // H3 handles poles, though topology can be weird (k-rings usually 5 or 6)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := g.TileAt(tt.lat, tt.lon)
			neighbors := g.Neighbors(origin)

			if len(neighbors) != tt.wantCount {
				// Pentagons have 5 neighbors, but rare at res 5.
				// Just logging if it's not 6 might be enough to verify it returns *something*.
				// But generally we expect 6.
				t.Errorf("Neighbors() count = %d, want %d", len(neighbors), tt.wantCount)
			}

			// Verify they are distinct
			seen := make(map[string]bool)
			seen[origin.Index] = true
			for _, n := range neighbors {
				if seen[n.Index] {
					t.Errorf("Duplicate neighbor found: %s", n.Index)
				}
				seen[n.Index] = true

				// Verify neighbor is actually adjacent (dist < 2 * spacing)
				// Spacing is ~15km, so centers should be ~15km apart.
				olat, olon := g.TileCenter(origin)
				nlat, nlon := g.TileCenter(n)
				dist := DistKm(olat, olon, nlat, nlon)
				if dist > 12.0 || dist < 0.5 { // Sanity check for Res 6 (~5.6km spacing)
					t.Errorf("Neighbor distance suspicious: %.2f km", dist)
				}
			}
		})
	}
}

func TestGrid_TileRadius(t *testing.T) {
	g := NewGrid()

	tests := []struct {
		name      string
		lat       float64
		lon       float64
		minRadius float64
		maxRadius float64
	}{
		{"Equator", 0.0, 0.0, 3.0, 5.0},    // Res 6 avg radius check
		{"High Lat", 60.0, 10.0, 2.0, 5.0}, // Distortion changes things check
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tile := g.TileAt(tt.lat, tt.lon)
			radius := g.TileRadius(tile)
			if radius < tt.minRadius || radius > tt.maxRadius {
				t.Errorf("TileRadius() = %.2f, want between %.2f and %.2f", radius, tt.minRadius, tt.maxRadius)
			}
		})
	}
}

func TestGrid_TileCorners(t *testing.T) {
	g := NewGrid()

	tests := []struct {
		name    string
		lat     float64
		lon     float64
		wantLen int
	}{
		{"Seattle", 47.6062, -122.3321, 6},
		{"London", 51.5074, -0.1278, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tile := g.TileAt(tt.lat, tt.lon)
			corners := g.TileCorners(tile)

			if len(corners) != tt.wantLen {
				t.Errorf("TileCorners() count = %d, want %d", len(corners), tt.wantLen)
			}

			// Verify corners are reasonably close to the center
			cLat, cLon := g.TileCenter(tile)
			for i, c := range corners {
				dist := DistKm(cLat, cLon, c.Lat, c.Lon)
				if dist > 6.0 || dist < 0.5 { // Sanity check for Res 6
					t.Errorf("Corner %d distance suspicious: %.2f km", i, dist)
				}
			}
		})
	}
}

func TestDistKm(t *testing.T) {
	tests := []struct {
		name string
		p1   [2]float64
		p2   [2]float64
		want float64
		tol  float64
	}{
		{
			name: "Zero Distance",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{0, 0},
			want: 0,
			tol:  0.01,
		},
		{
			name: "1 deg Lat at Equator",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{1, 0},
			want: 111.132,
			tol:  1.0,
		},
		{
			name: "1 deg Lon at Equator",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{0, 1},
			want: 111.132,
			tol:  1.0,
		},
		{
			name: "1 deg Lon at 60 deg Lat", // Cos(60) = 0.5
			p1:   [2]float64{60, 0},
			p2:   [2]float64{60, 1},
			want: 111.132 * 0.5,
			tol:  1.0,
		},
		{
			name: "Dateline crossing (west to east)",
			p1:   [2]float64{0, -179.5},
			p2:   [2]float64{0, 179.5},
			want: 111.132, // 1 degree apart
			tol:  1.0,
		},
		{
			name: "Dateline crossing (east to west)",
			p1:   [2]float64{0, 179.5},
			p2:   [2]float64{0, -179.5},
			want: 111.132, // 1 degree apart
			tol:  1.0,
		},
		{
			name: "Dateline crossing at high lat (Chukotka)",
			p1:   [2]float64{65.5, -179.7},
			p2:   [2]float64{65.5, 179.0},
			want: 111.132 * 1.3 * 0.42, // ~1.3 deg at lat 65.5 (cos ~0.42)
			tol:  10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DistKm(tt.p1[0], tt.p1[1], tt.p2[0], tt.p2[1])
			if math.Abs(got-tt.want) > tt.tol {
				t.Errorf("DistKm() = %v, want %v (+/-%v)", got, tt.want, tt.tol)
			}
		})
	}
}
