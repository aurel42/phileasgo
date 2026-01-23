package scorer

import (
	"math"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
)

func TestDefaultSession_CheckDeferral(t *testing.T) {
	s := &Scorer{
		config: &config.ScorerConfig{
			DeferralEnabled:    true,
			DeferralThreshold:  0.75,
			DeferralMultiplier: 0.1,
		},
	}

	// Mock future positions: +1, +3, +5, +10 minutes
	// For testing, let's assume GS=60kts (1nm per min) heading NORTH (0deg)
	futurePositions := []geo.Point{
		{Lat: 1.0 / 60.0, Lon: 0},  // +1 min (1nm north)
		{Lat: 3.0 / 60.0, Lon: 0},  // +3 min (3nm north)
		{Lat: 5.0 / 60.0, Lon: 0},  // +5 min (5nm north)
		{Lat: 10.0 / 60.0, Lon: 0}, // +10 min (10nm north)
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
			name:       "POI directly ahead (will fly over)",
			poiPos:     geo.Point{Lat: 6.0 / 60.0, Lon: 0}, // 6nm North
			heading:    0,
			expectMult: 0.1, // distNow=5nm, minFuture=1nm (at +5min) -> 1 < 0.75*5 (3.75)
		},
		{
			name:       "POI already passed at +1min",
			poiPos:     geo.Point{Lat: 0.5 / 60.0, Lon: 0}, // 0.5nm North (passed at +1min)
			heading:    0,
			expectMult: 1.0,
		},
		{
			name:       "POI significantly closer in future (Tangent)",
			poiPos:     geo.Point{Lat: 4.0 / 60.0, Lon: 1.0 / 60.0}, // 4nm North, 1nm East
			heading:    0,
			expectMult: 0.1,
			// at +1min (1N, 0E): dist to (4N, 1E) = sqrt(3^2 + 1^2) = 3.16nm
			// at +3min (3N, 0E): dist to (4N, 1E) = sqrt(1^2 + 1^2) = 1.41nm
			// 1.41 < 0.75 * 3.16 (2.37) -> DEFER
		},
		{
			name:       "POI NOT significantly closer in future (Strict Tangent)",
			poiPos:     geo.Point{Lat: 0.5 / 60.0, Lon: 5.0 / 60.0}, // 0.5nm North, 5nm East
			heading:    0,
			expectMult: 1.0,
			// at +1min (1N, 0E): dist to (0.5N, 5E) = sqrt(0.5^2 + 5^2) = 5.02nm
			// at +3min (3N, 0E): dist to (0.5N, 5E) = sqrt(2.5^2 + 5^2) = 5.59nm
			// minFuture(3,5,10) = 5.59nm > 0.75 * 5.02 (3.76) -> NO DEFER
		},
		{
			name:       "POI far ahead, won't get much closer",
			poiPos:     geo.Point{Lat: 50.0 / 60.0, Lon: 0}, // 50nm North
			heading:    0,
			expectMult: 1.0, // distNow=49nm, dist[10]=40nm. 40/49 = 0.81 > 0.75 -> NO DEFER
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mult, _ := sess.checkDeferral(tt.poiPos, tt.heading)
			if math.Abs(mult-tt.expectMult) > 0.001 {
				t.Errorf("expected multiplier %f, got %f", tt.expectMult, mult)
			}
		})
	}
}
