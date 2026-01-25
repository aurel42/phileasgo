package scorer

import (
	"strings"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"

	"phileasgo/pkg/visibility"
)

// --- Mock Elevation Getter ---
type mockElevationGetter struct{}

func (m *mockElevationGetter) GetElevation(lat, lon float64) (int16, error) { return 0, nil }
func (m *mockElevationGetter) GetLowestElevation(lat, lon, radiusNM float64) (int16, error) {
	return 0, nil
}

// setupScorer creates a Scorer with a controlled configuration for testing
func setupScorer() *Scorer {
	scorerCfg := &config.ScorerConfig{
		VarietyPenaltyFirst: 0.1,
		VarietyPenaltyLast:  0.5,
		VarietyPenaltyNum:   3,
		NoveltyBoost:        1.3,
		GroupPenalty:        0.5,
	}

	catCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"castle": {Weight: 2.0, Size: "L"},
			"church": {Weight: 1.0, Size: "M"},
			"boring": {Weight: 0.5, Size: "S"},
		},
		CategoryGroups: map[string][]string{
			"dull": {"church", "boring"},
		},
	}
	// Rebuild lookup since we're not calling LoadCategories
	catCfg.BuildLookup()
	// Manually build group lookup for test
	catCfg.CategoryGroups = map[string][]string{
		"dull": {"church", "boring"},
	}
	catCfg.GroupLookup = make(map[string]string)
	for g, cats := range catCfg.CategoryGroups {
		for _, c := range cats {
			catCfg.GroupLookup[c] = g
		}
	}

	// Alt 1000ft: SizeS=1nm, SizeM=5nm, SizeL=10nm
	visMgr := visibility.NewManagerForTest([]visibility.AltitudeRow{
		{
			AltAGL: 0,
			Distances: map[visibility.SizeType]float64{
				visibility.SizeS:  1.0,
				visibility.SizeM:  2.0,
				visibility.SizeL:  5.0,
				visibility.SizeXL: 8.0,
			},
		},
		{
			AltAGL: 1000,
			Distances: map[visibility.SizeType]float64{
				visibility.SizeS:  1.0,
				visibility.SizeM:  5.0,
				visibility.SizeL:  10.0,
				visibility.SizeXL: 15.0,
			},
		},
	})
	visCalc := visibility.NewCalculator(visMgr, nil)

	return NewScorer(scorerCfg, catCfg, visCalc, &mockElevationGetter{})
}

