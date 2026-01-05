package tracker

import (
	"sync"
	"sync/atomic"
)

// Tracker tracks usage statistics per provider.
type Tracker struct {
	mu    sync.RWMutex
	stats map[string]*ProviderStats
}

// ProviderStats holds metrics for a specific provider.
// Fields are accessed atomically.
type ProviderStats struct {
	CacheHits     int64
	CacheMisses   int64
	APISuccess    int64
	APIFailures   int64
	APIZeroResult int64
}

// New creates a new Tracker.
func New() *Tracker {
	return &Tracker{
		stats: make(map[string]*ProviderStats),
	}
}

// getStats returns the stats object for a provider, creating it if needed.
func (t *Tracker) getStats(provider string) *ProviderStats {
	t.mu.RLock()
	s, ok := t.stats[provider]
	t.mu.RUnlock()
	if ok {
		return s
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	// Double check
	if s, ok = t.stats[provider]; ok {
		return s
	}
	s = &ProviderStats{}
	t.stats[provider] = s
	return s
}

// TrackCacheHit increments the cache hit counter.
func (t *Tracker) TrackCacheHit(provider string) {
	atomic.AddInt64(&t.getStats(provider).CacheHits, 1)
}

func (t *Tracker) TrackCacheMiss(provider string) {
	atomic.AddInt64(&t.getStats(provider).CacheMisses, 1)
}

func (t *Tracker) TrackAPISuccess(provider string) {
	atomic.AddInt64(&t.getStats(provider).APISuccess, 1)
}

func (t *Tracker) TrackAPIFailure(provider string) {
	atomic.AddInt64(&t.getStats(provider).APIFailures, 1)
}

func (t *Tracker) TrackAPIZero(provider string) {
	atomic.AddInt64(&t.getStats(provider).APIZeroResult, 1)
}

// GetSnapshot returns a copy of the current stats.
func (t *Tracker) Snapshot() map[string]ProviderStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]ProviderStats)
	for k, v := range t.stats {
		result[k] = ProviderStats{
			CacheHits:     atomic.LoadInt64(&v.CacheHits),
			CacheMisses:   atomic.LoadInt64(&v.CacheMisses),
			APISuccess:    atomic.LoadInt64(&v.APISuccess),
			APIFailures:   atomic.LoadInt64(&v.APIFailures),
			APIZeroResult: atomic.LoadInt64(&v.APIZeroResult),
		}
	}
	return result
}
