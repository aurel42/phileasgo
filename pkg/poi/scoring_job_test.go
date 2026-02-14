package poi

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/scorer"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/visibility"
)

// MockScoringManager for testing ScoringJob
type MockScoringManager struct {
	CapturedStateLat float64
	CapturedStateLon float64
	CallbackCalled   bool
	ShouldFail       bool
}

func (m *MockScoringManager) UpdateScoringState(lat, lon float64) {
	m.CapturedStateLat = lat
	m.CapturedStateLon = lon
}

func (m *MockScoringManager) NotifyScoringComplete(ctx context.Context, t *sim.Telemetry, lowestElev float64) {
	m.CallbackCalled = true
}

func (m *MockScoringManager) GetTrackedPOIs() []*model.POI {
	return []*model.POI{}
}

func (m *MockScoringManager) FetchHistory(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (m *MockScoringManager) GetBoostFactor(ctx context.Context) float64 {
	return 1.0
}

func TestScoringJob_ShouldFire_Interval(t *testing.T) {
	tests := []struct {
		name           string
		elapsed        time.Duration
		runningLock    int32
		wantShouldFire bool
	}{
		{
			name:           "Before Interval (1s)",
			elapsed:        1 * time.Second,
			runningLock:    0,
			wantShouldFire: false,
		},
		{
			name:           "Before Interval (4.9s)",
			elapsed:        4900 * time.Millisecond,
			runningLock:    0,
			wantShouldFire: false,
		},
		{
			name:           "Exactly Interval (5s)",
			elapsed:        5 * time.Second,
			runningLock:    0,
			wantShouldFire: true,
		},
		{
			name:           "After Interval (6s)",
			elapsed:        6 * time.Second,
			runningLock:    0,
			wantShouldFire: true,
		},
		{
			name:           "Interval Met but Locked",
			elapsed:        10 * time.Second,
			runningLock:    1,
			wantShouldFire: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := NewScoringJob("TestScoring", nil, nil, nil, config.NewProvider(&config.Config{}, nil), nil, nil)

			// Manipulate lastRun to simulate elapsed time
			job.lastRun = time.Now().Add(-tt.elapsed)

			// Manipulate lock state
			if tt.runningLock == 1 {
				job.TryLock() // Sets running to 1
			}

			got := job.ShouldFire(&sim.Telemetry{})
			if got != tt.wantShouldFire {
				t.Errorf("ShouldFire() = %v, want %v (elapsed: %v, lock: %d)", got, tt.wantShouldFire, tt.elapsed, tt.runningLock)
			}
		})
	}
}

// MockSimClient stub
type MockSimClient struct {
	State sim.State
	Telem *sim.Telemetry
}

func (m *MockSimClient) GetState() sim.State { return m.State }
func (m *MockSimClient) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	if m.Telem != nil {
		return *m.Telem, nil
	}
	return sim.Telemetry{}, nil
}
func (m *MockSimClient) Close() error                             { return nil }
func (m *MockSimClient) GetLastTransition(stage string) time.Time { return time.Time{} }
func (m *MockSimClient) SetPredictionWindow(d time.Duration)      {}
func (m *MockSimClient) GetStageState() sim.StageState            { return sim.StageState{} }
func (m *MockSimClient) RestoreStageState(s sim.StageState)       {}
func (m *MockSimClient) SetEventRecorder(r sim.EventRecorder)     {}

func (m *MockSimClient) ExecuteCommand(ctx context.Context, cmd string, args map[string]any) error {
	return nil
}

// MockElevation stub
type MockElevation struct{}

func (m *MockElevation) GetLowestElevation(lat, lon, radiusNM float64) (int16, error) {
	return 0, nil
}
func (m *MockElevation) GetElevation(lat, lon float64) (int16, error) { return 0, nil }
func (m *MockElevation) Start(ctx context.Context) error              { return nil }

func TestScoringJob_Run(t *testing.T) {
	mockMgr := &MockScoringManager{}
	// Setup Sim: Active, airborne
	mockSim := &MockSimClient{
		State: sim.StateActive,
		Telem: &sim.Telemetry{
			Latitude:    48.8566,
			Longitude:   2.3522,
			AltitudeMSL: 1000,
			IsOnGround:  false,
		},
	}

	// Setup valid Scorer
	visCalc := visibility.NewCalculator(nil, nil)
	elev := &MockElevation{}
	scCfg := &config.ScorerConfig{DeferralEnabled: false}
	catCfg := &config.CategoriesConfig{}

	sc := scorer.NewScorer(scCfg, catCfg, visCalc, elev, nil, false)

	prov := config.NewProvider(&config.Config{}, nil)
	job := NewScoringJob("TestScoring", mockMgr, mockSim, sc, prov, nil, nil)

	// Execute Run
	ctx := context.Background()
	job.Run(ctx, mockSim.Telem)

	// Verify interactions
	if !mockMgr.CallbackCalled {
		t.Error("Expected NotifyScoringComplete to be called")
	}
	if mockMgr.CapturedStateLat != 48.8566 || mockMgr.CapturedStateLon != 2.3522 {
		t.Errorf("Expected lat/lon updated, got %.4f, %.4f", mockMgr.CapturedStateLat, mockMgr.CapturedStateLon)
	}

	// Verify Lock Released
	if !job.TryLock() {
		t.Error("Job should be unlocked after Run. TryLock failed.")
	}
}
