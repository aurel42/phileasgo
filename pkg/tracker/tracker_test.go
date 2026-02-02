package tracker

import (
	"testing"
)

func TestTracker(t *testing.T) {
	tr := New()
	provider := "test.provider"

	// Test Initial State
	stats := tr.Snapshot()
	if len(stats) != 0 {
		t.Errorf("Expected empty stats, got %d", len(stats))
	}

	// Test Tracking
	tr.TrackCacheHit(provider)
	tr.TrackCacheMiss(provider)
	tr.TrackAPISuccess(provider)
	tr.TrackAPIFailure(provider)
	tr.TrackAPIZero(provider)

	// Verify Snapshot
	stats = tr.Snapshot()
	pStats, ok := stats[provider]
	if !ok {
		t.Fatalf("Expected stats for provider %s", provider)
	}

	if pStats.CacheHits != 1 {
		t.Errorf("Expected 1 CacheHit, got %d", pStats.CacheHits)
	}
	if pStats.CacheMisses != 1 {
		t.Errorf("Expected 1 CacheMiss, got %d", pStats.CacheMisses)
	}
	if pStats.APISuccess != 1 {
		t.Errorf("Expected 1 APISuccess, got %d", pStats.APISuccess)
	}
	if pStats.APIFailures != 1 {
		t.Errorf("Expected 1 APIFailure, got %d", pStats.APIFailures)
	}
	if pStats.APIZeroResult != 1 {
		t.Errorf("Expected 1 APIZeroResult, got %d", pStats.APIZeroResult)
	}
}

func TestResetPreservesFreeTier(t *testing.T) {
	tr := New()
	provider := "free.provider"

	// Setup: Mark as free and add some stats
	tr.SetFreeTier(provider, true)
	tr.TrackAPISuccess(provider)

	// Verify Setup
	stats := tr.Snapshot()
	if !stats[provider].FreeTier {
		t.Fatal("Pre-Reset: Expected FreeTier to be true")
	}
	if stats[provider].APISuccess != 1 {
		t.Fatal("Pre-Reset: Expected APISuccess to be 1")
	}

	// Action: Reset
	tr.Reset()

	// Verify Result
	stats = tr.Snapshot()
	s, ok := stats[provider]
	if !ok {
		t.Fatal("Post-Reset: Provider should still exist in map")
	}

	if !s.FreeTier {
		t.Error("Post-Reset: FreeTier should still be true")
	}
	if s.APISuccess != 0 {
		t.Errorf("Post-Reset: APISuccess should be 0, got %d", s.APISuccess)
	}
}
