package narrator

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSampleNarrationLength_Scaling(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.NarrationLengthShortWords = 100
	cfg.Narrator.NarrationLengthLongWords = 200
	cfg.Narrator.LengthScalingFactor = 0.5

	s := &AIService{
		cfg:    cfg,
		poiMgr: &MockPOIProvider{},
		st:     &MockStore{},
	}

	tests := []struct {
		name         string
		strategy     string
		sourceWords  int
		expectWords  int
		expectCapped bool
	}{
		{
			name:        "Large source - No capping (Long)",
			strategy:    StrategyMaxSkew,
			sourceWords: 1000,
			expectWords: 200,
		},
		{
			name:        "Small source - Capped (Long)",
			strategy:    StrategyMaxSkew,
			sourceWords: 100, // 100 * 0.5 = 50
			expectWords: 50,
		},
		{
			name:        "Tiny source - Capped (Short)",
			strategy:    StrategyMinSkew,
			sourceWords: 40, // 40 * 0.5 = 20
			expectWords: 20,
		},
		{
			name:        "Zero source - No capping (Fallback)",
			strategy:    StrategyMinSkew,
			sourceWords: 0,
			expectWords: 100,
		},
		{
			name:        "Factor 1.0 - Proportional",
			strategy:    StrategyMaxSkew,
			sourceWords: 150,
			expectWords: 150, // Factor override below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Factor 1.0 - Proportional" {
				old := s.cfg.Narrator.LengthScalingFactor
				s.cfg.Narrator.LengthScalingFactor = 1.0
				defer func() { s.cfg.Narrator.LengthScalingFactor = old }()
			}

			words, _ := s.sampleNarrationLength(&model.POI{}, tt.strategy, tt.sourceWords)
			assert.Equal(t, tt.expectWords, words)
		})
	}
}
