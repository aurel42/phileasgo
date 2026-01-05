package classifier

import (
	"testing"
)

func TestDimensionTracker_SlidingWindow(t *testing.T) {
	dt := NewDimensionTracker(3)

	// Tile 1: Max Height 100
	dt.ResetTile()
	dt.ObserveArticle(100, 0, 0)
	dt.FinalizeTile()

	// Tile 2: Max Height 200
	dt.ResetTile()
	dt.ObserveArticle(200, 0, 0)
	dt.FinalizeTile()

	// Tile 3: Max Height 300
	dt.ResetTile()
	dt.ObserveArticle(300, 0, 0)
	dt.FinalizeTile()

	// Medians should be from [100, 200, 300] -> 200 (since 3/2 = 1, hs[1]=200)
	medH, _, _ := dt.getMedians()
	if medH != 200 {
		t.Errorf("Expected median height 200, got %f", medH)
	}

	// Tile 4 (Evicts 1)
	dt.ResetTile()
	dt.ObserveArticle(400, 0, 0)
	dt.FinalizeTile()

	// Medians from [200, 300, 400] -> 300
	medH2, _, _ := dt.getMedians()
	if medH2 != 300 {
		t.Errorf("Expected median height 300, got %f", medH2)
	}
}

func TestDimensionTracker_Rescue(t *testing.T) {
	dt := NewDimensionTracker(10)

	// Mocking some previous records (10 tiles with max height 50)
	for i := 0; i < 10; i++ {
		dt.ResetTile()
		dt.ObserveArticle(50, 0, 0)
		dt.FinalizeTile()
	}

	// Current tile: max so far is 0
	dt.ResetTile()

	// Case 1: Article with height 100. Should be rescue because it's a TILE RECORD (100 > 0)
	if !dt.ShouldRescue(100, 0, 0) {
		t.Error("Expected rescue for tile record")
	}

	// Update tile record
	dt.ObserveArticle(100, 0, 0)

	// Case 2: Article with height 20. Should NOT be rescued (not record, and not > median 50)
	if dt.ShouldRescue(20, 0, 0) {
		t.Error("Did not expect rescue for small article")
	}

	// Case 3: Article with height 60.
	// It's NOT a tile record (60 < 100), but it EXCEEDS MEDIAN (60 > 50).
	if !dt.ShouldRescue(60, 0, 0) {
		t.Error("Expected rescue for exceeding global median")
	}
}
