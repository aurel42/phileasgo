package visibility

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCalculator(t *testing.T) {
	// 1. Create Manager with specific test data
	// Alt 1000ft: SizeS=1nm, SizeM=5nm, SizeL=10nm
	// Alt 5000ft: SizeS=5nm, SizeM=25nm...
	manager := NewManagerForTest([]AltitudeRow{
		{AltAGL: 0, Distances: map[SizeType]float64{SizeS: 0, SizeM: 0, SizeL: 0}},
		{AltAGL: 1000, Distances: map[SizeType]float64{SizeS: 1.0, SizeM: 5.0, SizeL: 10.0}},
		{AltAGL: 5000, Distances: map[SizeType]float64{SizeS: 5.0, SizeM: 25.0, SizeL: 50.0}},
	})
	calculator := NewCalculator(manager)

	tests := []struct {
		name       string
		heading    float64
		alt        float64
		effAlt     float64 // Optional, defaults to alt
		bearing    float64
		dist       float64
		size       SizeType
		isOnGround bool
		wantScore  float64 // Expected score (approx)
		wantLog    string  // Substring expected in log
	}{
		// --- 1. Basic Visibility (Distance Decay) ---
		{
			name:    "Invisible (Ground)",
			heading: 0, alt: 0, bearing: 0, dist: 0.1, size: SizeM, isOnGround: true,
			wantScore: 0.0, wantLog: "Invisible (M @ 0ft)",
		},
		{
			name:    "Visible (1000ft, M, 2.5nm - Halfway)",
			heading: 0, alt: 1000, bearing: 315, dist: 2.5, size: SizeM,
			// Max=5nm. Dist=2.5. Ratio=0.5. Base=0.5.
			// Bearing 315 (Left Front) -> x2.0. Result = 1.0
			wantScore: 1.0, wantLog: "x0.50",
		},
		{
			name:    "Invisible (Too Far)",
			heading: 0, alt: 1000, bearing: 0, dist: 6.0, size: SizeM,
			wantScore: 0.0, wantLog: "Too far",
		},
		{
			name:    "Small Object (S) - Closer Limits",
			heading: 0, alt: 1000, bearing: 315, dist: 2.0, size: SizeS,
			// Max=1nm. Dist=2.0. Invisible.
			wantScore: 0.0, wantLog: "Too far",
		},

		// --- 2. Bearing Logic ---
		{
			name:    "Right Front (x1.0)",
			heading: 0, alt: 1000, bearing: 45, dist: 1.0, size: SizeM,
			// Base(dist 1/5) = 0.8. Mult 1.0. = 0.8
			wantScore: 0.8, wantLog: "", // 1.0 multiplier is not logged
		},
		{
			name:    "Rear (x0.0 Invisible)",
			heading: 0, alt: 1000, bearing: 180, dist: 1.0, size: SizeM,
			// Base 0.8. Mult 0.0. = 0.0 (Invisible behind aircraft)
			wantScore: 0.0, wantLog: "Rear",
		},
		{
			name:    "Left Front (Best x2.0)",
			heading: 0, alt: 1000, bearing: 315, dist: 1.0, size: SizeM,
			// Base 0.8. Mult 2.0. = 1.6
			wantScore: 1.6, wantLog: "Left Front",
		},

		// --- 3. Blind Spot ---
		{
			name:    "Blind Spot (Under Nose)",
			heading: 0, alt: 1000, bearing: 0, dist: 0.05, size: SizeM,
			// 1000ft -> BlindRadius ~ 0.07nm (New Formula). 0.05 < 0.07.
			// Penalty x0.1
			wantScore: 0.099, // Base ~0.99 * 0.1
			wantLog:   "Blind Spot",
		},

		// --- 4. Valley Boost ---
		{
			name:    "Valley Boost (Low Real, High Eff)",
			heading: 0, alt: 100, effAlt: 2000, bearing: 315, dist: 2.0, size: SizeM,
			// Real: 100ft. Interp 0-1000.  0->0, 1000->5. 100->0.5nm.
			// Dist 2.0 > 0.5. Invisible real.
			// Eff: 2000ft. Interp 1000-5000. 1000->5, 5000->25.
			// 2000 is 1/4 way. 5 + 5 = 10nm.
			// Max=10nm. Dist=2.0. Ratio=0.2. Base=0.8.
			// Bearing x2.0. Total 1.6.
			wantScore: 1.6,
			wantLog:   "Valley Boost Applied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eff := tt.effAlt
			if eff == 0 {
				eff = tt.alt
			}
			score, details := calculator.CalculatePOIScore(tt.heading, tt.alt, eff, tt.bearing, tt.dist, tt.size, tt.isOnGround, 1.0)

			// Fuzzy match score
			diff := math.Abs(score - tt.wantScore)
			if diff > 0.05 {
				t.Errorf("score %.3f, want %.3f. Details: %s", score, tt.wantScore, details)
			}

			if tt.wantLog != "" && !contains(details, tt.wantLog) {
				t.Errorf("log missing %q. Got: %s", tt.wantLog, details)
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestConfigLoader(t *testing.T) {
	// Create temp file
	content := []byte(`
visibility:
  table:
    - altitude: 0
      distances: { S: 0, M: 0, L: 0 }
    - altitude: 1000
      distances: { S: 1, M: 5, L: 10 }
`)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "visibility.yaml")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	m, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	dist := m.GetMaxVisibleDist(500, SizeM, 1.0)
	if dist < 2.0 || dist > 3.0 {
		t.Errorf("Interpolation 500ft failed. Expected 2.5, got %f", dist)
	}
}

// TestCalculator_BlindSpot tests the blind spot logic
func TestCalculator_BlindSpot(t *testing.T) {

	tests := []struct {
		name       string
		altAGL     float64
		distNM     float64
		relBearing float64
		wantBlind  bool
	}{
		// 1. Below 500ft: No blind spot
		{"Low Alt (100ft), Close (0.01nm)", 100, 0.01, 0, false},
		{"Low Alt (499ft), Close (0.01nm)", 499, 0.01, 0, false},

		// 2. Transition at 500ft (starts at 0nm)
		{"500ft, 0.01nm", 500, 0.01, 0, false}, // 0 radius

		// 3. Mid-Altitude Scaling
		// Formula: (Alt - 500) / 34500 * 5.0
		// At 17750ft (midpoint): Radius should be ~2.5nm
		{"Mid Alt (17750ft), Inside (2.4nm)", 17750, 2.4, 0, true},
		{"Mid Alt (17750ft), Outside (2.6nm)", 17750, 2.6, 0, false},

		// 4. High Altitude Cap (35,000ft -> 5.0nm)
		{"High Alt (35000ft), Inside (4.9nm)", 35000, 4.9, 0, true},
		{"High Alt (35000ft), Outside (5.1nm)", 35000, 5.1, 0, false},

		// 5. Above Cap (>35,000ft -> Still 5.0nm)
		{"Orbit (60000ft), Inside (4.9nm)", 60000, 4.9, 0, true},
		{"Orbit (60000ft), Outside (5.1nm)", 60000, 5.1, 0, false},

		// 6. Bearing Check (+/- 90 deg)
		{"Bearing Side (91 deg)", 35000, 1.0, 91, false},
		{"Bearing Rear (180 deg)", 35000, 1.0, 180, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBlindSpot(tt.altAGL, tt.distNM, tt.relBearing); got != tt.wantBlind {
				t.Errorf("isBlindSpot(%.0f, %.2f, %.0f) = %v, want %v",
					tt.altAGL, tt.distNM, tt.relBearing, got, tt.wantBlind)
			}
		})
	}
}

// TestCalculateVisibilityForSize tests size-specific visibility
func TestCalculateVisibilityForSize(t *testing.T) {
	manager := NewManagerForTest([]AltitudeRow{
		{AltAGL: 0, Distances: map[SizeType]float64{SizeS: 0, SizeM: 0, SizeL: 0, SizeXL: 0}},
		{AltAGL: 1000, Distances: map[SizeType]float64{SizeS: 1.0, SizeM: 5.0, SizeL: 10.0, SizeXL: 20.0}},
	})
	calculator := NewCalculator(manager)

	tests := []struct {
		name      string
		dist      float64
		size      SizeType
		wantScore float64
	}{
		{"S close", 0.5, SizeS, 0.5},
		{"S too far", 2.0, SizeS, 0.0},
		{"M close", 2.5, SizeM, 0.5},
		{"L close", 5.0, SizeL, 0.5},
		{"XL close", 10.0, SizeXL, 0.5},
		{"XL at max", 20.0, SizeXL, 0.0}, // Exactly at max = invisible
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculator.CalculateVisibilityForSize(0, 1000, 1000, 45, tt.dist, tt.size, false, 1.0)
			diff := math.Abs(score - tt.wantScore)
			if diff > 0.1 {
				t.Errorf("got %.3f, want %.3f", score, tt.wantScore)
			}
		})
	}
}

