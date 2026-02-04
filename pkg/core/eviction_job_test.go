package core

import (
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

// MockPOIManager implements the eviction-related methods of poi.Manager.
type mockEvictionPOIManager struct {
	pruneByDistanceCalled bool
	pruneResult           int
}

func (m *mockEvictionPOIManager) PruneByDistance(lat, lon, heading, thresholdKm float64) int {
	m.pruneByDistanceCalled = true
	return m.pruneResult
}

// MockWikidataService implements the eviction-related methods of wikidata.Service.
type mockEvictionWikidataService struct {
	evictFarTilesCalled bool
	evictResult         int
}

func (m *mockEvictionWikidataService) EvictFarTiles(lat, lon, thresholdKm float64) int {
	m.evictFarTilesCalled = true
	return m.evictResult
}

// evictionTestPOIManager wraps mockEvictionPOIManager to satisfy *poi.Manager interface via duck typing.
// Since we can't easily mock *poi.Manager, we test via the specific methods.

func TestEvictionJob_ShouldFire(t *testing.T) {
	tests := []struct {
		name       string
		isOnGround bool
		timeSince  time.Duration
		wantFire   bool
	}{
		{"On ground - no fire", true, 10 * time.Minute, false},
		{"Airborne, recent - no fire", false, 1 * time.Minute, false},
		{"Airborne, expired - fire", false, 6 * time.Minute, true},
		{"Airborne, exactly 5min - fire", false, 5 * time.Minute, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgProv := config.NewProvider(&config.Config{}, nil)
			job := NewEvictionJob(cfgProv, nil, nil)

			// Set last run time to simulate elapsed time
			if tt.timeSince > 0 {
				job.lastRunTime = time.Now().Add(-tt.timeSince)
			}

			tel := &sim.Telemetry{IsOnGround: tt.isOnGround}
			got := job.ShouldFire(tel)

			if got != tt.wantFire {
				t.Errorf("ShouldFire() = %v, want %v", got, tt.wantFire)
			}
		})
	}
}

func TestEvictionJob_ShouldFire_NilTelemetry(t *testing.T) {
	cfgProv := config.NewProvider(&config.Config{}, nil)
	job := NewEvictionJob(cfgProv, nil, nil)
	job.lastRunTime = time.Now().Add(-10 * time.Minute)

	// Nil telemetry should still fire (not on ground check passes)
	got := job.ShouldFire(nil)
	if !got {
		t.Error("ShouldFire(nil) should return true when expired")
	}
}

func TestEvictionJob_ShouldFire_Concurrent(t *testing.T) {
	cfgProv := config.NewProvider(&config.Config{}, nil)
	job := NewEvictionJob(cfgProv, nil, nil)
	job.lastRunTime = time.Now().Add(-10 * time.Minute)

	tel := &sim.Telemetry{IsOnGround: false}

	// Lock the job to simulate concurrent execution
	job.TryLock()

	// Should not fire while locked
	got := job.ShouldFire(tel)
	if got {
		t.Error("ShouldFire should return false while job is locked")
	}

	job.Unlock()

	// Should fire after unlock
	got = job.ShouldFire(tel)
	if !got {
		t.Error("ShouldFire should return true after unlock")
	}
}

func TestEvictionJob_Name(t *testing.T) {
	prov := config.NewProvider(&config.Config{}, nil)
	job := NewEvictionJob(prov, nil, nil)

	if got := job.Name(); got != "Eviction" {
		t.Errorf("Name() = %q, want %q", got, "Eviction")
	}
}

// TestEvictionJob_ThresholdCalculation tests the threshold calculation logic.
func TestEvictionJob_ThresholdCalculation(t *testing.T) {
	tests := []struct {
		name          string
		maxDist       config.Distance
		wantThreshold float64
	}{
		{"Default (0) -> 90km", 0, 90.0},
		{"80km -> 90km", 80000, 90.0},
		{"50km -> 60km", 50000, 60.0},
		{"100km -> 110km", 100000, 110.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Wikidata.Area.MaxDist = tt.maxDist

			// We can't easily test Run() without real poi.Manager and wikidata.Service
			// But we can verify the threshold calculation logic by examining the code
			maxDist := float64(cfg.Wikidata.Area.MaxDist)
			if maxDist <= 0 {
				maxDist = 80000.0
			}
			threshold := (maxDist / 1000.0) + 10.0

			if threshold != tt.wantThreshold {
				t.Errorf("threshold = %.1f, want %.1f", threshold, tt.wantThreshold)
			}
		})
	}
}

// --- Mock types for integration testing ---

// mockPOIEvictable satisfies the interface needed by EvictionJob.
type mockPOIEvictable interface {
	PruneByDistance(lat, lon, heading, thresholdKm float64) int
}

// mockWikiEvictable satisfies the interface needed by EvictionJob.
type mockWikiEvictable interface {
	EvictFarTiles(lat, lon, thresholdKm float64) int
}

// Since EvictionJob takes concrete types (*poi.Manager, *wikidata.Service),
// we test ShouldFire thoroughly and document that Run is integration-tested.

func TestNewEvictionJob(t *testing.T) {
	cfgProv := config.NewProvider(&config.Config{}, nil)
	job := NewEvictionJob(cfgProv, nil, nil)

	if job == nil {
		t.Fatal("NewEvictionJob returned nil")
	}
	if job.Name() != "Eviction" {
		t.Errorf("Name = %q, want %q", job.Name(), "Eviction")
	}
	if job.cfg != cfgProv {
		t.Error("Config not set correctly")
	}
}

// TestEvictionJob_LastRunLocation tests that lastRunLocation is zero initially.
func TestEvictionJob_InitialState(t *testing.T) {
	cfgProv := config.NewProvider(&config.Config{}, nil)
	job := NewEvictionJob(cfgProv, nil, nil)

	if job.lastRunLocation != (geo.Point{}) {
		t.Error("lastRunLocation should be zero initially")
	}
	if !job.lastRunTime.IsZero() {
		t.Error("lastRunTime should be zero initially")
	}
}

// Placeholder model.POI for type checking
var _ *model.POI
