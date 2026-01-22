package api

import (
	"encoding/json"
	"net/http"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/tracker"
	"runtime"
	"sync"
)

type StatsHandler struct {
	tracker *tracker.Tracker
	poiMgr  *poi.Manager
	mu      sync.Mutex
	maxMem  uint64
}

func NewStatsHandler(t *tracker.Tracker, pm *poi.Manager) *StatsHandler {
	return &StatsHandler{
		tracker: t,
		poiMgr:  pm,
	}
}

type ProviderStatsDTO struct {
	CacheHits     int64 `json:"cache_hits"`
	CacheMisses   int64 `json:"cache_misses"`
	APISuccess    int64 `json:"api_success"`
	APIZeroResult int64 `json:"api_zero"`
	APIFailures   int64 `json:"api_errors"`
	HitRate       int64 `json:"hit_rate"`
	FreeTier      bool  `json:"free_tier"`
}

type SystemStats struct {
	MemoryAllocMB    uint64 `json:"memory_alloc_mb"`
	MemoryMaxAllocMB uint64 `json:"memory_max_mb"`
	Goroutines       int    `json:"goroutines"`
}

type TrackingStats struct {
	ActivePOIs int `json:"active_pois"`
}

type StatsResponse struct {
	System    SystemStats                 `json:"system"`
	Tracking  TrackingStats               `json:"tracking"`
	Providers map[string]ProviderStatsDTO `json:"providers"`
}

func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Snapshots
	snapshot := h.tracker.Snapshot()

	// System Stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	h.mu.Lock()
	if m.Alloc > h.maxMem {
		h.maxMem = m.Alloc
	}
	peak := h.maxMem
	h.mu.Unlock()

	// Build Response
	resp := StatsResponse{
		System: SystemStats{
			MemoryAllocMB:    bToMb(m.Alloc),
			MemoryMaxAllocMB: bToMb(peak),
			Goroutines:       runtime.NumGoroutine(),
		},
		Tracking: TrackingStats{
			ActivePOIs: h.poiMgr.ActiveCount(),
		},
		Providers: make(map[string]ProviderStatsDTO),
	}

	for provider, stats := range snapshot {
		totalCache := stats.CacheHits + stats.CacheMisses

		hitRate := int64(0)
		if totalCache > 0 {
			hitRate = (stats.CacheHits * 100) / totalCache
		}

		resp.Providers[provider] = ProviderStatsDTO{
			CacheHits:     stats.CacheHits,
			CacheMisses:   stats.CacheMisses,
			APISuccess:    stats.APISuccess,
			APIZeroResult: stats.APIZeroResult,
			APIFailures:   stats.APIFailures,
			HitRate:       hitRate,
			FreeTier:      stats.FreeTier,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		_ = err
	}
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
