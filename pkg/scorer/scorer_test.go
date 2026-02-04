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
		wantVisible   bool
		wantVisMin    float64 // Visibility score (position-based)
		wantVisMax    float64
		wantScoreMin  float64 // Intrinsic score (content-based)
		wantScoreMax  float64
		wantLogSubstr string
		wantBadges    []string
	}{
		// --- 1. Visibility Tests ---
		{
			name: "Airborne Visible (Close, Head On)",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church", // Size M -> 5nm max
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:   true,
			wantVisMin:    0.5, // ~0.52 visibility
			wantVisMax:    0.6,
			wantScoreMin:  1.25, // Novelty 1.3 (no content multipliers)
			wantScoreMax:  1.35,
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
					Latitude: -0.0001, Longitude: 0.0, // Very close
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:   false, // Now invisible because 0.0 visibility score returns true for visScore <= 0
			wantVisMin:    0.0,
			wantVisMax:    0.0,
			wantScoreMin:  0.0,
			wantLogSubstr: "Blind Spot: x0.0 (Hidden by airframe)",
		},

		// --- 2. Dimension Multiplier (affects visibility, not intrinsic) ---
		{
			name: "Dimension Boost",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
				DimensionMultiplier: 2.0,
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:   true,
			wantVisMin:    1.0, // 0.52 × 2.0 dimension
			wantVisMax:    1.1,
			wantScoreMin:  1.25, // Novelty only
			wantScoreMax:  1.35,
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
			wantVisible:   true,
			wantVisMin:    0.65, // ~0.7 visibility
			wantVisMax:    0.75,
			wantScoreMin:  1.25, // Novelty 1.3
			wantScoreMax:  1.35,
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
			wantVisible:   true,
			wantVisMin:    0.95, // Close, ~1.0 visibility
			wantVisMax:    1.05,
			wantScoreMin:  1.25, // Novelty only
			wantScoreMax:  1.35,
			wantLogSubstr: "Visibility (M@0ft)",
		},

		// --- 4. Content Multipliers (affect intrinsic score) ---
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
			wantScoreMin:  2.5, // Length 2.0 × Novelty 1.3
			wantScoreMax:  2.7,
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
			wantScoreMin:  5.0, // Sitelinks 4.0 × Novelty 1.3
			wantScoreMax:  5.5,
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
			wantScoreMin:  2.5, // Weight 2.0 × Novelty 1.3
			wantScoreMax:  2.7,
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
			wantVisible:   true,
			wantScoreMin:  0.15, // Weight 2.0 × Penalty 0.1
			wantScoreMax:  0.25,
			wantLogSubstr: "Variety Penalty (Pos 1)",
		},
		{
			name: "Category Group Penalty",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Boring", // "Boring" is in "Dull" group
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: 0.0, Longitude: 0.005, AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 315,
				},
				CategoryHistory: []string{"Church"}, // "Church" is also in "Dull" group
			},
			// Weight 0.5 × Novelty 1.3 × Group Penalty 0.5 = 0.325
			wantVisible:   true,
			wantScoreMin:  0.30,
			wantScoreMax:  0.35,
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
			wantVisible:   true,
			wantScoreMin:  2.5, // Weight 2.0 × Novelty 1.3
			wantScoreMax:  2.7,
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
			// Weight 2.0 × MSFS 4.0 × Novelty 1.3 = 10.4
			wantVisible:   true,
			wantScoreMin:  10.0,
			wantScoreMax:  11.0,
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
				RepeatTTL: 24 * time.Hour,
			},
			wantVisible:  false, // Should remain false (skipped)
			wantScoreMin: 0.0,
		},
		// --- 7. Stub Detection ---
		{
			name: "Stub Badge",
			poi: &model.POI{
				Lat: 0.0, Lon: 0.0, Category: "Church",
				WPArticleLength: 500, // < 2500 -> Stub
			},
			input: &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			},
			wantVisible:  true,
			wantScoreMin: 1.25, // Novelty only (no length boost)
			wantScoreMax: 1.35,
			wantBadges:   []string{"stub"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := s.NewSession(tt.input)
			session.Calculate(tt.poi)

			if tt.poi.IsVisible != tt.wantVisible {
				t.Errorf("got visible %v, want %v", tt.poi.IsVisible, tt.wantVisible)
			}

			// Check Visibility score (position-based)
			if tt.wantVisible && tt.wantVisMin > 0 {
				if tt.poi.Visibility < tt.wantVisMin || tt.poi.Visibility > tt.wantVisMax {
					t.Errorf("got visibility %.2f, want [%.2f, %.2f]", tt.poi.Visibility, tt.wantVisMin, tt.wantVisMax)
				}
			}

			// Check Score (intrinsic, content-based)
			if tt.wantScoreMin > 0 {
				maxScore := tt.wantScoreMax
				if maxScore == 0 {
					maxScore = tt.wantScoreMin * 1.2
				}
				if tt.poi.Score < tt.wantScoreMin || tt.poi.Score > maxScore {
					t.Errorf("got score %.2f, want [%.2f, %.2f]", tt.poi.Score, tt.wantScoreMin, maxScore)
				}
			}

			if tt.wantLogSubstr != "" && !strings.Contains(tt.poi.ScoreDetails, tt.wantLogSubstr) {
				t.Errorf("ScoreDetails missing %q, got: %s", tt.wantLogSubstr, tt.poi.ScoreDetails)
			}

			if len(tt.wantBadges) > 0 {
				for _, b := range tt.wantBadges {
					found := false
					for _, pb := range tt.poi.Badges {
						if pb == b {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("wanted badge %q, got badges: %v", b, tt.poi.Badges)
					}
				}
			}
		})
	}
}

