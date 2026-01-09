package poi

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_PruneByDistance(t *testing.T) {
	// Table-driven test for PruneByDistance
	// Setup: Aircraft at (0,0) heading North (0 deg)
	acLat, acLon := 0.0, 0.0
	heading := 0.0
	thresholdKm := 10.0

	// 1 degree lat is approx 111km. 0.1 deg is 11.1km.

	tests := []struct {
		name        string
		poiLat      float64
		poiLon      float64
		shouldPrune bool
		description string
	}{
		{
			name:   "Close Ahead",
			poiLat: 0.05, poiLon: 0.0, // ~5.5km North (Ahead)
			shouldPrune: false,
			description: "Within threshold, ahead",
		},
		{
			name:   "Far Ahead",
			poiLat: 0.2, poiLon: 0.0, // ~22km North (Ahead)
			shouldPrune: false, // DistancePruning only prunes BEHIND
			description: "Outside threshold calculation logic generally keeps ahead POIs, but let's verify intent.",
		},
		{
			name:   "Close Behind",
			poiLat: -0.05, poiLon: 0.0, // ~5.5km South (Behind)
			shouldPrune: false,
			description: "Behind but within threshold (keep)",
		},
		{
			name:   "Far Behind",
			poiLat: -0.2, poiLon: 0.0, // ~22km South (Behind)
			shouldPrune: true,
			description: "Behind and outside threshold (prune)",
		},
		{
			name:   "Far Side (Behind)",
			poiLat: -0.1, poiLon: 0.2, // South-West-ish, far
			shouldPrune: true,
			description: "Behind/Side and far (prune)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock basic dependencies
			// We can use the mock store from manager_test.go if exported,
			// OR just stub it here since we don't need persistence for this test.
			// Ideally we reuse NewMockStore but it's in manager_test.go package poi.
			// Since we are in package poi, we can access NewMockStore if it is in the same package test files.

			// Note: manager_test.go defines MockStore and NewMockStore.
			// 'go test' compiles all _test.go files in the package together, so it should be visible.

			mgr := NewManager(&config.Config{}, NewMockStore(), nil)
			p := &model.POI{WikidataID: tt.name, Lat: tt.poiLat, Lon: tt.poiLon}
			_ = mgr.TrackPOI(context.Background(), p)

			count := mgr.PruneByDistance(acLat, acLon, heading, thresholdKm)

			if tt.shouldPrune {
				assert.Equal(t, 1, count, "Expected 1 prune for "+tt.description)
				_, exists := mgr.trackedPOIs[tt.name]
				assert.False(t, exists, "POI should be evicted")
			} else {
				assert.Equal(t, 0, count, "Expected 0 prune for "+tt.description)
				_, exists := mgr.trackedPOIs[tt.name]
				assert.True(t, exists, "POI should remain")
			}
		})
	}
}
