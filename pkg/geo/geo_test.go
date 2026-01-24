package geo

import (
	"math"
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
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

	// 1. Fallback (Empty City)
	loc := s.GetLocation(0, 0)
	if loc.CityName != "" {
		t.Errorf("Expected empty city name, got %s", loc.CityName)
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

func TestGetLocation_CrossBorder(t *testing.T) {
	s := &Service{
		grid: make(map[int][]City),
	}

	// City in France
	s.grid[s.getGridKey(48, 7)] = []City{{
		Name:        "FrenchCity",
		Lat:         48.0,
		Lon:         7.0,
		CountryCode: "FR",
		Admin1Name:  "FrenchRegion",
	}}

	// Mock CountryService that says we are in Germany
	s.countrySvc = &CountryService{
		features: &geojson.FeatureCollection{
			Features: []*geojson.Feature{
				{
					Properties: map[string]interface{}{"ISO_A2": "DE", "NAME": "Germany"},
					Geometry:   orb.Polygon{{{6.0, 47.0}, {8.0, 47.0}, {8.0, 49.0}, {6.0, 49.0}, {6.0, 47.0}}},
				},
				{
					Properties: map[string]interface{}{"ISO_A2": "FR", "NAME": "France"},
					Geometry:   orb.Polygon{{{0.0, 40.0}, {5.0, 40.0}, {5.0, 50.0}, {0.0, 50.0}, {0.0, 40.0}}},
				},
			},
		},
	}

	// We are in Germany (48.1, 7.1) but FrenchCity (48.0, 7.0) is the only one in grid
	loc := s.GetLocation(48.1, 7.1)

	if loc.CountryCode != "DE" {
		t.Errorf("Expected legal country 'DE', got %s", loc.CountryCode)
	}
	if loc.CityCountryCode != "FR" {
		t.Errorf("Expected city country 'FR', got %s", loc.CityCountryCode)
	}
	if loc.CityName != "FrenchCity" {
		t.Errorf("Expected city 'FrenchCity', got %s", loc.CityName)
	}
	if loc.CityAdmin1Name != "FrenchRegion" {
		t.Errorf("Expected city region 'FrenchRegion', got %s", loc.CityAdmin1Name)
	}
}

func TestBearing(t *testing.T) {
	tests := []struct {
		name string
		p1   Point
		p2   Point
		want float64
	}{
		{
			name: "North",
			p1:   Point{Lat: 10, Lon: 20},
			p2:   Point{Lat: 11, Lon: 20},
			want: 0,
		},
		{
			name: "East",
			p1:   Point{Lat: 10, Lon: 20},
			p2:   Point{Lat: 10, Lon: 21},
			want: 90,
		},
		{
			name: "South",
			p1:   Point{Lat: 10, Lon: 20},
			p2:   Point{Lat: 9, Lon: 20},
			want: 180,
		},
		{
			name: "West",
			p1:   Point{Lat: 10, Lon: 20},
			p2:   Point{Lat: 10, Lon: 19},
			want: 270,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Bearing(tt.p1, tt.p2)
			if math.Abs(got-tt.want) > 0.1 {
				t.Errorf("Bearing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetLocation_Admin1CountryLock(t *testing.T) {
	s := &Service{
		grid: make(map[int][]City),
	}

	// 1. City in France (The standard "near" city)
	s.grid[s.getGridKey(48, 7)] = []City{{
		Name:        "FrenchCity",
		Lat:         48.0,
		Lon:         7.0,
		CountryCode: "FR",
		Admin1Name:  "Grand Est",
	}}

	// 2. City in Germany (The legal region city, further away)
	s.grid[s.getGridKey(48, 8)] = []City{{
		Name:        "GermanCity",
		Lat:         48.0,
		Lon:         8.0,
		CountryCode: "DE",
		Admin1Name:  "Baden-Württemberg",
	}}

	// Mock CountryService: We are legally in Germany
	s.countrySvc = &CountryService{
		features: &geojson.FeatureCollection{
			Features: []*geojson.Feature{
				{
					Properties: map[string]interface{}{"ISO_A2": "DE", "NAME": "Germany"},
					Geometry:   orb.Polygon{{{6.0, 47.0}, {9.0, 47.0}, {9.0, 49.0}, {6.0, 49.0}, {6.0, 47.0}}},
				},
			},
		},
	}

	// POSITION: 48.05, 7.1 (Very close to FrenchCity, further from GermanCity)
	loc := s.GetLocation(48.05, 7.1)

	// Verification
	if loc.CountryCode != "DE" {
		t.Errorf("Expected legal country 'DE', got %s", loc.CountryCode)
	}
	if loc.CityName != "FrenchCity" {
		t.Errorf("Expected city context 'FrenchCity', got %s", loc.CityName)
	}
	if loc.CityAdmin1Name != "Grand Est" {
		t.Errorf("Expected city admin 'Grand Est', got %s", loc.CityAdmin1Name)
	}

	// CRITICAL FIX CHECK: Admin1Name should be from the German city, not the French one!
	if loc.Admin1Name != "Baden-Württemberg" {
		t.Errorf("Expected legal region 'Baden-Württemberg', got %s", loc.Admin1Name)
	}
}
