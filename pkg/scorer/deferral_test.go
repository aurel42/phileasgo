package scorer

import (
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
)

func TestDefaultSession_DetermineDeferral(t *testing.T) {
	s := &Scorer{
		config: &config.ScorerConfig{
			DeferralEnabled:   true,
			DeferralThreshold: 0.75,
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
		name              string
		poiPos            geo.Point
		heading           float64
		visibility        float64
		timeToBehind      float64 // -1 means not set
		expectDeferred    bool
	}{
		{
			name:           "Classic Approach - Low Visibility (Defer)",
			poiPos:         geo.Point{Lat: 15.0 / 60.0, Lon: 0}, // 15nm North
			heading:        0,
			visibility:     0.2, // Below threshold (0.4)
			timeToBehind:   -1,
			expectDeferred: true,
			// Close Group (1, 3): BestClose = 12nm
			// Far Group (5, 10, 15): BestFar = 0nm
			// 0 < 0.75 * 12 -> DEFER
		},
		{
			name:           "Classic Approach - Good Visibility (No Defer)",
			poiPos:         geo.Point{Lat: 15.0 / 60.0, Lon: 0}, // 15nm North
			heading:        0,
			visibility:     0.5, // Above threshold (0.4)
			timeToBehind:   -1,
			expectDeferred: false,
			// Good visibility overrides deferral
		},
		{
			name:           "Approaching but Urgent (No Defer)",
			poiPos:         geo.Point{Lat: 15.0 / 60.0, Lon: 0}, // 15nm North
			heading:        0,
			visibility:     0.2,
			timeToBehind:   180, // 3 minutes - urgent
			expectDeferred: false,
			// Urgent POIs should not be deferred
		},
		{
			name:           "Already Close (No Defer)",
			poiPos:         geo.Point{Lat: 4.0 / 60.0, Lon: 0}, // 4nm North
			heading:        0,
			visibility:     0.3,
			timeToBehind:   -1,
			expectDeferred: false,
			// All Far points are invalid/behind -> no defer
		},
		{
			name:           "Mid-Range Approach (Defer)",
			poiPos:         geo.Point{Lat: 8.0 / 60.0, Lon: 0}, // 8nm North
			heading:        0,
			visibility:     0.3,
			timeToBehind:   -1,
			expectDeferred: true,
			// Close Group: BestClose = 5nm
			// Far Group: BestFar = 3nm
			// 3 < 0.75 * 5 -> DEFER
		},
		{
			name:           "POI Side Tangent - Steady Distance (No Defer)",
			poiPos:         geo.Point{Lat: 5.0 / 60.0, Lon: 5.0 / 60.0}, // 5nm North, 5nm East
			heading:        0,
			visibility:     0.3,
			timeToBehind:   -1,
			expectDeferred: false,
			// Distance stays roughly the same, no significant improvement
		},
		{
			name:           "POI Side Tangent - Converging (Defer)",
			poiPos:         geo.Point{Lat: 15.0 / 60.0, Lon: 2.0 / 60.0}, // 15nm North, 2nm East
			heading:        0,
			visibility:     0.2,
			timeToBehind:   -1,
			expectDeferred: true,
			// Will be much closer later
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poi := &model.POI{
				Lat:          tt.poiPos.Lat,
				Lon:          tt.poiPos.Lon,
				TimeToBehind: tt.timeToBehind,
			}
			result := sess.determineDeferral(poi, tt.heading, tt.visibility)
			if result != tt.expectDeferred {
				t.Errorf("expected IsDeferred=%v, got %v", tt.expectDeferred, result)
			}
		})
	}
}
