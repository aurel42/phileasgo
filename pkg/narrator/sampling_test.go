package narrator

import (
	"phileasgo/pkg/config"
	"testing"
)

// TestSampleNarrationLength verifies that the sampled length respects min/max and the step size of 10.
func TestSampleNarrationLength(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.NarrationLengthMin = 100
	cfg.Narrator.NarrationLengthMax = 200

	// Use a nil-heavy service since we only test the pure function
	s := &AIService{
		cfg: cfg,
	}

	for i := 0; i < 1000; i++ {
		val := s.sampleNarrationLength()

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
	s := &AIService{cfg: cfg}

	// Case 1: Max <= Min -> returns Min
	cfg.Narrator.NarrationLengthMin = 500
	cfg.Narrator.NarrationLengthMax = 400
	val := s.sampleNarrationLength()
	if val != 500 {
		t.Errorf("Expected 500 for inverted range, got %d", val)
	}

	// Case 2: Zero values -> defaults (Min 400)
	cfg.Narrator.NarrationLengthMin = 0
	cfg.Narrator.NarrationLengthMax = 0
	val2 := s.sampleNarrationLength()
	// Default logic: defaults min=400, max=600 if 0
	if val2 < 400 || val2 > 600 {
		t.Errorf("Expected default range 400-600, got %d", val2)
	}
}
