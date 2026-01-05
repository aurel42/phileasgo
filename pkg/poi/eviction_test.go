package poi

import (
	"log/slog"
	"os"
	"phileasgo/pkg/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_PruneByDistance(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := &Manager{
		logger:      logger,
		trackedPOIs: make(map[string]*model.POI),
	}

	// 1. Close POI (Front)
	m.trackedPOIs["close-front"] = &model.POI{
		WikidataID: "close-front", Lat: 10.001, Lon: 10.0,
	}
	// 2. Far POI (Front)
	m.trackedPOIs["far-front"] = &model.POI{
		WikidataID: "far-front", Lat: 11.0, Lon: 10.0, // ~111km away North
	}
	// 3. Far POI (Behind)
	m.trackedPOIs["far-behind"] = &model.POI{
		WikidataID: "far-behind", Lat: 9.0, Lon: 10.0, // ~111km away South
	}
	// 4. Close POI (Behind)
	m.trackedPOIs["close-behind"] = &model.POI{
		WikidataID: "close-behind", Lat: 9.999, Lon: 10.0,
	}

	// Aircraft at 10.0, 10.0, Heading 0 (North)
	aircraftLat := 10.0
	aircraftLon := 10.0
	heading := 0.0
	thresholdKm := 50.0

	// Act
	count := m.PruneByDistance(aircraftLat, aircraftLon, heading, thresholdKm)

	// Assert
	assert.Equal(t, 1, count, "Should prune exactly 1 POI")

	_, exists := m.trackedPOIs["far-behind"]
	assert.False(t, exists, "far-behind should be evicted")

	_, exists = m.trackedPOIs["far-front"]
	assert.True(t, exists, "far-front should stay (it's ahead)")

	_, exists = m.trackedPOIs["close-front"]
	assert.True(t, exists, "close-front should stay")

	_, exists = m.trackedPOIs["close-behind"]
	assert.True(t, exists, "close-behind should stay (too close)")
}