// TestGetMaxVisibleDist tests interpolation edge cases
func TestGetMaxVisibleDist(t *testing.T) {
	manager := NewManagerForTest([]AltitudeRow{
		{AltAGL: 1000, Distances: map[SizeType]float64{SizeS: 1.0, SizeM: 5.0, SizeL: 10.0}},
		{AltAGL: 5000, Distances: map[SizeType]float64{SizeS: 5.0, SizeM: 25.0, SizeL: 50.0}},
	})

	tests := []struct {
		name string
		alt  float64
		size SizeType
		want float64
	}{
		{"Below table floor", 0, SizeM, 5.0},            // Returns first row
		{"At first row", 1000, SizeM, 5.0},              // Exact match
		{"At last row", 5000, SizeM, 25.0},              // Exact match
		{"Above table ceiling", 10000, SizeM, 25.0},     // Returns last row
		{"Mid interpolation", 3000, SizeM, 15.0},        // 5+(25-5)*0.5=15
		{"Missing size fallback", 1000, "INVALID", 5.0}, // Falls back to M
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := manager.GetMaxVisibleDist(tt.alt, tt.size, 1.0)
			diff := math.Abs(dist - tt.want)
			if diff > 0.1 {
				t.Errorf("got %.2f, want %.2f", dist, tt.want)
			}
		})
	}
}

