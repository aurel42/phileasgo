package rescue

import (
	"testing"
)

func TestCalculateMedian(t *testing.T) {
	neighbors := []TileStats{
		{MaxHeight: 10, MaxLength: 100, MaxArea: 1000},
		{MaxHeight: 20, MaxLength: 200, MaxArea: 2000},
		{MaxHeight: 30, MaxLength: 300, MaxArea: 3000},
	}

	medians := CalculateMedian(neighbors)

	if medians.MedianHeight != 20 {
		t.Errorf("Expected MedianHeight 20, got %f", medians.MedianHeight)
	}
	if medians.MedianLength != 200 {
		t.Errorf("Expected MedianLength 200, got %f", medians.MedianLength)
	}
	if medians.MedianArea != 2000 {
		t.Errorf("Expected MedianArea 2000, got %f", medians.MedianArea)
	}
}

func TestBatch(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		name       string
		candidates []Article
		localMax   TileStats
		medians    MedianStats
		wantCount  int
		wantID     string
		wantCat    string
	}{
		{
			name: "Single Height Outlier",
			candidates: []Article{
				{ID: "Q1", Height: floatPtr(10)},
				{ID: "Q2", Height: floatPtr(500)},
			},
			localMax:  TileStats{MaxHeight: 500},
			medians:   MedianStats{MedianHeight: 50}, // Threshold 100
			wantCount: 1,
			wantID:    "Q2",
			wantCat:   "height",
		},
		{
			name: "Uniform Height (No Rescue)",
			candidates: []Article{
				{ID: "Q1", Height: floatPtr(50)},
				{ID: "Q2", Height: floatPtr(50)},
			},
			localMax:  TileStats{MaxHeight: 50},
			medians:   MedianStats{MedianHeight: 50}, // Threshold 100
			wantCount: 0,
		},
		{
			name: "Length Outlier",
			candidates: []Article{
				{ID: "Q1", Length: floatPtr(10)},
				{ID: "Q2", Length: floatPtr(1000)},
			},
			localMax:  TileStats{MaxLength: 1000},
			medians:   MedianStats{MedianLength: 100}, // Threshold 200
			wantCount: 1,
			wantID:    "Q2",
			wantCat:   "length",
		},
		{
			name: "Area Outlier",
			candidates: []Article{
				{ID: "Q1", Area: floatPtr(100)},
				{ID: "Q2", Area: floatPtr(10000)},
			},
			localMax:  TileStats{MaxArea: 10000},
			medians:   MedianStats{MedianArea: 1000}, // Threshold 2000
			wantCount: 1,
			wantID:    "Q2",
			wantCat:   "area",
		},
		{
			name: "Multiple Dimensions (No Multi-Rescue for same ID)",
			candidates: []Article{
				{ID: "Q1", Height: floatPtr(500), Length: floatPtr(1000)},
			},
			localMax:  TileStats{MaxHeight: 500, MaxLength: 1000},
			medians:   MedianStats{MedianHeight: 50, MedianLength: 100},
			wantCount: 1,
			wantID:    "Q1",
			wantCat:   "height", // Priority order in Batch: Height -> Length -> Area
		},
		{
			name: "Anchor Baseline (Sparse Area)",
			candidates: []Article{
				{ID: "Q_TOWER", Height: floatPtr(50)},
				{ID: "Q_ANTENNA", Height: floatPtr(5)},
			},
			localMax:  TileStats{MaxHeight: 50},
			medians:   MedianStats{MedianHeight: 0}, // No neighbors have height
			wantCount: 1,
			wantID:    "Q_TOWER",
			wantCat:   "height",
		},
		{
			name: "Anchor Baseline (Noise Suppression)",
			candidates: []Article{
				{ID: "Q_ANTENNA", Height: floatPtr(5)},
			},
			localMax:  TileStats{MaxHeight: 5},
			medians:   MedianStats{MedianHeight: 0},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Batch(tt.candidates, tt.localMax, tt.medians, 30.0, 500.0, 10000.0)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d rescued, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 {
				if got[0].ID != tt.wantID {
					t.Errorf("got ID %s, want %s", got[0].ID, tt.wantID)
				}
				if got[0].Category != tt.wantCat {
					t.Errorf("got category %s, want %s", got[0].Category, tt.wantCat)
				}
			}
		})
	}
}

func TestRescueBatch_Duplicate(t *testing.T) {
	h := 500.0
	candidates := []Article{
		{ID: "Q1", Height: &h, Length: &h},
	}

	localMax := TileStats{MaxHeight: 500.0, MaxLength: 500.0}
	medians := MedianStats{MedianHeight: 50.0, MedianLength: 50.0}

	rescued := Batch(candidates, localMax, medians, 30.0, 500.0, 10000.0)

	if len(rescued) != 1 {
		t.Fatalf("Expected 1 rescued article (no duplicates), got %d", len(rescued))
	}
}
func TestAnalyzeTile(t *testing.T) {
	v10, v20, v30 := 10.0, 20.0, 30.0
	articles := []Article{
		{ID: "Q1", Height: &v10, Length: &v20, Area: &v30},
		{ID: "Q2", Height: &v30, Length: &v10, Area: &v20},
	}

	stats := AnalyzeTile(50.0, 10.0, articles)

	if stats.MaxHeight != 30 {
		t.Errorf("Expected MaxHeight 30, got %f", stats.MaxHeight)
	}
	if stats.MaxLength != 20 {
		t.Errorf("Expected MaxLength 20, got %f", stats.MaxLength)
	}
	if stats.MaxArea != 30 {
		t.Errorf("Expected MaxArea 30, got %f", stats.MaxArea)
	}
}
