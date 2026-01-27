package narrator

import (
	"phileasgo/pkg/config"
	"testing"
)

// TestSampleNarrationLength verifies that the sampled length respects min/max and the step size of 10.
func TestSampleNarrationLength(t *testing.T) {
	cfg := config.DefaultConfig()
	// Default config
	cfg.Narrator.NarrationLengthShortWords = 100
	cfg.Narrator.NarrationLengthLongWords = 200

	// Use a nil-heavy service since we only test the pure function
	s := &AIService{
		cfg:    cfg,
		poiMgr: &MockPOIProvider{},
		st:     &MockStore{},
	}

	for i := 0; i < 1000; i++ {
		val, _ := s.sampleNarrationLength(nil, "", 1000)

		if val < 100 {
			t.Errorf("Sampled value %d below min 100", val)
		}
		if val > 200 {
			t.Errorf("Sampled value %d above max 200", val)
		}
		if val%10 != 0 {
			t.Errorf("Sampled value %d is not a multiple of 10", val)
		}
	}
}

// TestSampleNarrationLength_EdgeCases verifies behavior with weird config values.
func TestSampleNarrationLength_EdgeCases(t *testing.T) {
	cfg := config.DefaultConfig()
	s := &AIService{cfg: cfg, poiMgr: &MockPOIProvider{}, st: &MockStore{}}

	// Case 2: Zero values -> defaults (Short 50, Long 200)
	cfg.Narrator.NarrationLengthShortWords = 0
	cfg.Narrator.NarrationLengthLongWords = 0
	val2, _ := s.sampleNarrationLength(nil, "", 1000)
	// Default logic: defaults Short=50, Long=200
	// Multiplier 1.0 (default)
	// Strategy will determine which one.
	if val2 < 50 || val2 > 200 {
		t.Errorf("Expected default range 50-200, got %d", val2)
	}
}
