package api

import (
	"encoding/json"
	"net/http"
	"os"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/tracker"
	"runtime"
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

// GoMemStats provides a breakdown of Go runtime memory usage.
type GoMemStats struct {
	HeapAllocMB   float64 `json:"heap_alloc_mb"`   // Live heap objects
	HeapInuseMB   float64 `json:"heap_inuse_mb"`   // Heap spans in use (includes fragmentation)
	HeapIdleMB    float64 `json:"heap_idle_mb"`     // Heap spans not in use (returned or reusable)
	HeapSysMB     float64 `json:"heap_sys_mb"`      // Heap memory obtained from OS
	StackInuseMB  float64 `json:"stack_inuse_mb"`   // Stack memory
	MSpanInuseMB  float64 `json:"mspan_inuse_mb"`   // Runtime mspan structures
	MCacheInuseMB float64 `json:"mcache_inuse_mb"`  // Runtime mcache structures
	GCSysMB       float64 `json:"gc_sys_mb"`        // GC metadata
	OtherSysMB    float64 `json:"other_sys_mb"`     // Other runtime allocations
	TotalSysMB    float64 `json:"total_sys_mb"`     // Total memory from OS
	NumGC         uint32  `json:"num_gc"`           // Completed GC cycles
	NumGoroutine  int     `json:"num_goroutine"`    // Active goroutines
	HeapObjects   uint64  `json:"heap_objects"`     // Live heap object count
}

type StatsResponse struct {
	Diagnostics []ComponentStats            `json:"diagnostics"`
	GoMem       GoMemStats                  `json:"go_mem"`
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

	// 2. Go runtime memory stats
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	goMem := GoMemStats{
		HeapAllocMB:   bytesToMBf(ms.HeapAlloc),
		HeapInuseMB:   bytesToMBf(ms.HeapInuse),
		HeapIdleMB:    bytesToMBf(ms.HeapIdle),
		HeapSysMB:     bytesToMBf(ms.HeapSys),
		StackInuseMB:  bytesToMBf(ms.StackInuse),
		MSpanInuseMB:  bytesToMBf(ms.MSpanInuse),
		MCacheInuseMB: bytesToMBf(ms.MCacheInuse),
		GCSysMB:       bytesToMBf(ms.GCSys),
		OtherSysMB:    bytesToMBf(ms.OtherSys),
		TotalSysMB:    bytesToMBf(ms.Sys),
		NumGC:         ms.NumGC,
		NumGoroutine:  runtime.NumGoroutine(),
		HeapObjects:   ms.HeapObjects,
	}

	// 3. Build Response
	resp := StatsResponse{
		Diagnostics: diagnostics,
		GoMem:       goMem,
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

func bytesToMBf(b uint64) float64 {
	return float64(b) / 1024 / 1024
}