// TestNormalizeBearing tests bearing normalization
func TestNormalizeBearing(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0, 0},
		{90, 90},
		{180, 180},
		{181, -179},
		{270, -90},
		{360, 0},
		{-90, -90},
		{-181, 179},
		{540, 180},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0f", tt.input), func(t *testing.T) {
			got := normalizeBearing(tt.input)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("normalizeBearing(%.0f) = %.1f, want %.1f", tt.input, got, tt.want)
			}
		})
	}
}

// TestGetBearingMultiplier tests all bearing sectors
func TestGetBearingMultiplier(t *testing.T) {
	tests := []struct {
		relBearing float64
		want       float64
		desc       string
	}{
		{45, 1.0, "Right Front"},
		{0, 1.0, "Right Front (straight ahead)"},
		{135, 0.0, "Rear"},
		{180, 0.0, "Rear"},
		{-135, 0.5, "Left Rear"},   // 225 (225-270 range)
		{-100, 0.5, "Left Rear"},   // 260 (225-270 range)
		{-80, 1.5, "Left Side"},    // 280 (270-300 range)
		{-45, 2.0, "Left Front"},   // 315 (300-330 range)
		{-15, 1.5, "Forward Left"}, // 345 (330+ range)
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := getBearingMultiplier(tt.relBearing)
			if got != tt.want {
				t.Errorf("getBearingMultiplier(%.0f) = %.1f, want %.1f", tt.relBearing, got, tt.want)
			}
		})
	}
}

// TestEmptyTable tests behavior with empty visibility table
func TestEmptyTable(t *testing.T) {
	manager := NewManagerForTest([]AltitudeRow{})
	dist := manager.GetMaxVisibleDist(1000, SizeM, 1.0)
	if dist != 0 {
		t.Errorf("Expected 0 for empty table, got %f", dist)
	}
}

func TestVisibilityBoost(t *testing.T) {
	// Setup: Alt 1000ft -> Max Dist 5nm for SizeM
	manager := NewManagerForTest([]AltitudeRow{
		{AltAGL: 1000, Distances: map[SizeType]float64{SizeS: 1.0, SizeM: 5.0, SizeL: 10.0}},
	})
	calculator := NewCalculator(manager)

	// Case 1: No Boost (1.0). Object at 6nm should be invisible (Max 5nm)
	// Score calculation: 5nm max. 6nm dist. Ratio > 1.0 -> 0 score.
	scoreNoBoost := calculator.CalculateVisibility(0, 1000, 315, 6.0, false, 1.0)
	if scoreNoBoost > 0 {
		t.Errorf("Expected score 0 for 6nm dist (max 5nm), got %.2f", scoreNoBoost)
	}

	// Case 2: Boost 1.5x. Max Dist becomes 5 * 1.5 = 7.5nm.
	// Object at 6nm is now visible.
	// Calculation: Ratio = 6.0 / 7.5 = 0.8.
	// Base Score = 1.0 - 0.8 = 0.2.
	// Bearing Multiplier (315/LeftFront) = 2.0.
	// Final Score = 0.2 * 2.0 = 0.4.
	scoreBoost := calculator.CalculateVisibility(0, 1000, 315, 6.0, false, 1.5)
	if scoreBoost <= 0 {
		t.Errorf("Expected positive score with boost, got %.2f", scoreBoost)
	}
	if math.Abs(scoreBoost-0.4) > 0.05 {
		t.Errorf("Expected approx 0.4 score, got %.2f", scoreBoost)
	}

	// Verify GetMaxVisibleDist respects boost
	maxDist := manager.GetMaxVisibleDist(1000, SizeM, 1.5)
	if maxDist != 7.5 {
		t.Errorf("Expected max dist 7.5 with boost, got %.2f", maxDist)
	}
}
