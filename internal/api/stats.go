package api

import (
	"encoding/json"
	"net/http"
	"os"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/tracker"
	"sync"
	"time"
)

type componentState struct {
	lastCPUNS int64
	lastTime  time.Time
	maxMem    uint64
	maxCPU    float64
}

type StatsHandler struct {
	tracker     *tracker.Tracker
	poiMgr      *poi.Manager
	llmFallback []string
	mu          sync.Mutex
	states      map[string]*componentState
}

func NewStatsHandler(t *tracker.Tracker, pm *poi.Manager, fallback []string) *StatsHandler {
	return &StatsHandler{
		tracker:     t,
		poiMgr:      pm,
		llmFallback: fallback,
		states:      make(map[string]*componentState),
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

type ComponentStats struct {
	Name        string  `json:"name"`
	MemoryMB    uint64  `json:"memory_mb"`
	MemoryMaxMB uint64  `json:"memory_max_mb"`
	CPUSec      float64 `json:"cpu_sec"`     // Seconds per second
	CPUMaxSec   float64 `json:"cpu_max_sec"` // Peak
}

type TrackingStats struct {
	ActivePOIs int `json:"active_pois"`
}

type StatsResponse struct {
	Diagnostics []ComponentStats            `json:"diagnostics"`
	Tracking    TrackingStats               `json:"tracking"`
	Providers   map[string]ProviderStatsDTO `json:"providers"`
	LLMFallback []string                    `json:"llm_fallback"`
}

func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot := h.tracker.Snapshot()

	// 1. Diagnostics Aggregation
	h.mu.Lock()
	diagnostics := h.gatherDiagnostics()
	h.mu.Unlock()

	// 2. Build Response
	resp := StatsResponse{
		Diagnostics: diagnostics,
		Tracking: TrackingStats{
			ActivePOIs: h.poiMgr.ActiveCount(),
		},
		Providers:   make(map[string]ProviderStatsDTO),
		LLMFallback: h.llmFallback,
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
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *StatsHandler) gatherDiagnostics() []ComponentStats {
	now := time.Now()
	var results []ComponentStats

	selfPID := os.Getpid()
	parentPID := os.Getppid()

	// Components to track
	targets := []struct {
		name string
		pids []int
	}{
		{"Server", []int{selfPID}},
	}

	// Only include GUI and Webview if we have a valid parent
	if parentPID > 1 {
		targets = append(targets, struct {
			name string
			pids []int
		}{"GUI", []int{parentPID}})

		// Find children of parent (WebView2 tree)
		if children, err := GetChildPIDs(parentPID); err == nil {
			var webviewPIDs []int
			for _, child := range children {
				if child != selfPID {
					webviewPIDs = append(webviewPIDs, child)
				}
			}
			if len(webviewPIDs) > 0 {
				targets = append(targets, struct {
					name string
					pids []int
				}{"Webview", webviewPIDs})
			}
		}
	}

	for _, t := range targets {
		var totalCPU int64
		var totalMem uint64

		for _, pid := range t.pids {
			cpu, mem, err := GetProcessStats(pid)
			if err == nil {
				totalCPU += cpu
				totalMem += mem
			}
		}

		state, ok := h.states[t.name]
		if !ok {
			state = &componentState{lastTime: now, lastCPUNS: totalCPU}
			h.states[t.name] = state
		}

		// Calculate CPU Delta (sec per sec)
		duration := now.Sub(state.lastTime).Seconds()
		cpuSec := 0.0
		if duration > 0 {
			cpuDeltaNS := totalCPU - state.lastCPUNS
			if cpuDeltaNS < 0 {
				cpuDeltaNS = 0 // Counter reset or wrap (unlikely on Windows)
			}
			cpuSec = float64(cpuDeltaNS) / 1e9 / duration
		}

		// Update State
		state.lastCPUNS = totalCPU
		state.lastTime = now
		if totalMem > state.maxMem {
			state.maxMem = totalMem
		}
		if cpuSec > state.maxCPU {
			state.maxCPU = cpuSec
		}

		results = append(results, ComponentStats{
			Name:        t.name,
			MemoryMB:    bToMb(totalMem),
			MemoryMaxMB: bToMb(state.maxMem),
			CPUSec:      cpuSec,
			CPUMaxSec:   state.maxCPU,
		})
	}

	// Always ensure Server is first, regardless of other components
	return results
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
