package classifier

import (
	"testing"
)

func TestDimensions_DynamicBaseline(t *testing.T) {
	dt := NewDimensionTracker(10)

	// Seed with large items to establish a high median (50m)
	for i := 0; i < 10; i++ {
		dt.ResetTile()
		dt.ObserveArticle(50, 0, 0)
		dt.FinalizeTile() // Each record is 50
	}

	// Verify median is 50
	medH, _, _ := dt.getMedians()
	if medH != 50 {
		t.Fatalf("Setup failed: expected median 50, got %f", medH)
	}

	// ResetTile should now set currentHeight = 50 * 1.0 = 50
	dt.ResetTile()

	// Case 1: Small item (10m) in a fresh tile
	// Previously (with 0 baseline), this would be rescued as a "record" (10 > 0).
	// Now (with 50 baseline), 10 < 50, so it should NOT be rescued.
	if dt.ShouldRescue(10, 0, 0) {
		t.Error("Expected small item (10m) to be ignored due to dynamic baseline (50m)")
	}

	// Case 2: Large item (60m)
	// 60 > 50, so it should be rescued.
	if !dt.ShouldRescue(60, 0, 0) {
		t.Error("Expected large item (60m) to be rescued (> baseline)")
	}
}
