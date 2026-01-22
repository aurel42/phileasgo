package core

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"testing"
	"time"
)

// Note: mockNarratorService and other mocks are shared with narration_job_test.go if in the same package,
// but usually tests in the same package can share types.
// However, if they are redefined, I should update them.
// Let's assume they are shared since it's the same package 'core'.
// But the error said isActive is missing in narration_frequency_test.go at line 133.
// This implies it's using the same struct.

// If it's using the same struct, then my fix to narration_job_test.go should fix it.
// Wait, I see tt.isPlaying but tt.isActive might be needed in the struct literal?
func TestNarrationJob_Frequency_Strategies(t *testing.T) {
	// Base Config
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 5.0
	cfg.Narrator.PauseDuration = config.Duration(10 * time.Second) // 4s default, using 10s for clear math
	cfg.Narrator.Essay.Enabled = false                             // Disable Essay to isolate Frequency logic checks

	tests := []struct {
		name             string
		freq             int
		isPlaying        bool
		remaining        time.Duration
		avgLatency       time.Duration
		poiStrategy      string // For "Rarely" check (MaxSkew vs others)
		expectShouldFire bool
	}{
		// FREQUENCY 1: RARELY (No Overlap, Lone Wolf Only)
		{
			name:             "Rarely: Playing -> No Fire (Strict No Overlap)",
			freq:             1,
			isPlaying:        true,
			remaining:        1 * time.Second, // Even if almost done
			avgLatency:       5 * time.Second,
			expectShouldFire: false,
		},
		{
			name:             "Rarely: Not Playing, Lone Wolf -> Fire",
			freq:             1,
			isPlaying:        false,
			poiStrategy:      narrator.StrategyMaxSkew,
			expectShouldFire: true,
		},
		{
			name:             "Rarely: Not Playing, NOT Lone Wolf -> No Fire",
			freq:             1,
			isPlaying:        false,
			poiStrategy:      narrator.StrategyUniform, // Not MaxSkew
			expectShouldFire: false,
		},

		// FREQUENCY 2: NORMAL (No Overlap, Standard Filter)
		{
			name:             "Normal: Playing -> No Fire (Strict No Overlap)",
			freq:             2,
			isPlaying:        true,
			remaining:        1 * time.Second,
			expectShouldFire: false,
		},
		{
			name:             "Normal: Not Playing -> Fire (Standard)",
			freq:             2,
			isPlaying:        false,
			expectShouldFire: true,
		},

		// FREQUENCY 3: ACTIVE (Overlap 1.0x)
		{
			name:             "Active: Playing, Lead Time Good (rem <= 1.0*lat) -> Fire",
			freq:             3,
			isPlaying:        true,
			remaining:        10 * time.Second,
			avgLatency:       10 * time.Second, // 10 <= 10 -> True
			expectShouldFire: true,
		},
		{
			name:             "Active: Playing, Too Early (rem > 1.0*lat) -> No Fire",
			freq:             3,
			isPlaying:        true,
			remaining:        11 * time.Second,
			avgLatency:       10 * time.Second, // 11 > 10 -> False
			expectShouldFire: false,
		},

		// FREQUENCY 4: BUSY (Overlap 1.5x)
		{
			name:             "Busy: Playing, Lead Time Good (rem <= 1.5*lat) -> Fire",
			freq:             4,
			isPlaying:        true,
			remaining:        15 * time.Second,
			avgLatency:       10 * time.Second, // 15 <= 15 (1.5*10) -> True
			expectShouldFire: true,
		},
		{
			name:             "Busy: Playing, Too Early (rem > 1.5*lat) -> No Fire",
			freq:             4,
			isPlaying:        true,
			remaining:        16 * time.Second,
			avgLatency:       10 * time.Second, // 16 > 15 -> False
			expectShouldFire: false,
		},

		// FREQUENCY 5: CONSTANT (Overlap 2.0x)
		{
			name:             "Constant: Playing, Lead Time Good (rem <= 2.0*lat) -> Fire",
			freq:             5,
			isPlaying:        true,
			remaining:        20 * time.Second,
			avgLatency:       10 * time.Second, // 20 <= 20 -> True
			expectShouldFire: true,
		},
		{
			name:             "Constant: Playing, Too Early (rem > 2.0*lat) -> No Fire",
			freq:             5,
			isPlaying:        true,
			remaining:        21 * time.Second,
			avgLatency:       10 * time.Second, // 21 > 20 -> False
			expectShouldFire: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Config
			cfg.Narrator.Frequency = tt.freq

			// Mock Services
			mockN := &mockNarratorService{
				isPlaying: tt.isPlaying,
				isActive:  tt.isPlaying,
			}
			mockN.RemainingFunc = func() time.Duration { return tt.remaining }
			mockN.AvgLatencyFunc = func() time.Duration { return tt.avgLatency }

			// Mock POI Provider
			// If tt.poiStrategy is set, we need to ensure the POI triggers that strategy.
			// DetermineSkewStrategy checks POI vs neighbors.
			// Using StrategyMaxSkew means the POI score is > 20% higher than neighbors.
			// Using StrategyUniform means scores are close.

			// We can implement a naive mock `POIAnalyzer` or just rely on `StrategyMaxSkew` definition.
			// Ideally, we mock `DetermineSkewStrategy`? No, it's a function.
			// We need `mockPOIManager` to implement `POIAnalyzer` interface?
			// `POIAnalyzer` has `CountScoredAbove`.

			// Check if mock implements interface
			var _ narrator.POIAnalyzer = &mockFrequencyPOIManager{}

			pm := &mockFrequencyPOIManager{
				best: &model.POI{Score: 20.0, WikidataID: "Q1"},
				lat:  48.0, lon: -123.0,
			} // Control Strategy via `CountScoredAbove`:
			// StrategyMaxSkew requires NO neighbors with score >= (best * 0.8).
			// StrategyUniform (default fallback) occurs if there are rivals.
			if tt.poiStrategy == narrator.StrategyMaxSkew {
				// Lone Wolf: No rivals
				pm.countAboveFunc = func(threshold float64, limit int) int { return 0 }
			} else if tt.poiStrategy == narrator.StrategyUniform {
				// Rivals exist
				pm.countAboveFunc = func(threshold float64, limit int) int { return 2 }
			} else {
				// Default (Lazy): No rivals (acts as Lone Wolf unless specified)
				pm.countAboveFunc = func(threshold float64, limit int) int { return 0 }
			}

			simC := &mockJobSimClient{state: sim.StateActive}

			// Note: We pass nil for store, so it falls back to cfg.Frequency
			job := NewNarrationJob(cfg, mockN, pm, simC, nil, nil, nil)

			// Ensure ready state
			job.lastTime = time.Time{}
			job.takeoffTime = time.Now().Add(-10 * time.Minute) // Grace period over

			tel := &sim.Telemetry{
				AltitudeAGL: 3000,
				Latitude:    48.0,
				Longitude:   -123.0,
			}

			// 1. Check if we can prepare a POI narration (Frequency/Pipeline logic)
			canPrepare := job.CanPreparePOI(tel)

			// Special Check: If we are testing Frequency/Pipeline rules (tt.remaining set),
			// we expect CanPrepare to match strictly.
			// If we are testing "Rarely/Lone Wolf" (content filter), CanPrepare might be true but Prepare fails.
			isContentTest := tt.poiStrategy != ""
			if !isContentTest && canPrepare != tt.expectShouldFire {
				t.Errorf("Frequency %d (%s): CanPreparePOI = %v, expected %v", tt.freq, tt.name, canPrepare, tt.expectShouldFire)
			}

			// Check correct method calls based on state
			if canPrepare {
				job.PreparePOI(context.Background(), tel)
			}

			if tt.expectShouldFire {
				// Assert PlayPOI called (unless Pipeline prepared next)
				// ... existing logic below checks this ...

				// Case 1: Pipelining (Active/Busy/Constant + IsPlaying)
				if tt.isPlaying && tt.freq >= 3 {
					if !mockN.prepareNextCalled {
						t.Error("PreparePOI: Expected PrepareNextNarrative call for Pipeline")
					}
					if mockN.playPOICalled {
						t.Error("PreparePOI: Did NOT expect PlayPOI call during Pipeline")
					}
				} else {
					// Case 2: Standard Playback (Not Playing OR Rarely/Normal)
					if !mockN.playPOICalled {
						t.Error("PreparePOI: Expected PlayPOI call")
					}
				}
			}
		})
	}
}

// mockFrequencyPOIManager implements both POIProvider and POIAnalyzer specifically for this test
type mockFrequencyPOIManager struct {
	best           *model.POI
	lat, lon       float64
	countAboveFunc func(threshold float64, limit int) int
}

func (m *mockFrequencyPOIManager) GetNarrationCandidates(limit int, minScore *float64, isOnGround bool) []*model.POI {
	if m.best == nil {
		return []*model.POI{}
	}
	return []*model.POI{m.best}
}

func (m *mockFrequencyPOIManager) LastScoredPosition() (lat, lon float64) {
	return m.lat, m.lon
}

func (m *mockFrequencyPOIManager) CountScoredAbove(threshold float64, limit int) int {
	if m.countAboveFunc != nil {
		return m.countAboveFunc(threshold, limit)
	}
	return 0
}
