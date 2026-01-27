package scorer

import (
	"math"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
)

func TestDefaultSession_CheckDeferral(t *testing.T) {
	s := &Scorer{
		config: &config.ScorerConfig{
			DeferralEnabled:    true,
			DeferralThreshold:  0.75,
			DeferralMultiplier: 0.1,
		},
	}

	// Mock future positions: +1, +3, +5, +10, +15 minutes
	// For testing, let's assume GS=60kts (1nm per min) heading NORTH (0deg)
	// Aircraft is at (0,0) at t=0
	futurePositions := []geo.Point{
		{Lat: 1.0 / 60.0, Lon: 0},  // +1 min (1nm north)
		{Lat: 3.0 / 60.0, Lon: 0},  // +3 min (3nm north)
		{Lat: 5.0 / 60.0, Lon: 0},  // +5 min (5nm north)
		{Lat: 10.0 / 60.0, Lon: 0}, // +10 min (10nm north)
		{Lat: 15.0 / 60.0, Lon: 0}, // +15 min (15nm north)
	}

	sess := &DefaultSession{
		scorer:          s,
		futurePositions: futurePositions,
	}

	tests := []struct {
		name       string
		poiPos     geo.Point
		heading    float64
		expectMult float64
	}{
		{
			name:       "Classic Approach (Significantly closer later)",
			poiPos:     geo.Point{Lat: 15.0 / 60.0, Lon: 0}, // 15nm North
			heading:    0,
			expectMult: 0.1,
			// Close Group (1, 3):
			// t=1: 14nm, t=3: 12nm. BestClose = 12nm.
			// Far Group (5, 10, 15):
			// t=5: 10nm, t=10: 5nm, t=15: 0nm. BestFar = 0nm.
			// 0 < 0.75 * 12 (9) -> DEFER
		},
		{
			name:       "Already Close (Don't defer)",
			poiPos:     geo.Point{Lat: 4.0 / 60.0, Lon: 0}, // 4nm North
			heading:    0,
			expectMult: 1.0,
			// Close Group (1, 3):
			// t=1: 3nm, t=3: 1nm. BestClose = 1nm.
			// Far Group (5, 10, 15):
			// t=5: 1nm (passed), t=10: 6nm (passed). BestFar = 1nm (distance is absolute, but direction matters)
			// Wait, at t=5 (5nm N), POI (4nm N) is BEHIND (180deg). So t=5 is INVALID.
			// All Far points are invalid/behind.
			// Result -> NO DEFER (1.0)
		},
		{
			name:       "Mid-Range Approach (Closer at 5m vs 3m)",
			poiPos:     geo.Point{Lat: 8.0 / 60.0, Lon: 0}, // 8nm North
			heading:    0,
			expectMult: 0.1,
			// Close Group: t=1 (7nm), t=3 (5nm). BestClose = 5nm.
			// Far Group: t=5 (3nm), t=10 (2nm passed/behind?).
			// at t=10 (10nm N), POI (8nm N) is behind. Invalid.
			// BestFar = 3nm (at t=5).
			// 3 < 0.75 * 5 (3.75) -> DEFER
		},
		{
			name:       "POI Side Tangent (Steady distance)",
			poiPos:     geo.Point{Lat: 5.0 / 60.0, Lon: 5.0 / 60.0}, // 5nm North, 5nm East
			heading:    0,
			expectMult: 1.0,
			// Close:
			// t=3 (3N): Dist to (5N, 5E) = sqrt(2^2 + 5^2) = 5.38nm.
			// Far:
			// t=5 (5N): Dist to (5N, 5E) = 5.0nm.
			// 5.0 is NOT < 0.75 * 5.38 (4.03). NO DEFER.
		},
		{
			name:       "POI Side Tangent (Converging enough)",
			poiPos:     geo.Point{Lat: 15.0 / 60.0, Lon: 2.0 / 60.0}, // 15nm North, 2nm East
			heading:    0,
			expectMult: 0.1,
			// Close:
			// t=3 (3N): Dist to (15N, 2E) = sqrt(12^2 + 2^2) = 12.16nm.
			// Far:
			// t=15 (15N): Dist to (15N, 2E) = 2.0nm.
			// 2.0 < 0.75 * 12.16 (9.12). DEFER.
		},
		{
			name:       "POI Passed at t=1 (Invalid Close)",
			poiPos:     geo.Point{Lat: 0.5 / 60.0, Lon: 0}, // 0.5nm North
			heading:    0,
			expectMult: 1.5, // 1.5 Urgency Boost (Disappearing soon)
			// t=1 (1N): POI (0.5N) is behind. Invalid.
			// t=3 (3N): POI (0.5N) is behind. Invalid.
			// BestClose = Inf.
			// Result -> NO DEFER (1.0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				poi := &model.POI{
					Lat: tt.poiPos.Lat,
					Lon: tt.poiPos.Lon,
				}
				mult, logs, _ := sess.checkDeferral(poi, tt.heading)
				if math.Abs(mult-tt.expectMult) > 0.001 {
					t.Errorf("expected multiplier %f, got %f (logs: %s)", tt.expectMult, mult, logs)
				}
			})
		})
	}
}
