package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"phileasgo/pkg/sim"
)

// TestBaseJob_LockUnlock tests the atomic lock behavior.
func TestBaseJob_LockUnlock(t *testing.T) {
	tests := []struct {
		name        string
		prelock     bool
		wantTryLock bool
	}{
		{"Unlocked - TryLock succeeds", false, true},
		{"Prelocked - TryLock fails", true, true}, // First TryLock succeeds, second fails
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBaseJob("test")

			if tt.prelock {
				// First lock should succeed
				if !b.TryLock() {
					t.Fatal("First TryLock should succeed")
				}
				// Second lock should fail
				if b.TryLock() {
					t.Error("Second TryLock should fail when already locked")
				}
				b.Unlock()
				// After unlock, should succeed again
				if !b.TryLock() {
					t.Error("TryLock should succeed after Unlock")
				}
			} else {
				if got := b.TryLock(); got != tt.wantTryLock {
					t.Errorf("TryLock() = %v, want %v", got, tt.wantTryLock)
				}
			}
		})
	}
}

// TestBaseJob_Name tests the Name method.
func TestBaseJob_Name(t *testing.T) {
	tests := []struct {
		name     string
		jobName  string
		wantName string
	}{
		{"Simple name", "TestJob", "TestJob"},
		{"Empty name", "", ""},
		{"Unicode name", "作业", "作业"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBaseJob(tt.jobName)
			if got := b.Name(); got != tt.wantName {
				t.Errorf("Name() = %v, want %v", got, tt.wantName)
			}
		})
	}
}

// TestDistanceJob_ShouldFire tests the distance-based trigger logic.
func TestDistanceJob_ShouldFire(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
		positions []struct {
			lat, lon float64
		}
		wantFires []bool
	}{
		{
			name:      "First run always fires",
			threshold: 1000,
			positions: []struct{ lat, lon float64 }{
				{0, 0},
			},
			wantFires: []bool{true},
		},
		{
			name:      "Below threshold - no fire",
			threshold: 10000, // 10km
			positions: []struct{ lat, lon float64 }{
				{0, 0},     // First run fires
				{0, 0.001}, // ~100m, below threshold
			},
			wantFires: []bool{true, false},
		},
		{
			name:      "Above threshold - fires",
			threshold: 1000, // 1km
			positions: []struct{ lat, lon float64 }{
				{0, 0},   // First fires
				{0, 0.1}, // ~11km, above threshold
			},
			wantFires: []bool{true, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := NewDistanceJob("test", tt.threshold, func(ctx context.Context, tel sim.Telemetry) {})

			for i, pos := range tt.positions {
				tel := sim.Telemetry{Latitude: pos.lat, Longitude: pos.lon}

				got := job.ShouldFire(&tel)
				if got != tt.wantFires[i] {
					t.Errorf("Position %d: ShouldFire() = %v, want %v", i, got, tt.wantFires[i])
				}

				// If should fire, actually run to update position
				if got {
					job.Run(context.Background(), &tel)
				}
			}
		})
	}
}

// TestDistanceJob_Running tests that job doesn't fire while running.
func TestDistanceJob_Running(t *testing.T) {
	var wg sync.WaitGroup
	started := make(chan struct{})
	finish := make(chan struct{})

	job := NewDistanceJob("test", 0, func(ctx context.Context, tel sim.Telemetry) {
		close(started)
		<-finish
	})

	tel := sim.Telemetry{Latitude: 0, Longitude: 0}

	// Start the job in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		job.Run(context.Background(), &tel)
	}()

	// Wait for job to start
	<-started

	// While running, ShouldFire should return false
	if job.ShouldFire(&tel) {
		t.Error("ShouldFire should return false while job is running")
	}

	// Allow job to finish
	close(finish)
	wg.Wait()

	// After job finishes, ShouldFire should work again (distance=0 so won't fire)
	job.firstRun = true
	if !job.ShouldFire(&tel) {
		t.Error("ShouldFire should return true (first run) after job finishes")
	}
}

// TestTimeJob_ShouldFire tests the time-based trigger logic.
func TestTimeJob_ShouldFire(t *testing.T) {
	tests := []struct {
		name      string
		threshold time.Duration
		wait      time.Duration
		wantFire  bool
	}{
		{"First run always fires", 1 * time.Hour, 0, true},
		{"Below threshold - no fire", 100 * time.Millisecond, 0, false}, // After first run
		{"Above threshold - fires", 10 * time.Millisecond, 20 * time.Millisecond, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := NewTimeJob("test", tt.threshold, func(ctx context.Context, tel sim.Telemetry) {})
			tel := sim.Telemetry{}

			// First run
			if !job.ShouldFire(&tel) {
				t.Fatal("First run should always fire")
			}
			job.Run(context.Background(), &tel)

			// Wait if specified
			if tt.wait > 0 {
				time.Sleep(tt.wait)
			}

			// Check second fire
			got := job.ShouldFire(&tel)
			if tt.name == "First run always fires" {
				// Skip second check for first run test
				return
			}
			if got != tt.wantFire {
				t.Errorf("ShouldFire() = %v, want %v", got, tt.wantFire)
			}
		})
	}
}

// TestTimeJob_Running tests that job doesn't fire while running.
func TestTimeJob_Running(t *testing.T) {
	var running int32
	job := NewTimeJob("test", 0, func(ctx context.Context, tel sim.Telemetry) {
		atomic.StoreInt32(&running, 1)
		time.Sleep(50 * time.Millisecond)
		atomic.StoreInt32(&running, 0)
	})

	tel := sim.Telemetry{}

	// First run
	go job.Run(context.Background(), &tel)

	// Wait for job to start
	time.Sleep(10 * time.Millisecond)

	// While running, ShouldFire should return false
	if job.ShouldFire(&tel) {
		t.Error("ShouldFire should return false while job is running")
	}
}
