package geo

import (
	"math"
	"testing"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		name string
		p1   Point
		p2   Point
		want float64
	}{
		{
			name: "Same Point",
			p1:   Point{Lat: 0, Lon: 0},
			p2:   Point{Lat: 0, Lon: 0},
			want: 0,
		},
		{
			name: "London to Paris",
			p1:   Point{Lat: 51.5074, Lon: -0.1278},
			p2:   Point{Lat: 48.8566, Lon: 2.3522},
			want: 344000, // Approx 344km
		},
		{
			name: "Equator 1 degree",
			p1:   Point{Lat: 0, Lon: 0},
			p2:   Point{Lat: 0, Lon: 1},
			want: 111319, // Approx 111km
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Distance(tt.p1, tt.p2)
			// Allow 1% margin of error due to float precision/earth radius var
			margin := tt.want * 0.01
			if math.Abs(got-tt.want) > margin && tt.want != 0 {
				t.Errorf("Distance() = %v, want %v (+/- %v)", got, tt.want, margin)
			}
		})
	}
}

func TestGetLocation(t *testing.T) {
	// We can't easily load the real data in a unit test without the files,
	// but we can test the fallback logic on an empty service.
	s := &Service{
		grid: make(map[int][]City),
	}

	// 1. Fallback (International Waters)
	loc := s.GetLocation(0, 0)
	if loc.CityName != "International Waters" {
		t.Errorf("Expected 'International Waters', got %s", loc.CityName)
	}
	if loc.CountryCode != "XZ" {
		t.Errorf("Expected 'XZ', got %s", loc.CountryCode)
	}

	// 2. Exact Match (Simulation)
	c := City{
		Name:        "TestCity",
		Lat:         10,
		Lon:         20,
		CountryCode: "TC",
		Admin1Code:  "01",
		Admin1Name:  "TestRegion",
	}
	key := s.getGridKey(10, 20)
	s.grid[key] = []City{c}

	loc = s.GetLocation(10.001, 20.001)
	if loc.CityName != "TestCity" {
		t.Errorf("Expected 'TestCity', got %s", loc.CityName)
	}
	if loc.Admin1Name != "TestRegion" {
		t.Errorf("Expected 'TestRegion', got %s", loc.Admin1Name)
	}
}
