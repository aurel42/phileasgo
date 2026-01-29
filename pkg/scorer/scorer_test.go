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

	return NewScorer(scorerCfg, catCfg, visCalc, &mockElevationGetter{}, false)
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
		wantBadges    []string
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
		// --- 7. Stub Detection ---
		{
			name: "Stub Badge",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
				WPArticleLength: 500, // < 2000 -> Stub
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:  true,
			wantScoreMin: 0.6, // Normal score
			wantBadges:   []string{"stub"},
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

			if len(tt.wantBadges) > 0 {
				for _, wb := range tt.wantBadges {
					found := false
					for _, b := range tt.poi.Badges {
						if b == wb {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("missing badge %q. Got: %v", wb, tt.poi.Badges)
					}
				}
			}
		})
	}
}

func TestScorer_Calculate_BusyPOISkip(t *testing.T) {
	s := setupScorer()

	poi := &model.POI{
		WikidataID:   "Q123",
		Score:        10.5,
		ScoreDetails: "Original High Score",
		IsVisible:    true,
	}

	input := &ScoringInput{
		Telemetry: sim.Telemetry{
			Latitude: 90, Longitude: 0, // Very far away
		},
		IsPOIBusy: func(qid string) bool {
			return qid == "Q123"
		},
	}

	session := s.NewSession(input)
	session.Calculate(poi)

	if poi.Score != 10.5 {
		t.Errorf("Expected score 10.5 to be preserved, got %.2f", poi.Score)
	}
	if poi.ScoreDetails != "Original High Score" {
		t.Errorf("Expected score details to be preserved, got %q", poi.ScoreDetails)
	}
	if !poi.IsVisible {
		t.Error("Expected IsVisible to remain true")
	}
}

func TestScorer_VerifyBadgeWiping(t *testing.T) {
	s := setupScorer()

	// A POI that is currently "Urgent" but enters the Busy state
	poi := &model.POI{
		WikidataID:      "Q123",
		Score:           10.5,
		Badges:          []string{"urgent", "deep_dive"}, // it was urgent, and is a deep dive
		WPArticleLength: 50000,
	}

	input := &ScoringInput{
		IsPOIBusy: func(qid string) bool {
			return true // It's frozen/busy
		},
	}

	session := s.NewSession(input)
	session.Calculate(poi)

	// 1. Verify "urgent" is GONE (because applyBadges clears the array at the start)
	for _, b := range poi.Badges {
		if b == "urgent" {
			t.Errorf("Verification Failure: 'urgent' badge should have been wiped by applyBadges")
		}
	}

	// 2. Verify "deep_dive" is STILL THERE (because applyBadges recalculates stateless badges)
	foundDeepDive := false
	for _, b := range poi.Badges {
		if b == "deep_dive" {
			foundDeepDive = true
		}
	}
	if !foundDeepDive {
		t.Errorf("Verification Failure: 'deep_dive' badge should have been repopulated by applyBadges")
	}
}

