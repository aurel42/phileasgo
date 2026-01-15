package narrator

import (
	"phileasgo/pkg/model"
	"testing"
)

// mockPOIAnalyzer implements POIAnalyzer for testing
type mockPOIAnalyzer struct {
	count int
}

func (m *mockPOIAnalyzer) CountScoredAbove(threshold float64, limit int) int {
	return m.count
}

func TestDetermineSkewStrategy(t *testing.T) {
	tests := []struct {
		name       string
		poi        *model.POI
		rivalCount int
		isOnGround bool
		want       string
	}{
		{
			name:       "nil POI returns uniform",
			poi:        nil,
			rivalCount: 0,
			isOnGround: false,
			want:       StrategyUniform,
		},
		{
			name:       "on ground always returns max_skew",
			poi:        &model.POI{Score: 5.0},
			rivalCount: 10, // Even with many rivals
			isOnGround: true,
			want:       StrategyMaxSkew,
		},
		{
			name:       "on ground with low score still returns max_skew",
			poi:        &model.POI{Score: 0.1},
			rivalCount: 0,
			isOnGround: true,
			want:       StrategyMaxSkew,
		},
		{
			name:       "airborne lone wolf (no rivals) returns max_skew",
			poi:        &model.POI{Score: 5.0},
			rivalCount: 1, // Only self
			isOnGround: false,
			want:       StrategyMaxSkew,
		},
		{
			name:       "airborne with competition returns min_skew",
			poi:        &model.POI{Score: 5.0},
			rivalCount: 2, // Self + 1 rival
			isOnGround: false,
			want:       StrategyMinSkew,
		},
		{
			name:       "airborne with many rivals returns min_skew",
			poi:        &model.POI{Score: 5.0},
			rivalCount: 5, // Self + 4 rivals
			isOnGround: false,
			want:       StrategyMinSkew,
		},
		{
			name:       "airborne low score no rivals returns max_skew",
			poi:        &model.POI{Score: 0.3},
			rivalCount: 1,
			isOnGround: false,
			want:       StrategyMaxSkew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &mockPOIAnalyzer{count: tt.rivalCount}
			got := DetermineSkewStrategy(tt.poi, analyzer, tt.isOnGround)
			if got != tt.want {
				t.Errorf("DetermineSkewStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}
