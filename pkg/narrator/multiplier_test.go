package narrator

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"
)

func TestSampleNarrationLength_Multipliers(t *testing.T) {
	mockStore := &MockStore{
		State: make(map[string]string),
	}
	cfg := config.DefaultConfig()
	cfg.Narrator.NarrationLengthShortWords = 100
	cfg.Narrator.NarrationLengthLongWords = 200

	svc := &AIService{
		cfg:    cfg,
		st:     mockStore,
		poiMgr: &MockPOIProvider{},
	}

	tests := []struct {
		name       string
		textLength string // "1".."5"
		strategy   string // "min_skew", "max_skew"
		want       int    // Expected target words
		tolerance  int    // Allow slight rounding diff if needed, though int cast is consistent
	}{
		{
			name:       "Default (1) - Short Skew",
			textLength: "1",
			strategy:   StrategyMinSkew,
			want:       100, // 100 * 1.0
		},
		{
			name:       "Default (1) - Long Skew",
			textLength: "1",
			strategy:   StrategyMaxSkew,
			want:       200, // 200 * 1.0
		},
		{
			name:       "Medium (3) - x1.5 - Short Skew",
			textLength: "3",
			strategy:   StrategyMinSkew,
			want:       150, // 100 * 1.5
		},
		{
			name:       "Medium (3) - x1.5 - Long Skew",
			textLength: "3",
			strategy:   StrategyMaxSkew,
			want:       300, // 200 * 1.5
		},
		{
			name:       "Max (5) - x2.0 - Short Skew",
			textLength: "5",
			strategy:   StrategyMinSkew,
			want:       200, // 100 * 2.0
		},
		{
			name:       "Max (5) - x2.0 - Long Skew",
			textLength: "5",
			strategy:   StrategyMaxSkew,
			want:       400, // 200 * 2.0
		},
		{
			name:       "Clamped Max (>5)",
			textLength: "10",
			strategy:   StrategyMaxSkew,
			want:       400, // Clamped to 5 -> 2.0x
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore.SetState(context.Background(), "text_length", tt.textLength)

			val, _ := svc.sampleNarrationLength(&model.POI{}, tt.strategy)

			if val != tt.want {
				t.Errorf("sampleNarrationLength(%s, %s) = %d, want %d", tt.textLength, tt.strategy, val, tt.want)
			}
		})
	}
}
