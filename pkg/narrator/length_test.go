package narrator

import (
	"context"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"

	"github.com/stretchr/testify/assert"
)

// Mock for testing logic (we can reuse the one in mocks_dev_test.go if it's exported, but it's likely not visible if in _test package?)
// Actually, mocks_dev_test.go is package narrator. So we can use it if we are in package narrator.
// But we need to control CountScoredAbove behavior for this test.

type TestPOIProvider struct {
	CountVal int
}

func (m *TestPOIProvider) GetPOI(ctx context.Context, qid string) (*model.POI, error) {
	return nil, nil
}
func (m *TestPOIProvider) GetBestCandidate() *model.POI { return nil }
func (m *TestPOIProvider) CountScoredAbove(threshold float64, limit int) int {
	if m.CountVal > limit {
		return limit
	}
	return m.CountVal
}

func TestSampleNarrationLength_RelativeDominance(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Narrator: config.NarratorConfig{
			NarrationLengthMin: 100,
			NarrationLengthMax: 200,
		},
	}

	// Case 1: High Competition (Rivals > 1) -> Skew Short
	// Logic: If strategy is min_skew, result should be the MIN of 3 rolls.
	// We can't deterministically test random, but we can verify statistical tendency OR just check logs?
	// The function returns an int.
	// Let's run it many times and check average.

	iterations := 1000

	// Mock High Competition (Rivals = 5)
	svcHigh := &AIService{
		cfg:    cfg,
		poiMgr: &TestPOIProvider{CountVal: 5},
	}

	sumHigh := 0
	for i := 0; i < iterations; i++ {
		val, _ := svcHigh.sampleNarrationLength(&model.POI{Score: 100}, "")
		sumHigh += val
	}
	avgHigh := float64(sumHigh) / float64(iterations)

	// Case 2: Lone Wolf (Rivals = 0 or 1 inclusive of self?)
	// Logic: "Rivals > 1" -> Short. "Rivals <= 1" -> Long.
	// If I am the only one > threshold, Rivals = 1.
	// The implementation says: if rivals <= 1 -> max_skew.

	svcLow := &AIService{
		cfg:    cfg,
		poiMgr: &TestPOIProvider{CountVal: 1}, // Just me
	}

	sumLow := 0
	for i := 0; i < iterations; i++ {
		val, _ := svcLow.sampleNarrationLength(&model.POI{Score: 100}, "")
		sumLow += val
	}
	avgLow := float64(sumLow) / float64(iterations)

	// Assert that High Competition (Short Skew) has lower average than Lone Wolf (Long Skew)
	// With Min=100, Max=200.
	// Uniform Avg ~ 150.
	// Min Skew Avg < 150.
	// Max Skew Avg > 150.

	t.Logf("High Comp Avg: %.2f", avgHigh)
	t.Logf("Low Comp Avg: %.2f", avgLow)

	assert.True(t, avgHigh < avgLow, "High competition should result in shorter average length")
	assert.Less(t, avgHigh, 150.0, "High competition should be skewed below midpoint")
	assert.Greater(t, avgLow, 150.0, "Low competition should be skewed above midpoint")
}
