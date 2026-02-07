package scorer

import (
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/visibility"
)

// setupDeferralScorer creates a Scorer with a visibility calculator for testing
func setupDeferralScorer() *Scorer {
	scorerCfg := &config.ScorerConfig{
		DeferralEnabled:   true,
		DeferralThreshold: 1.1, // New default
	}

	// Simple visibility manager:
	// Alt 1000ft: SizeM visible up to 10nm
	visMgr := visibility.NewManagerForTest([]visibility.AltitudeRow{
		{
			AltAGL: 1000,
			Distances: map[visibility.SizeType]float64{
				visibility.SizeM: 20.0,
			},
		},
	})
	visCalc := visibility.NewCalculator(visMgr, nil)

	// Mock other dependencies
	catCfg := &config.CategoriesConfig{Categories: map[string]config.Category{}}
	return NewScorer(scorerCfg, catCfg, visCalc, &mockElevationGetter{}, nil, false)
}

func TestDefaultSession_DetermineDeferral(t *testing.T) {
	s := setupDeferralScorer()

	// Base Telemetry: 1000ft AGL, Heading North (0)
	baseTel := sim.Telemetry{
		Latitude:    0.0,
		Longitude:   0.0,
		AltitudeAGL: 1000,
		Heading:     0,
		GroundSpeed: 60, // 1nm/min
		IsOnGround:  false,
	}

	// Mock future positions: straight North at 60kts (1nm/min)
	// Horizons: 1, 2, 3, 5, 7, 9, 11, 13, 15 minutes
	futurePositions := []geo.Point{
		{Lat: 1.0 / 60.0, Lon: 0},  // +1 min
		{Lat: 2.0 / 60.0, Lon: 0},  // +2 min
		{Lat: 3.0 / 60.0, Lon: 0},  // +3 min
		{Lat: 5.0 / 60.0, Lon: 0},  // +5 min
		{Lat: 7.0 / 60.0, Lon: 0},  // +7 min
		{Lat: 9.0 / 60.0, Lon: 0},  // +9 min
		{Lat: 11.0 / 60.0, Lon: 0}, // +11 min
		{Lat: 13.0 / 60.0, Lon: 0}, // +13 min
		{Lat: 15.0 / 60.0, Lon: 0}, // +15 min
	}

	tests := []struct {
		name           string
		poiPos         geo.Point
		heading        float64
		timeToBehind   float64 // -1 means not set
		expectDeferred bool
	}{
		{
			name: "Fly-By: Improve Distance (Defer)",
			// Two-bucket design: current bucket (+1, +3 min) vs future bucket (+5, +10, +15 min).
			// POI at 12nm North, 1nm East - we're flying towards it.
			// Current bucket:
			//   +1min (1nm N): POI 11nm away, forward-right (1.0x). Score ~0.45
			//   +3min (3nm N): POI 9nm away, forward-right (1.0x). Score ~0.55
			// Future bucket:
			//   +10min (10nm N): POI 2.2nm away, forward-right (1.0x). Score ~0.89
			// Future bucket (0.89) > Current bucket best (0.55) * 1.1 (0.605) -> Defer.
			poiPos:         geo.Point{Lat: 12.0 / 60.0, Lon: 1.0 / 60.0},
			heading:        0,
			timeToBehind:   -1,
			expectDeferred: true,
		},
		{
			name: "Already Best View (No Defer)",
			// POI: 2nm North, 1nm West.
			// Current: Bearing ~330 (Left Front Best x2.0). Dist ~2.2nm.
			// Future (+1m): At 1nm North. Bearing ~315 (Left Front Best x2.0). Dist ~1.4nm.
			// Future (+3m): At 3nm North. POI is Behind (Rear). Vis 0.
			// We are at peak visibility or close to it.
			// Distance improves slightly (2.2 -> 1.4), so score goes up due to proximity.
			// 2.2nm / 20nm = 0.11 -> Vis 0.89 * 2 = 1.78
			// 1.4nm / 20nm = 0.07 -> Vis 0.93 * 2 = 1.86
			// Improvement: 1.86 / 1.78 = 1.04.
			// Threshold is 1.1. 1.04 < 1.1 -> No Defer.
			poiPos:         geo.Point{Lat: 2.0 / 60.0, Lon: -1.0 / 60.0},
			heading:        0,
			timeToBehind:   -1,
			expectDeferred: false,
		},
		{
			name: "Urgent POI (No Defer)",
			// Even if future is better, if urgent, play now.
			poiPos:         geo.Point{Lat: 10.0 / 60.0, Lon: -2.0 / 60.0}, // Same as "Fly-By"
			heading:        0,
			timeToBehind:   120, // 2 mins remaining
			expectDeferred: false,
		},
		{
			name: "Entering Blind Spot (No Defer)",
			// Flying directly over POI.
			// Current: 2nm North. Vis ~0.9.
			// Future (+1m): 1nm North. Vis ~0.95.
			// Improvement ~1.05x < 1.1x Threshold -> No Defer -> Play Now.
			// Ideally we don't want to wait until 0.1nm to start playing 30s audio.
			poiPos:         geo.Point{Lat: 2.0 / 60.0, Lon: 0.0001},
			heading:        0,
			timeToBehind:   -1,
			expectDeferred: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &ScoringInput{
				Telemetry:       baseTel,
				CategoryHistory: []string{},
				BoostFactor:     1.0,
			}
			sess := &DefaultSession{
				scorer:          s,
				input:           input,
				futurePositions: futurePositions,
			}

			poi := &model.POI{
				Lat:          tt.poiPos.Lat,
				Lon:          tt.poiPos.Lon,
				TimeToBehind: tt.timeToBehind,
				Category:     "Church", // Size M
			}

			// Pre-calculate current visibility (usually done in Calculate)
			bearing := geo.Bearing(geo.Point{Lat: 0, Lon: 0}, tt.poiPos)
			distNM := geo.Distance(geo.Point{Lat: 0, Lon: 0}, tt.poiPos) / 1852.0
			curVis := s.visCalc.CalculateVisibility(tt.heading, 1000, bearing, distNM, false, 1.0)

			// For testing "No Defer" because of blind spot, we need to ensure current vis > 0
			// s.visCalc is mocked to return >0 if <20nm

			result := sess.determineDeferral(poi, tt.heading, curVis)

			if result != tt.expectDeferred {
				t.Errorf("expected IsDeferred=%v, got %v (Current Vis: %.2f)", tt.expectDeferred, result, curVis)
			}
		})
	}
}
