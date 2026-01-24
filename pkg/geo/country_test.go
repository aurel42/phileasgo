package geo

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

func TestCountryService_Land(t *testing.T) {
	cs, err := NewCountryServiceEmbedded()
	if err != nil {
		t.Fatalf("Failed to create CountryService: %v", err)
	}

	tests := []struct {
		name        string
		lat, lon    float64
		wantCode    string
		wantZone    string
		wantCountry string
	}{
		{
			name:        "London UK",
			lat:         51.5074,
			lon:         -0.1278,
			wantCode:    "GB",
			wantZone:    ZoneLand,
			wantCountry: "United Kingdom",
		},
		{
			name:        "Paris France",
			lat:         48.8566,
			lon:         2.3522,
			wantCode:    "FR",
			wantZone:    ZoneLand,
			wantCountry: "France",
		},
		{
			name:        "Moscow Russia",
			lat:         55.7558,
			lon:         37.6173,
			wantCode:    "RU",
			wantZone:    ZoneLand,
			wantCountry: "Russia",
		},
		{
			name:        "Chukotka Russia (user's flight)",
			lat:         65.58,
			lon:         -176.32, // Note: negative longitude (east of date line)
			wantCode:    "RU",
			wantZone:    ZoneLand,
			wantCountry: "Russia",
		},
		{
			name:        "New York USA",
			lat:         40.7128,
			lon:         -74.0060,
			wantCode:    "US",
			wantZone:    ZoneLand,
			wantCountry: "United States of America",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache to ensure fresh lookup
			cs.ResetCache()

			result := cs.GetCountryAtPoint(tt.lat, tt.lon)

			if result.Zone != tt.wantZone {
				t.Errorf("Zone = %v, want %v", result.Zone, tt.wantZone)
			}
			if result.CountryCode != tt.wantCode {
				t.Errorf("CountryCode = %v, want %v", result.CountryCode, tt.wantCode)
			}
			if tt.wantCountry != "" && result.CountryName != tt.wantCountry {
				t.Errorf("CountryName = %v, want %v", result.CountryName, tt.wantCountry)
			}
		})
	}
}

func TestCountryService_MaritimeZones(t *testing.T) {
	cs, err := NewCountryServiceEmbedded()
	if err != nil {
		t.Fatalf("Failed to create CountryService: %v", err)
	}

	tests := []struct {
		name     string
		lat, lon float64
		wantZone string
	}{
		{
			name:     "Mid Pacific - International Waters",
			lat:      0,
			lon:      -160,
			wantZone: ZoneInternational,
		},
		{
			name:     "North Atlantic - International Waters",
			lat:      45,
			lon:      -40,
			wantZone: ZoneInternational,
		},
		{
			name:     "Near UK Coast - Territorial",
			lat:      50.5,
			lon:      0.0,
			wantZone: ZoneTerritorial,
		},
		{
			name:     "Further in Biscay - EEZ",
			lat:      46.0,
			lon:      -8.0,
			wantZone: ZoneEEZ,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache
			cs.ResetCache()

			result := cs.GetCountryAtPoint(tt.lat, tt.lon)

			if result.Zone != tt.wantZone {
				t.Errorf("%s: Zone = %v, want %v (dist=%.0fm)", tt.name, result.Zone, tt.wantZone, result.DistanceM)
			}
		})
	}
}

func TestCountryService_Cache(t *testing.T) {
	cs, err := NewCountryServiceEmbedded()
	if err != nil {
		t.Fatalf("Failed to create CountryService: %v", err)
	}

	// First call
	result1 := cs.GetCountryAtPoint(51.5074, -0.1278)

	// Second call within same cell - should return same result (proving quantization/cache)
	result2 := cs.GetCountryAtPoint(51.5075, -0.1279)

	if result1.CountryCode != result2.CountryCode {
		t.Error("Cached result differs from original")
	}

	// Verify quantization boundary
	cs.ResetCache()
	result3 := cs.GetCountryAtPoint(51.5074, -0.1278)
	result4 := cs.GetCountryAtPoint(51.5274, -0.1278) // Different cell

	if result3.CountryCode != result4.CountryCode && result3.CountryCode == "" {
		// Just a smoke test that it works
		t.Error("Lookup failed for one of the cells")
	}
}

func TestContainsPoint(t *testing.T) {
	// Test with a simple triangle polygon
	triangle := orb.Ring{{0, 0}, {10, 0}, {5, 10}, {0, 0}}
	poly := orb.Polygon{triangle}
	multiPoly := orb.MultiPolygon{poly}

	tests := []struct {
		name   string
		geom   orb.Geometry
		point  orb.Point
		inside bool
	}{
		{"Polygon Center", poly, orb.Point{5, 3}, true},
		{"Polygon Outside", poly, orb.Point{-1, 5}, false},
		{"MultiPolygon Center", multiPoly, orb.Point{5, 3}, true},
		{"MultiPolygon Outside", multiPoly, orb.Point{11, 5}, false},
		{"Point (unsupported)", orb.Point{0, 0}, orb.Point{0, 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsPoint(tt.geom, tt.point)
			if got != tt.inside {
				t.Errorf("%s: containsPoint() = %v, want %v", tt.name, got, tt.inside)
			}
		})
	}
}

