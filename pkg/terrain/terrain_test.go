package terrain

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"phileasgo/pkg/geo"
)

// --- ElevationProvider Tests ---

func TestElevationProvider_GetElevation(t *testing.T) {
	// Skip if ETOPO1 file not available (CI environments)
	path := "../../data/etopo1/etopo1_ice_g_i2.bin"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ETOPO1 data file not available")
	}

	provider, err := NewElevationProvider(path)
	if err != nil {
		t.Fatalf("Failed to open ETOPO1: %v", err)
	}
	defer provider.Close()

	tests := []struct {
		name      string
		lat       float64
		lon       float64
		wantMin   int16 // Allow range for elevation validation
		wantMax   int16
		wantError bool
	}{
		{
			name:    "Mount Everest",
			lat:     27.9881,
			lon:     86.9250,
			wantMin: 8000, // ~8849m
			wantMax: 9000,
		},
		{
			name:    "Dead Sea",
			lat:     31.5,
			lon:     35.5,
			wantMin: -500, // ~-430m
			wantMax: 0,
		},
		{
			name:    "Pacific Ocean",
			lat:     0.0,
			lon:     -140.0,
			wantMin: -6000,
			wantMax: -3000,
		},
		{
			name:    "North Pole (Arctic Ocean)",
			lat:     90.0,
			lon:     0.0,
			wantMin: -5000, // Arctic Ocean
			wantMax: 0,
		},
		{
			name:    "South Pole",
			lat:     -90.0,
			lon:     0.0,
			wantMin: 2000, // Antarctic plateau ~2800m
			wantMax: 3500,
		},
		{
			name:      "Invalid Lat High",
			lat:       91.0,
			lon:       0.0,
			wantError: true,
		},
		{
			name:      "Invalid Lat Low",
			lat:       -91.0,
			lon:       0.0,
			wantError: true,
		},
		{
			name:      "Invalid Lon High",
			lat:       0.0,
			lon:       181.0,
			wantError: true,
		},
		{
			name:      "Invalid Lon Low",
			lat:       0.0,
			lon:       -181.0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elev, err := provider.GetElevation(tt.lat, tt.lon)
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if elev < tt.wantMin || elev > tt.wantMax {
				t.Errorf("Elevation %d not in expected range [%d, %d]", elev, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestElevationProvider_InvalidFile(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "NonExistent",
			path:    "/nonexistent/path/file.bin",
			wantErr: true,
		},
		{
			name:    "WrongSize",
			path:    createTempFile(t, 1024), // Too small
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewElevationProvider(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewElevationProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- LOSChecker Tests ---

func TestLOSChecker_IsVisible_NoData(t *testing.T) {
	// With nil elevation provider, should always return true (fail open)
	checker := NewLOSChecker(nil)

	p1 := geo.Point{Lat: 48.0, Lon: -122.0}
	p2 := geo.Point{Lat: 48.1, Lon: -122.1}

	if !checker.IsVisible(p1, p2, 10000, 1000, 2.0) {
		t.Error("Expected true when no elevation data available")
	}
}

func TestLOSChecker_IsVisible_ClosePoints(t *testing.T) {
	// Points closer than step size should always be visible
	checker := NewLOSChecker(nil)

	p1 := geo.Point{Lat: 48.0, Lon: -122.0}
	p2 := geo.Point{Lat: 48.001, Lon: -122.001} // ~150m apart

	if !checker.IsVisible(p1, p2, 10000, 0, 2.0) {
		t.Error("Close points should always be visible")
	}
}

func TestLOSChecker_GetElevation_NoData(t *testing.T) {
	checker := NewLOSChecker(nil)

	elev, err := checker.GetElevation(48.0, -122.0)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if elev != 0 {
		t.Errorf("Expected 0 elevation when no data, got: %f", elev)
	}
}

func TestLOSChecker_GetElevation_WithData(t *testing.T) {
	path := "../../data/etopo1/etopo1_ice_g_i2.bin"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ETOPO1 data file not available")
	}

	provider, err := NewElevationProvider(path)
	if err != nil {
		t.Fatalf("Failed to open ETOPO1: %v", err)
	}
	defer provider.Close()

	checker := NewLOSChecker(provider)

	// Test Mount Everest
	elev, err := checker.GetElevation(27.9881, 86.9250)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if elev < 8000 || elev > 9000 {
		t.Errorf("Expected Everest elevation ~8849m, got: %f", elev)
	}
}

func TestElevationProvider_GetElevation_EdgeCases(t *testing.T) {
	path := "../../data/etopo1/etopo1_ice_g_i2.bin"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ETOPO1 data file not available")
	}

	provider, err := NewElevationProvider(path)
	if err != nil {
		t.Fatalf("Failed to open ETOPO1: %v", err)
	}
	defer provider.Close()

	tests := []struct {
		name string
		lat  float64
		lon  float64
	}{
		{"Prime Meridian", 0.0, 0.0},
		{"Date Line West", 0.0, -180.0},
		{"Date Line East", 0.0, 180.0},
		{"Near South Pole", -89.99, 0.0},
		{"Near North Pole", 89.99, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.GetElevation(tt.lat, tt.lon)
			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}

// Integration test - requires ETOPO1 data
func TestLOSChecker_IsVisible_Integration(t *testing.T) {
	path := "../../data/etopo1/etopo1_ice_g_i2.bin"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ETOPO1 data file not available")
	}

	provider, err := NewElevationProvider(path)
	if err != nil {
		t.Fatalf("Failed to open ETOPO1: %v", err)
	}
	defer provider.Close()

	checker := NewLOSChecker(provider)

	tests := []struct {
		name    string
		p1      geo.Point
		alt1Ft  float64
		p2      geo.Point
		alt2Ft  float64
		stepKM  float64
		wantVis bool
	}{
		{
			name:    "High Altitude Clear",
			p1:      geo.Point{Lat: 48.0, Lon: -122.0},
			alt1Ft:  35000, // 35,000 ft
			p2:      geo.Point{Lat: 48.5, Lon: -122.5},
			alt2Ft:  0,
			stepKM:  2.0,
			wantVis: true, // High altitude should see everything
		},
		{
			name:    "Low Altitude Flat Terrain",
			p1:      geo.Point{Lat: 52.0, Lon: 5.0}, // Netherlands (flat)
			alt1Ft:  1000,
			p2:      geo.Point{Lat: 52.1, Lon: 5.1},
			alt2Ft:  0,
			stepKM:  1.0,
			wantVis: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.IsVisible(tt.p1, tt.p2, tt.alt1Ft, tt.alt2Ft, tt.stepKM)
			if got != tt.wantVis {
				t.Errorf("IsVisible() = %v, want %v", got, tt.wantVis)
			}
		})
	}
}

// Helper to create temp file with specific size
func createTempFile(t *testing.T, size int) string {
	t.Helper()
	f, err := os.CreateTemp("", "etopo_test_*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := f.Truncate(int64(size)); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Remove(f.Name()) })
	return filepath.Clean(f.Name())
}

// --- GetLowestElevation Tests ---

func TestElevationProvider_GetLowestElevation(t *testing.T) {
	path := "../../data/etopo1/etopo1_ice_g_i2.bin"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ETOPO1 data file not available")
	}

	provider, err := NewElevationProvider(path)
	if err != nil {
		t.Fatalf("Failed to open ETOPO1: %v", err)
	}
	defer provider.Close()

	// Helper to convert KM to NM - REMOVED

	tests := []struct {
		name        string
		lat         float64
		lon         float64
		radiusNM    float64
		wantMinLow  int16 // Lower bound of expected min elevation
		wantMinHigh int16 // Upper bound of expected min elevation
	}{
		{
			name:        "Dead Sea (Capped at MSL)",
			lat:         31.7683, // Jerusalem
			lon:         35.2137,
			radiusNM:    50,
			wantMinLow:  0, // Capped at 0
			wantMinHigh: 0,
		},
		{
			name:        "Pacific Ocean (Capped at MSL)",
			lat:         0.0,
			lon:         -140.0,
			radiusNM:    50,
			wantMinLow:  0,
			wantMinHigh: 0,
		},
		{
			name:        "Pole Proximity (Arctic Ocean)",
			lat:         89.0, // Near North Pole
			lon:         0.0,
			radiusNM:    100,
			wantMinLow:  0,
			wantMinHigh: 0,
		},
		{
			name:        "Date Line Crossing (Ocean)",
			lat:         0.0,
			lon:         179.5, // Close to 180
			radiusNM:    60,
			wantMinLow:  0,
			wantMinHigh: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minElev, err := provider.GetLowestElevation(tt.lat, tt.lon, tt.radiusNM)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if minElev < tt.wantMinLow || minElev > tt.wantMinHigh {
				t.Errorf("GetLowestElevation = %d, want between [%d, %d]", minElev, tt.wantMinLow, tt.wantMinHigh)
			}
		})
	}
}

func TestElevationProvider_GetLowestElevation_Performance_Everest(t *testing.T) {
	path := "../../data/etopo1/etopo1_ice_g_i2.bin"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("ETOPO1 data file not available")
	}

	provider, err := NewElevationProvider(path)
	if err != nil {
		t.Fatalf("Failed to open ETOPO1: %v", err)
	}
	defer provider.Close()

	lat := 27.9881 // Everest
	lon := 86.9250

	// Radius in NM
	radiiNM := []float64{10, 20, 30, 40, 50}

	for _, rNM := range radiiNM {
		start := time.Now()
		minElev, err := provider.GetLowestElevation(lat, lon, rNM)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Error for radius %v NM: %v", rNM, err)
		}

		t.Logf("Radius %2.0f NM: MinElev = %5d m, Time = %v", rNM, minElev, duration)

		// Performance Assertion: Should be under 100ms (generous budget, aiming for <10ms)
		if duration > 100*time.Millisecond {
			t.Errorf("Performance Warning: Radius %.0f NM took %v (limit 100ms)", rNM, duration)
		}
	}
}
