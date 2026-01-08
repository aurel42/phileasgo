package core

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/wikidata"
)

// EvictionJob periodically cleans up distant POIs and cache entries.
// This allows memory to be freed and trailing data to be evicted,
// ensuring that if the aircraft turns around, data can be re-loaded.
type EvictionJob struct {
	BaseJob
	appCfg  *config.Config
	poi     *poi.Manager
	wikiSvc *wikidata.Service

	lastRunLocation geo.Point
	lastRunTime     time.Time
}

func NewEvictionJob(
	appCfg *config.Config,
	poiMgr *poi.Manager,
	wikiSvc *wikidata.Service,
) *EvictionJob {
	return &EvictionJob{
		BaseJob: NewBaseJob("Eviction"),
		appCfg:  appCfg,
		poi:     poiMgr,
		wikiSvc: wikiSvc,
	}
}

func (j *EvictionJob) ShouldFire(t *sim.Telemetry) bool {
	// Prevent eviction on ground to keep loaded POIs available while parked/taxiing.
	if t != nil && t.IsOnGround {
		return false
	}

	// Simple periodic check
	// We use 300s (5m) to allow plenty of time for turn-arounds before cache eviction.
	if j.TryLock() {
		j.Unlock()
	} else {
		return false
	}

	if time.Since(j.lastRunTime) >= 300*time.Second {
		return true
	}

	return false
}

func (j *EvictionJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	j.lastRunTime = time.Now()
	j.lastRunLocation = geo.Point{Lat: t.Latitude, Lon: t.Longitude}

	// 1. Calculate Threshold
	// "more than max_dist_km + 10km behind us"
	// Current config default is 80.0, so valid threshold is 90.0
	maxDist := j.appCfg.Wikidata.Area.MaxDist
	if maxDist <= 0 {
		maxDist = 80.0
	}
	thresholdKm := maxDist + 10.0

	// 2. Evict from POI Manager (Limit Memory)
	// Only evict if BEHIND us to allow pre-fetching ahead.
	start := time.Now()
	evictedPOIs := j.poi.PruneByDistance(t.Latitude, t.Longitude, t.Heading, thresholdKm)

	// 3. Evict from Wikidata Service Cache (Allow Re-hydration)
	// We evict tiles that are > thresholdKm away regardless of direction?
	// User said "We already have code to let the dynamic configuration reprocess... adapt it so it'll help us get the evicted POIs back"
	// If we turn around, we fly INTO the tile.
	// If the tile is in `recentTiles` (memory), the Scheduler sees it but the Service skips it.
	// So we MUST remove it from `recentTiles` when it is far away.
	// Doing it at the same threshold (90km) is safe because the fetch radius is usually smaller.
	evictedTiles := j.wikiSvc.EvictFarTiles(t.Latitude, t.Longitude, thresholdKm)

	if evictedPOIs > 0 || evictedTiles > 0 {
		slog.Debug("Eviction Job Completed",
			"evicted_pois", evictedPOIs,
			"evicted_tiles", evictedTiles,
			"duration", time.Since(start),
			"threshold_km", thresholdKm,
		)
	}
}