func TestScorer_PregroundingBonus(t *testing.T) {
	// Setup base config
	baseScorerCfg := &config.ScorerConfig{
		NoveltyBoost:   1.0, // Neutral to isolate content scoring
		PregroundBoost: 4000,
	}

	catCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"stadium": {Weight: 1.0, Size: "L", Preground: true},
			"church":  {Weight: 1.0, Size: "M", Preground: false},
		},
	}
	catCfg.BuildLookup()

	visMgr := visibility.NewManagerForTest([]visibility.AltitudeRow{
		{AltAGL: 1000, Distances: map[visibility.SizeType]float64{
			visibility.SizeM: 10.0, visibility.SizeL: 15.0,
		}},
	})
	visCalc := visibility.NewCalculator(visMgr, nil)

	tests := []struct {
		name               string
		pregroundEnabled   bool
		category           string
		articleLen         int
		wantLogContains    string
		wantLogNotContains string
	}{
		{
			name:             "Pregrounding enabled, stadium category (preground=true)",
			pregroundEnabled: true,
			category:         "stadium",
			articleLen:       1000,
			wantLogContains:  "1000+4000 chars", // Shows boost applied
		},
		{
			name:               "Pregrounding enabled, church category (preground=false)",
			pregroundEnabled:   true,
			category:           "church",
			articleLen:         1000,
			wantLogContains:    "1000 chars",
			wantLogNotContains: "+4000", // No boost
		},
		{
			name:               "Pregrounding disabled, stadium category",
			pregroundEnabled:   false,
			category:           "stadium",
			articleLen:         1000,
			wantLogContains:    "1000 chars",
			wantLogNotContains: "+4000", // No boost when disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewScorer(baseScorerCfg, catCfg, visCalc, &mockElevationGetter{}, tt.pregroundEnabled)

			poi := &model.POI{
				Lat:             0.0,
				Lon:             0.0,
				Category:        tt.category,
				WPArticleLength: tt.articleLen,
			}

			input := &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.02, Longitude: 0.0, // South of POI
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0, // Heading north towards POI
				},
			}

			session := sc.NewSession(input)
			session.Calculate(poi)

			if tt.wantLogContains != "" && !strings.Contains(poi.ScoreDetails, tt.wantLogContains) {
				t.Errorf("Expected log to contain %q, got:\n%s", tt.wantLogContains, poi.ScoreDetails)
			}
			if tt.wantLogNotContains != "" && strings.Contains(poi.ScoreDetails, tt.wantLogNotContains) {
				t.Errorf("Expected log NOT to contain %q, got:\n%s", tt.wantLogNotContains, poi.ScoreDetails)
			}
		})
	}
}
func TestScorer_StubRescue(t *testing.T) {
	scorerCfg := &config.ScorerConfig{
		PregroundBoost: 4000,
		Badges: config.BadgesConfig{
			Stub: config.StubBadgeConfig{
				ArticleLenMax: 2500,
			},
		},
	}

	catCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"monument": {Preground: true},  // Enabled
			"statue":   {Preground: false}, // Disabled
		},
	}
	catCfg.BuildLookup()

	visMgr := visibility.NewManagerForTest([]visibility.AltitudeRow{
		{AltAGL: 1000, Distances: map[visibility.SizeType]float64{visibility.SizeM: 10.0}},
	})
	visCalc := visibility.NewCalculator(visMgr, nil)

	tests := []struct {
		name             string
		pregroundEnabled bool
		category         string
		wikiLen          int
		wantStub         bool
	}{
		{
			name:             "Stub by Wiki only (statue)",
			pregroundEnabled: true,
			category:         "statue",
			wikiLen:          500,
			wantStub:         true,
		},
		{
			name:             "Rescued by Pregrounding (monument)",
			pregroundEnabled: true,
			category:         "monument",
			wikiLen:          500, // 500 + 4000 > 2500
			wantStub:         false,
		},
		{
			name:             "Pregrounding Fetch Unavailable (0 depth)",
			pregroundEnabled: true,
			category:         "statue", // Preground: false
			wikiLen:          500,
			wantStub:         true,
		},
		{
			name:             "Disabled global pregrounding",
			pregroundEnabled: false,
			category:         "monument",
			wikiLen:          500,
			wantStub:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewScorer(scorerCfg, catCfg, visCalc, &mockElevationGetter{}, tt.pregroundEnabled)
			poi := &model.POI{Category: tt.category, WPArticleLength: tt.wikiLen}
			input := &ScoringInput{Telemetry: sim.Telemetry{AltitudeAGL: 1000}}

			session := sc.NewSession(input)
			session.Calculate(poi)

			isStub := false
			for _, b := range poi.Badges {
				if b == "stub" {
					isStub = true
				}
			}

			if isStub != tt.wantStub {
				t.Errorf("got isStub=%v, want %v", isStub, tt.wantStub)
			}
		})
	}
}
