package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

type mockSchedGeoProvider struct{}

func (m *mockSchedGeoProvider) GetLocation(lat, lon float64) model.LocationInfo {
	return model.LocationInfo{
		CityName:    "London",
		CountryCode: "GB",
	}
}

func (m *mockSchedGeoProvider) ReorderFeatures(lat, lon float64) {
	// no-op
}

// mockSimClient implements sim.Client
type mockSimClient struct {
	tel   sim.Telemetry
	err   error
	state sim.State
	mu    sync.Mutex
}

func (m *mockSimClient) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tel, m.err
}

func (m *mockSimClient) GetState() sim.State {
	if m.state == "" {
		return sim.StateActive
	}
	return m.state
}

func (m *mockSimClient) GetLastTransition(stage string) time.Time { return time.Time{} }

func (m *mockSimClient) SetPredictionWindow(d time.Duration) {}

func (m *mockSimClient) Close() error { return nil }

func (m *mockSimClient) SetTelemetry(t *sim.Telemetry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tel = *t
}

func (m *mockSimClient) GetStageState() sim.StageState        { return sim.StageState{} }
func (m *mockSimClient) RestoreStageState(s sim.StageState)   {}
func (m *mockSimClient) SetEventRecorder(r sim.EventRecorder) {}

func TestScheduler_JobExecution(t *testing.T) {
	// Setup
	cfg := config.DefaultConfig()
	cfg.Ticker.TelemetryLoop = config.Duration(10 * time.Millisecond) // Fast loop

	mockSim := &mockSimClient{state: sim.StateActive}
	prov := config.NewProvider(cfg, nil)
	sched := NewScheduler(prov, mockSim, nil, &mockSchedGeoProvider{})

	// job fired latch
	var firedCount int32
	fired := make(chan struct{})

	// Distance Job
	job := NewDistanceJob("TestDist", 100, func(ctx context.Context, tel sim.Telemetry) {
		atomic.AddInt32(&firedCount, 1)
		fired <- struct{}{}
	})
	sched.AddJob(job)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Scheduler
	go sched.Start(ctx)

	// 1. Initial State: (0,0) -> Job inits lastPos, no fire (firstRun=true returns true? Wait, logic check)
	// My logic: if j.firstRun { return true }
	// So it fires immediately on first tick.
	select {
	case <-fired:
		// OK, first firing expected to initialize
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Job should have fired once for initialization")
	}

	// 2. Move < Threshold (50m)
	mockSim.SetTelemetry(&sim.Telemetry{Latitude: 0.00045, Longitude: 0}) // ~50m
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&firedCount) > 1 {
		t.Error("Job fired when movement was small")
	}

	// 3. Move > Threshold (150m total)
	mockSim.SetTelemetry(&sim.Telemetry{Latitude: 0.00135, Longitude: 0}) // ~150m
	select {
	case <-fired:
		// OK
	case <-time.After(200 * time.Millisecond):
		t.Error("Job should have fired after movement")
	}
}

func TestJob_Concurrency(t *testing.T) {
	// Ensure job doesn't double fire if slow
	job := NewBaseJob("SlowJob")

	// Simulate "ShouldFire" check
	if !job.TryLock() {
		t.Fatal("Should lock when free")
	}

	// Simulate re-entry
	if job.TryLock() {
		t.Fatal("Should fail lock when busy")
	}

	job.Unlock()

	if !job.TryLock() {
		t.Fatal("Should lock again after unlock")
	}
}

// mockStatefulSimClient allows controlling the SimState
type mockStatefulSimClient struct {
	mockSimClient
	state sim.State
}

func (m *mockStatefulSimClient) GetState() sim.State {
	return m.state
}

// mockSink implements TelemetrySink
type mockSink struct {
	mu             sync.Mutex
	updateCount    int
	stateUpdateCnt int
	lastState      sim.State
}

func (m *mockSink) Update(t *sim.Telemetry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCount++
}

func (m *mockSink) UpdateState(s sim.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateUpdateCnt++
	m.lastState = s
}

func (m *mockSink) getUpdateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCount
}

func TestScheduler_SkipsTelemetryWhenInactive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Ticker.TelemetryLoop = config.Duration(10 * time.Millisecond)

	mockSim := &mockStatefulSimClient{state: sim.StateInactive}
	sink := &mockSink{}
	prov := config.NewProvider(cfg, nil)
	sched := NewScheduler(prov, mockSim, sink, &mockSchedGeoProvider{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	// Wait for a few ticks
	time.Sleep(50 * time.Millisecond)

	// Telemetry Update should NOT have been called (state is inactive)
	if cnt := sink.getUpdateCount(); cnt > 0 {
		t.Errorf("Telemetry was updated %d times, but should be 0 when inactive", cnt)
	}

	// Now switch to active
	mockSim.state = sim.StateActive
	time.Sleep(50 * time.Millisecond)

	// Telemetry Update SHOULD have been called
	if cnt := sink.getUpdateCount(); cnt == 0 {
		t.Error("Telemetry was never updated after switching to active")
	}
}