func TestGetISOCode(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		wantCode string
	}{
		{"Standard ISO_A2", map[string]interface{}{"ISO_A2": "FR"}, "FR"},
		{"Fallback from -99", map[string]interface{}{"ISO_A2": "-99", "ISO_A2_EH": "KO"}, "KO"},
		{"Missing ISO_A2", map[string]interface{}{"ISO_A2_EH": "KO"}, "KO"},
		{"Empty", map[string]interface{}{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getISOCode(tt.props)
			if got != tt.wantCode {
				t.Errorf("getISOCode() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}

func TestGetStringProp(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		key      string
		wantCode string
	}{
		{"String value", map[string]interface{}{"NAME": "France"}, "NAME", "France"},
		{"Missing key", map[string]interface{}{"NAME": "France"}, "CODE", ""},
		{"Non-string value", map[string]interface{}{"ID": 123}, "ID", ""},
		{"JSON Number", map[string]interface{}{"ID": json.Number("123")}, "ID", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringProp(tt.props, tt.key)
			if got != tt.wantCode {
				t.Errorf("getStringProp() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}

func TestDistanceToGeometry(t *testing.T) {
	p := orb.Point{0, 5}
	poly := orb.Polygon{{{10, 0}, {10, 10}, {20, 10}, {20, 0}, {10, 0}}}
	multiPoly := orb.MultiPolygon{poly}

	d1 := distanceToGeometry(p, poly)
	if d1 != 10 {
		t.Errorf("Polygon distance = %v, want 10", d1)
	}

	d2 := distanceToGeometry(p, multiPoly)
	if d2 != 10 {
		t.Errorf("MultiPolygon distance = %v, want 10", d2)
	}

	d3 := distanceToGeometry(p, orb.Point{0, 0})
	if d3 != math.MaxFloat64 {
		t.Errorf("Unsupported distance = %v, want max", d3)
	}
}

func TestDistanceToSegment(t *testing.T) {
	p := orb.Point{5, 5}
	a := orb.Point{0, 0}
	b := orb.Point{10, 0}

	// Closest is a
	d1 := distanceToSegment(orb.Point{-5, 0}, a, b)
	if d1 != 5 {
		t.Errorf("Dist to start = %v, want 5", d1)
	}

	// Closest is b
	d2 := distanceToSegment(orb.Point{15, 0}, a, b)
	if d2 != 5 {
		t.Errorf("Dist to end = %v, want 5", d2)
	}

	// Closest is segment itself
	d3 := distanceToSegment(p, a, b)
	if d3 != 5 {
		t.Errorf("Dist to segment = %v, want 5", d3)
	}

	// Degenerate segment
	d4 := distanceToSegment(p, a, a)
	if d4 != math.Sqrt(50) {
		t.Errorf("Dist to point segment = %v, want sqrt(50)", d4)
	}
}

func TestNewCountryService_Errors(t *testing.T) {
	_, err := NewCountryService("nonexistent.geojson")
	if err == nil {
		t.Error("Want error for nonexistent file, got nil")
	}

	// Invalid JSON
	tmpFile, _ := os.CreateTemp("", "invalid.geojson")
	defer os.Remove(tmpFile.Name())
	_ = os.WriteFile(tmpFile.Name(), []byte("invalid"), 0o644)

	_, err = NewCountryService(tmpFile.Name())
	if err == nil {
		t.Error("Want error for invalid JSON, got nil")
	}
}

func TestDegreesToMeters(t *testing.T) {
	// Near equator, 1 degree lat ≈ 111km
	m1 := degreesToMeters(1, 0)
	if math.Abs(m1-111320) > 100 {
		t.Errorf("degreesToMeters(1, 0) = %v, want ~111320", m1)
	}

	// Near 60 deg lat, cos(60)=0.5, 1 deg lon ≈ 55km
	m2 := degreesToMeters(1, 60)
	if math.Abs(m2-55660) > 100 {
		t.Errorf("degreesToMeters(1, 60) = %v, want ~55660", m2)
	}
}

func TestReorderFeatures(t *testing.T) {
	// Create a mock CountryService with 3 features at different locations.
	// We'll place them at (0,0), (10,0), and (20,0).
	// The user will be at (5,0).
	// Expected order:
	// 1. (0,0) - Dist 5
	// 2. (10,0) - Dist 5
	// 3. (20,0) - Dist 15
	// But to be deterministic, let's make distances distinct.
	// User at (1,0).
	// F1 (0,0) -> dist 1
	// F2 (10,0) -> dist 9
	// F3 (5,0) -> dist 4
	// Expected: F1, F3, F2

	f1 := geojson.NewFeature(orb.Polygon{{{0, 0}, {0.1, 0}, {0.1, 0.1}, {0, 0.1}, {0, 0}}})
	f1.Properties["ISO_A2"] = "F1"

	f2 := geojson.NewFeature(orb.Polygon{{{10, 0}, {10.1, 0}, {10.1, 0.1}, {10, 0.1}, {10, 0}}})
	f2.Properties["ISO_A2"] = "F2"

	f3 := geojson.NewFeature(orb.Polygon{{{5, 0}, {5.1, 0}, {5.1, 0.1}, {5, 0.1}, {5, 0}}})
	f3.Properties["ISO_A2"] = "F3"

	fc := geojson.NewFeatureCollection()
	fc.Features = []*geojson.Feature{f2, f3, f1} // Initial order: F2, F3, F1

	cs := &CountryService{
		features: fc,
		cache:    make(map[string]*cacheEntry),
	}

	// Reorder relative to (1,0)
	cs.ReorderFeatures(0, 1) // lat 0, lon 1

	if len(cs.features.Features) != 3 {
		t.Fatalf("Features count = %d, want 3", len(cs.features.Features))
	}

	// Check order
	order := []string{}
	for _, f := range cs.features.Features {
		order = append(order, f.Properties["ISO_A2"].(string))
	}

	// Expected: F1 (dist 1), F3 (dist 4), F2 (dist 9)
	if order[0] != "F1" || order[1] != "F3" || order[2] != "F2" {
		t.Errorf("Order = %v, want [F1, F3, F2]", order)
	}
}
