package poi

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
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
			job := NewScoringJob("TestScoring", nil, nil, nil, nil, nil)

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
