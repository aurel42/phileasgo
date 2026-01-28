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

func TestRescueBatch(t *testing.T) {
	h1 := 10.0
	h2 := 500.0
	candidates := []Article{
		{ID: "Q1", Height: &h1},
		{ID: "Q2", Height: &h2},
	}

	localMax := TileStats{MaxHeight: 500.0}
	medians := MedianStats{MedianHeight: 50.0} // Threshold = 100

	rescued := Batch(candidates, localMax, medians)

	if len(rescued) != 1 {
		t.Fatalf("Expected 1 rescued article, got %d", len(rescued))
	}
	if rescued[0].ID != "Q2" {
		t.Errorf("Expected Q2 to be rescued, got %s", rescued[0].ID)
	}
	if rescued[0].Category != "height" {
		t.Errorf("Expected category 'height', got %s", rescued[0].Category)
	}

	// Test Length Rescue
	l1, l2 := 10.0, 500.0
	candidatesLength := []Article{
		{ID: "Q3", Length: &l1},
		{ID: "Q4", Length: &l2},
	}
	localMaxL := TileStats{MaxLength: 500.0}
	mediansL := MedianStats{MedianLength: 50.0}
	rescuedL := Batch(candidatesLength, localMaxL, mediansL)
	if len(rescuedL) != 1 || rescuedL[0].ID != "Q4" || rescuedL[0].Category != "length" {
		t.Errorf("Length rescue failed: got %v", rescuedL)
	}

	// Test Area Rescue
	a1, a2 := 10.0, 500.0
	candidatesArea := []Article{
		{ID: "Q5", Area: &a1},
		{ID: "Q6", Area: &a2},
	}
	localMaxA := TileStats{MaxArea: 500.0}
	mediansA := MedianStats{MedianArea: 50.0}
	rescuedA := Batch(candidatesArea, localMaxA, mediansA)
	if len(rescuedA) != 1 || rescuedA[0].ID != "Q6" || rescuedA[0].Category != "area" {
		t.Errorf("Area rescue failed: got %v", rescuedA)
	}
}

func TestRescueBatch_Duplicate(t *testing.T) {
	h := 500.0
	candidates := []Article{
		{ID: "Q1", Height: &h, Length: &h},
	}

	localMax := TileStats{MaxHeight: 500.0, MaxLength: 500.0}
	medians := MedianStats{MedianHeight: 50.0, MedianLength: 50.0}

	rescued := Batch(candidates, localMax, medians)

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
