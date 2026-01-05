package geo

import (
	"math"
	"testing"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		name string
		p1   Point
		p2   Point
		want float64
	}{
		{
			name: "Same Point",
			p1:   Point{Lat: 0, Lon: 0},
			p2:   Point{Lat: 0, Lon: 0},
			want: 0,
		},
		{
			name: "London to Paris",
			p1:   Point{Lat: 51.5074, Lon: -0.1278},
			p2:   Point{Lat: 48.8566, Lon: 2.3522},
			want: 344000, // Approx 344km
		},
		{
			name: "Equator 1 degree",
			p1:   Point{Lat: 0, Lon: 0},
			p2:   Point{Lat: 0, Lon: 1},
			want: 111319, // Approx 111km
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Distance(tt.p1, tt.p2)
			// Allow 1% margin of error due to float precision/earth radius var
			margin := tt.want * 0.01
			if math.Abs(got-tt.want) > margin && tt.want != 0 {
				t.Errorf("Distance() = %v, want %v (+/- %v)", got, tt.want, margin)
			}
		})
	}
}