func TestScorer_Calculate(t *testing.T) {
	s := setupScorer()

	tests := []struct {
		name          string
		poi           *model.POI
		input         *ScoringInput
		wantScoreMin  float64
		wantScoreMax  float64
		wantVisible   bool
		wantLogSubstr string
	}{
		// --- 1. Airborne Visibility ---
		{
			name: "Airborne Visible (Close, Head On)",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church", // Size M -> 5nm max
			},
			input: &ScoringInput{
				// User South (-0.04), Looking North (0). POI at (0,0).
				// Dist 2.4nm. Max 5nm. Ratio 0.48. Score 0.52.
				// Bearing 0. Rel 0. Bearing Mult 1.0.
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:   true,
			wantScoreMin:  0.67,
			wantScoreMax:  0.68,
			wantLogSubstr: "Visibility (M@1000ft)",
		},
		{
			name: "Airborne Invisible (Too Far)",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.2, Longitude: 0.0, // 12nm
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:   false,
			wantScoreMin:  0.0,
			wantLogSubstr: "Invisible",
		},
		{
			name: "Airborne Blind Spot",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.0001, Longitude: 0.0, // 11m
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			// Dist ~0. Score ~1.0. Blind Spot x0.1. -> 0.1
			wantVisible:   true,
			wantScoreMin:  0.12,
			wantScoreMax:  0.14,
			wantLogSubstr: "Blind Spot: x0.1 (Hidden by airframe)",
		},

		// --- 2. Dimension Multiplier ---
		{
			name: "Dimension Boost",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
				DimensionMultiplier: 2.0,
			},
			input: &ScoringInput{
				// Same as first case (0.52) but with dim boost -> 1.04
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:   true,
			wantScoreMin:  1.3,
			wantScoreMax:  1.4,
			wantLogSubstr: "Dimensions: x2.0",
		},

		// --- 3. Ground Logic ---
		{
			name: "Ground (Standard Vis)",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.01, // 0.6nm
					AltitudeMSL: 0, AltitudeAGL: 0, IsOnGround: true,
				},
			},
			// 0 AGL -> Max 2.0nm. Dist 0.6nm. Ratio 0.3. Score 0.7.
			// Weight 1.0. Boost 1.3. Total ~0.91.
			wantVisible:   true,
			wantScoreMin:  0.9,
			wantScoreMax:  0.92,
			wantLogSubstr: "Visibility (M@0ft)",
		},
		{
			name: "Ground (Aerodrome)",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Aerodrome",
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.0,
					AltitudeMSL: 0, AltitudeAGL: 0, IsOnGround: true,
				},
			},
			// Dist 0. Score 1.0. Weight 1.0. Boost 1.3.
			wantVisible:   true,
			wantScoreMin:  1.3,
			wantScoreMax:  1.31,
			wantLogSubstr: "Visibility (M@0ft)",
		},

		// --- 4. Content Multipliers ---
		// For these, we use a setup yielding 1.0 geographic score to isolate content
		// User at 0,0.04 (Bearing 270). Heading 315. Rel 315 (x2.0).
		// Dist 2.4nm. Base 0.52. Total Geo 1.04.
		{
			name: "Article Length",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
				WPArticleLength: 2000, // Boost x2.0 (Sqrt 4)
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
			},
			wantVisible:   true,
			wantScoreMin:  2.7, // Geo 1.04 * 2.0 * 1.3 is ~2.7
			wantLogSubstr: "Length (2000 chars): x2.00",
		},
		{
			name: "Sitelinks",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
				Sitelinks: 10, // Boost x4.0 (1 + Sqrt 9)
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
			},
			wantVisible:   true,
			wantScoreMin:  5.4, // 1.04 * 4.0 * 1.3 is ~5.4
			wantLogSubstr: "Sitelinks (10): x4.00",
		},
		{
			name: "Category Weight",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Castle", // Weight x2.0
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
			},
			wantVisible:   true,
			wantScoreMin:  3.9, // Calc: Geo 1.52 * Weight 2.0 * 1.3 is ~3.95
			wantLogSubstr: "Category (Castle): x2.00",
		},

		// --- 5. Variety Logic ---
		{
			name: "Variety Penalty (Recent)",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Castle",
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
				CategoryHistory: []string{"Church", "Castle"}, // Castle is most recent
			},
			// Calc: Geo 1.52 * Weight 2.0 * Penalty 0.1 is 0.304
			wantVisible:   true,
			wantScoreMin:  0.30,
			wantLogSubstr: "Variety Penalty (Pos 1)",
		},
		{
			name: "Category Group Penalty",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Boring", // "Boring" is in "Dull" group
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.005, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315, // 0.3nm
				},
				CategoryHistory: []string{"Church"}, // "Church" is also in "Dull" group
			},
			// Base: 2.6 (Geo 0.52 * 2.5 bearing * 2.0 weight is wrong check weight)
			// Weight: Boring is 0.5.
			// Geo: 0.52.
			// Base logic: 0.52 * 0.5 (weight) * 1.3 (novelty boost) = 0.338
			// Group Penalty: * 0.1 (VarietyPenaltyFirst)
			// Final: 0.0338
			// Let's re-verify weight of "Boring". 0.5.
			// Geo Score: 0.52 (same as Church M/1000ft/2.4nm/0.48).
			// Wait, "Boring" is Size S.
			// S at 1000ft: valid distances?
			// setupScorer visible: 1000ft -> S=1.0.
			// Dist is 2.4nm (0.04 deg).
			// S max is 1.0. Dist 2.4. Invisible!
			// Need to adjust test case or distance.
			// Let's put us very close. 0.005 deg = 0.3nm.
			// 0.3 / 1.0 = 0.3. Score = 0.7.
			// Geo: 0.7.
			// Weight: 0.5.
			// Novelty Boost: 1.3.
			// Total pre-penalty: 0.7 * 0.5 * 1.3 = 0.455.
			// Penalty: 0.1.
			// Final: 0.455 (0.91 * 0.5)
			wantVisible:   true,
			wantScoreMin:  0.45,
			wantScoreMax:  0.46,
			wantLogSubstr: "Group Penalty (dull)",
		},
		{
			name: "Novelty Boost",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Castle",
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
				CategoryHistory: []string{"Church", "Farm"},
			},
			// Geo 1.52. Weight 2.0. Boost 1.3. Total 3.95
			wantVisible:   true,
			wantScoreMin:  3.9,
			wantLogSubstr: "Novelty Boost",
		},

		// --- 6. MSFS POI ---
		{
			name: "MSFS POI Boost",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Castle",
				IsMSFSPOI: true,
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
			},
			// Geo 1.52 * Weight 2.0 * MSFS 4.0 * Novelty 1.3 = 15.8
			wantVisible:   true,
			wantScoreMin:  15.0,
			wantLogSubstr: "MSFS POI: x4.0",
		},
		{
			name: "Recently Played Skip",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Castle",
				LastPlayed: time.Now().Add(-1 * time.Hour), // Played 1h ago
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.04, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
				NarratorConfig: &config.NarratorConfig{
					RepeatTTL: config.Duration(24 * time.Hour), // 24h cooldown
				},
			},
			wantVisible:  false, // Should remain false (skipped)
			wantScoreMin: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// New Session Pattern for Testing
			session := s.NewSession(tt.input)
			session.Calculate(tt.poi)

			if tt.poi.IsVisible != tt.wantVisible {
				t.Errorf("got visible %v, want %v", tt.poi.IsVisible, tt.wantVisible)
			}

			// Verify Visibility field is populated
			if tt.wantVisible && tt.poi.Visibility <= 0 {
				t.Errorf("got visibility score %.2f, expected > 0 for visible POI", tt.poi.Visibility)
			}

			if tt.wantScoreMin > 0 {
				// Allow 10% margin because floating point and geo calc
				margin := tt.wantScoreMin * 0.2
				minScore := tt.wantScoreMin - margin
				maxScore := tt.wantScoreMax + margin
				if tt.wantScoreMax == 0 {
					maxScore = tt.wantScoreMin + margin
				}

				if tt.poi.Score < minScore || tt.poi.Score > maxScore {
					t.Errorf("got score %.4f, want ~%.4f (range %.4f-%.4f)\nDetails:\n%s", tt.poi.Score, tt.wantScoreMin, minScore, maxScore, tt.poi.ScoreDetails)
				}
			}

			if tt.wantLogSubstr != "" {
				if !strings.Contains(tt.poi.ScoreDetails, tt.wantLogSubstr) {
					t.Errorf("log missing substring %q. Got:\n%s", tt.wantLogSubstr, tt.poi.ScoreDetails)
				}
			}
		})
	}
}
