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

func TestSampleSkewedValue(t *testing.T) {
	tests := []struct {
		name     string
		minVal   int
		maxVal   int
		strategy string
	}{
		{
			name:     "min_skew samples from lower range",
			minVal:   100,
			maxVal:   500,
			strategy: StrategyMinSkew,
		},
		{
			name:     "max_skew samples from upper range",
			minVal:   100,
			maxVal:   500,
			strategy: StrategyMaxSkew,
		},
		{
			name:     "uniform samples across range",
			minVal:   100,
			maxVal:   500,
			strategy: StrategyUniform,
		},
		{
			name:     "fixed returns minVal",
			minVal:   100,
			maxVal:   500,
			strategy: StrategyFixed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple samples to test distribution
			samples := make([]int, 100)
			for i := 0; i < 100; i++ {
				samples[i] = SampleSkewedValue(tt.minVal, tt.maxVal, tt.strategy)
				// Basic bounds check
				if samples[i] < tt.minVal || samples[i] > tt.maxVal {
					t.Errorf("Sample %d out of bounds: %d (min=%d, max=%d)",
						i, samples[i], tt.minVal, tt.maxVal)
				}
			}

			// Strategy-specific checks
			if tt.strategy == StrategyFixed {
				for i, s := range samples {
					if s != tt.minVal {
						t.Errorf("Fixed strategy sample %d = %d, want %d", i, s, tt.minVal)
					}
				}
			}
		})
	}
}

func TestSampleSkewedValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		minVal   int
		maxVal   int
		strategy string
		want     int
	}{
		{
			name:     "equal min max returns min",
			minVal:   100,
			maxVal:   100,
			strategy: StrategyUniform,
			want:     100,
		},
		{
			name:     "max less than min returns min",
			minVal:   100,
			maxVal:   50,
			strategy: StrategyUniform,
			want:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SampleSkewedValue(tt.minVal, tt.maxVal, tt.strategy)
			if got != tt.want {
				t.Errorf("SampleSkewedValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
