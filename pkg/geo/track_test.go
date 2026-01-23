package geo

import (
	"math"
	"testing"
)

func TestTrackBuffer(t *testing.T) {
	// Table-driven test for rolling track calculation
	tests := []struct {
		name       string
		windowSize int
		points     []Point
		wantTracks []float64 // Expected track after EACH push
	}{
		{
			name:       "Standard 3-Sample Window",
			windowSize: 3,
			points: []Point{
				{Lat: 10, Lon: 20}, // 1st: return default
				{Lat: 11, Lon: 20}, // 2nd: North (0)
				{Lat: 11, Lon: 21}, // 3rd: NE based on 10,20 -> 11,21 (approx 45)
				{Lat: 10, Lon: 21}, // 4th: SE based on 11,20 -> 10,21 (approx 135)
			},
			wantTracks: []float64{
				99,  // Default provided in test
				0,   // 10,20 -> 11,20
				45,  // 10,20 -> 11,21
				135, // 11,20 -> 10,21
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewTrackBuffer(tt.windowSize)
			for i, p := range tt.points {
				got := b.Push(p, 99)
				// We use approx comparison for Bearing since math is complex
				if math.Abs(got-tt.wantTracks[i]) > 1.0 {
					t.Errorf("Step %d: Push() = %v, want approx %v", i, got, tt.wantTracks[i])
				}
			}
		})
	}
}

func TestTrackBuffer_Reset(t *testing.T) {
	b := NewTrackBuffer(5)
	b.Push(Point{10, 20}, 0)
	b.Push(Point{11, 20}, 0)

	if len(b.samples) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(b.samples))
	}

	b.Reset()
	if len(b.samples) != 0 {
		t.Errorf("Expected 0 samples after reset, got %d", len(b.samples))
	}
}