func TestScorer_Calculate_BusyPOISkip(t *testing.T) {
	s := setupScorer()

	poi := &model.POI{
		WikidataID: "Q123",
		Lat:        0.0, Lon: 0.0, Category: "Church",
	}
	input := &ScoringInput{
		Telemetry: sim.Telemetry{
			Latitude: -0.04, Longitude: 0.0,
			AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
		},
		IsPOIBusy: func(qid string) bool {
			return qid == "Q123"
		},
	}

	session := s.NewSession(input)
	session.Calculate(poi)

	// Score should remain 0 (not calculated)
	if poi.Score != 0 {
		t.Errorf("expected Score 0 for busy POI, got %.2f", poi.Score)
	}
}

func TestScorer_VerifyBadgeWiping(t *testing.T) {
	s := setupScorer()

	poi := &model.POI{
		WikidataID: "Q123",
		Lat:        0.0, Lon: 0.0, Category: "Church",
		Badges: []string{"old_badge", "stale"},
	}
	input := &ScoringInput{
		Telemetry: sim.Telemetry{
			Latitude: -0.04, Longitude: 0.0,
			AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
		},
	}

	session := s.NewSession(input)
	session.Calculate(poi)

	// Old badges should be wiped
	for _, b := range poi.Badges {
		if b == "old_badge" || b == "stale" {
			t.Errorf("old badge %q was not wiped", b)
		}
	}
}

func TestScorer_PregroundingBonus(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		category   string
		wantBoost  bool
		wantMinLen int
	}{
		{
			name:       "Pregrounding enabled, stadium category (preground=true)",
			enabled:    true,
			category:   "Stadium",
			wantBoost:  true,
			wantMinLen: 4000,
		},
		{
			name:      "Pregrounding enabled, church category (preground=false)",
			enabled:   true,
			category:  "Church",
			wantBoost: false,
		},
		{
			name:      "Pregrounding disabled, stadium category",
			enabled:   false,
			category:  "Stadium",
			wantBoost: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorerCfg := &config.ScorerConfig{
				NoveltyBoost:   1.3,
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

			s := NewScorer(scorerCfg, catCfg, visCalc, &mockElevationGetter{}, tt.enabled)

			poi := &model.POI{
				Lat: 0.0, Lon: 0.0, Category: tt.category,
				WPArticleLength: 100, // Tiny article
			}
			input := &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			}

			session := s.NewSession(input)
			session.Calculate(poi)

			if tt.wantBoost {
				if !strings.Contains(poi.ScoreDetails, "+4000") {
					t.Errorf("expected pregrounding boost in score details, got: %s", poi.ScoreDetails)
				}
			} else {
				if strings.Contains(poi.ScoreDetails, "+4000") {
					t.Errorf("unexpected pregrounding boost, got: %s", poi.ScoreDetails)
				}
			}
		})
	}
}

func TestScorer_StubRescue(t *testing.T) {
	tests := []struct {
		name          string
		category      string
		articleLen    int
		pregroundOn   bool
		wantStubBadge bool
	}{
		{
			name:          "Stub by Wiki only (statue)",
			category:      "Statue",
			articleLen:    500,
			pregroundOn:   true,
			wantStubBadge: true, // 500 + 0 (no preground for statue) = 500 < 2500
		},
		{
			name:          "Rescued by Pregrounding (monument)",
			category:      "Monument",
			articleLen:    500,
			pregroundOn:   true,
			wantStubBadge: false, // 500 + 4000 = 4500 > 2500
		},
		{
			name:          "Pregrounding Fetch Unavailable (0 depth)",
			category:      "Monument",
			articleLen:    0,
			pregroundOn:   true,
			wantStubBadge: false, // 0 article = no stub badge (design: we only mark stubs with SOME text)
		},
		{
			name:          "Disabled global pregrounding",
			category:      "Monument",
			articleLen:    500,
			pregroundOn:   false,
			wantStubBadge: true, // 500 < 2500
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorerCfg := &config.ScorerConfig{
				NoveltyBoost:   1.3,
				PregroundBoost: 4000,
				Badges: config.BadgesConfig{
					Stub: config.StubBadgeConfig{ArticleLenMax: 2500},
				},
			}
			catCfg := &config.CategoriesConfig{
				Categories: map[string]config.Category{
					"statue":   {Weight: 1.0, Size: "S", Preground: false},
					"monument": {Weight: 1.0, Size: "M", Preground: true},
				},
			}
			catCfg.BuildLookup()

			visMgr := visibility.NewManagerForTest([]visibility.AltitudeRow{
				{AltAGL: 1000, Distances: map[visibility.SizeType]float64{
					visibility.SizeS: 5.0, visibility.SizeM: 10.0,
				}},
			})
			visCalc := visibility.NewCalculator(visMgr, nil)

			s := NewScorer(scorerCfg, catCfg, visCalc, &mockElevationGetter{}, tt.pregroundOn)

			poi := &model.POI{
				Lat: 0.0, Lon: 0.0, Category: tt.category,
				WPArticleLength: tt.articleLen,
			}
			input := &ScoringInput{
				Telemetry: sim.Telemetry{
					Latitude: -0.04, Longitude: 0.0,
					AltitudeMSL: 1000, AltitudeAGL: 1000, Heading: 0,
				},
			}

			session := s.NewSession(input)
			session.Calculate(poi)

			hasStub := false
			for _, b := range poi.Badges {
				if b == "stub" {
					hasStub = true
					break
				}
			}

			if hasStub != tt.wantStubBadge {
				t.Errorf("stub badge: got %v, want %v (badges: %v)", hasStub, tt.wantStubBadge, poi.Badges)
			}
		})
	}
}
