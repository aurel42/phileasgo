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

// Mock Service for Phase 2 Tests (Renamed to avoid conflict with narration_job_test.go)
type mockPhase2NarratorService struct {
	isActive          bool
	isPlaying         bool
	isGeneratingVal   bool
	hasStagedAuto     bool
	playPOICalled     bool
	playDebriefCalled bool
	RemainingFunc     func() time.Duration
	AvgLatencyFunc    func() time.Duration
}

func (m *mockPhase2NarratorService) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
	m.playPOICalled = true
}

func (m *mockPhase2NarratorService) ProcessPlaybackQueue(ctx context.Context) {}

func (m *mockPhase2NarratorService) PlayDebrief(ctx context.Context, tel *sim.Telemetry) bool {
	m.playDebriefCalled = true
	return true
}

func (m *mockPhase2NarratorService) IsActive() bool {
	return m.isActive
}

func (m *mockPhase2NarratorService) IsPlaying() bool {
	return m.isPlaying
}

func (m *mockPhase2NarratorService) CurrentTitle() string {
	return ""
}

func (m *mockPhase2NarratorService) CurrentType() model.NarrativeType {
	return ""
}

func (m *mockPhase2NarratorService) IsGenerating() bool {
	return m.isGeneratingVal
}

func (m *mockPhase2NarratorService) HasStagedAuto() bool {
	return m.hasStagedAuto
}

func (m *mockPhase2NarratorService) Remaining() time.Duration {
	if m.RemainingFunc != nil {
		return m.RemainingFunc()
	}
	return 0
}

func (m *mockPhase2NarratorService) AverageLatency() time.Duration {
	if m.AvgLatencyFunc != nil {
		return m.AvgLatencyFunc()
	}
	return 0
}

func (m *mockPhase2NarratorService) IsPaused() bool { return false }
func (m *mockPhase2NarratorService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	return true
}
func (m *mockPhase2NarratorService) PlayImage(ctx context.Context, path string, tel *sim.Telemetry) {}
func (m *mockPhase2NarratorService) GetPendingManualOverride() (string, string, bool) {
	return "", "", false
}
func (m *mockPhase2NarratorService) HasPendingManualOverride() bool { return false }
func (m *mockPhase2NarratorService) GetPreparedPOI() *model.POI     { return nil }
func (m *mockPhase2NarratorService) PrepareNextNarrative(ctx context.Context, poiID string, strategy string, tel *sim.Telemetry) error {
	return nil
}
func (m *mockPhase2NarratorService) CurrentPOI() *model.POI             { return nil } // Added missing method
func (m *mockPhase2NarratorService) Pause(duration time.Duration)       {}
func (m *mockPhase2NarratorService) Resume()                            {}
func (m *mockPhase2NarratorService) CurrentNarrative() *model.Narrative { return nil }
func (m *mockPhase2NarratorService) CurrentImagePath() string           { return "" }
func (m *mockPhase2NarratorService) IsPOIBusy(poiID string) bool        { return false }
func (m *mockPhase2NarratorService) GenerateNarrative(ctx context.Context, req *narrator.GenerationRequest) (*model.Narrative, error) {
	return nil, nil
}
func (m *mockPhase2NarratorService) NarratedCount() int    { return 0 }
func (m *mockPhase2NarratorService) Stats() map[string]any { return nil }
func (m *mockPhase2NarratorService) Start()                {}
func (m *mockPhase2NarratorService) Stop()                 {}
func (m *mockPhase2NarratorService) PlayNarrative(ctx context.Context, n *model.Narrative) error {
	return nil
}
func (m *mockPhase2NarratorService) SkipCooldown()                       {}
func (m *mockPhase2NarratorService) ShouldSkipCooldown() bool            { return false }
func (m *mockPhase2NarratorService) ResetSkipCooldown()                  {}
func (m *mockPhase2NarratorService) ReplayLast(ctx context.Context) bool { return false }
func TestPhase2_CanPreparePOI(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true

	tests := []struct {
		name             string
		isGenerating     bool
		isPlaying        bool
		remaining        time.Duration
		avgLatency       time.Duration
		freq             int // 1-5
		expectCanPrepare bool
	}{
		{
			name:             "Idle State - Ready",
			isGenerating:     false,
			isPlaying:        false,
			expectCanPrepare: true,
		},
		{
			name:             "Generating - Blocked",
			isGenerating:     true,
			isPlaying:        false,
			expectCanPrepare: false,
		},
		{
			name:             "Playing (Freq: Normal/2) - Blocked (No Pipeline)",
			isPlaying:        true,
			freq:             2,
			expectCanPrepare: false,
		},
		{
			name:             "Playing (Freq: Active/3) - Pipeline Ready (Low Remaining)",
			isPlaying:        true,
			freq:             3,
			remaining:        5 * time.Second,
			avgLatency:       6 * time.Second, // 5 <= 6 -> Ready
			expectCanPrepare: true,
		},
		{
			name:             "Playing (Freq: Active/3) - Pipeline Blocked (High Remaining)",
			isPlaying:        true,
			freq:             3,
			remaining:        20 * time.Second,
			avgLatency:       6 * time.Second, // 20 > 6 -> Blocked
			expectCanPrepare: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Narrator.Frequency = tt.freq
			if cfg.Narrator.Frequency == 0 {
				cfg.Narrator.Frequency = 3 // default
			}

			localMock := &mockPhase2NarratorService{
				isActive:        tt.isGenerating || tt.isPlaying,
				isPlaying:       tt.isPlaying,
				RemainingFunc:   func() time.Duration { return tt.remaining },
				AvgLatencyFunc:  func() time.Duration { return tt.avgLatency },
				isGeneratingVal: tt.isGenerating,
			}

			job := &NarrationJob{
				cfg:      cfg,
				narrator: localMock,
				poiMgr:   &mockPOIManager{lat: 48.0, lon: -123.0},   // Needed for PreConditions
				sim:      &mockJobSimClient{state: sim.StateActive}, // Needed for PreConditions
				lastTime: time.Time{},
			}

			// The method we are testing:
			got := job.CanPreparePOI(&sim.Telemetry{IsOnGround: false, Latitude: 48.0, Longitude: -123.0})
			if got != tt.expectCanPrepare {
				t.Errorf("CanPreparePOI() = %v, want %v", got, tt.expectCanPrepare)
			}
		})
	}
}

// TestPhase2_CanPrepareEssay tests essay eligibility.
func TestPhase2_CanPrepareEssay(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Essay.Enabled = true
	cfg.Narrator.PauseDuration = config.Duration(10 * time.Second)
	cfg.Narrator.Essay.DelayBeforeEssay = config.Duration(1 * time.Second)

	job := &NarrationJob{
		cfg:      cfg,
		narrator: &mockNarratorService{},
		poiMgr:   &mockPOIManager{lat: 48.0, lon: -123.0},
		sim:      &mockJobSimClient{state: sim.StateActive},
		lastTime: time.Now().Add(-30 * time.Second), // Quiet for 30s
	}

	// 1. Valid
	tel := &sim.Telemetry{AltitudeAGL: 3000, Latitude: 48.0, Longitude: -123.0}
	if !job.CanPrepareEssay(tel) {
		t.Error("Expected CanPrepareEssay to true")
	}

	// 2. Low Altitude
	telInfo := &sim.Telemetry{AltitudeAGL: 1000, Latitude: 48.0, Longitude: -123.0}
	if job.CanPrepareEssay(telInfo) {
		t.Error("Expected CanPrepareEssay to be false (Low Altitude)")
	}
}

// TestPhase2_PreparePOI verifies PreparePOI behavior including boost logic.
func TestPhase2_PreparePOI(t *testing.T) {
	cfg := config.DefaultConfig()
	mockN := &mockPhase2NarratorService{}
	// Setup: 1 Candidate available
	pm := &mockPOIManager{
		best: &model.POI{WikidataID: "Q_RUN", Score: 10.0},
		lat:  48.0, lon: -123.0,
	}
	simC := &mockJobSimClient{state: sim.StateActive}
	store := NewMockStore()

	job := NewNarrationJob(cfg, mockN, pm, simC, store, nil, nil)

	tel := &sim.Telemetry{
		AltitudeAGL: 3000,
		Latitude:    48.0,
		Longitude:   -123.0,
	}
	job.lastAGL = 3000 // Ensure boost logic isn't skipped

	// 1. Success Case
	if !job.PreparePOI(context.Background(), tel) {
		t.Error("PreparePOI returned false, expected true")
	}
	if !mockN.playPOICalled {
		t.Error("PlayPOI not called")
	}

	// 2. Failure Case (No Candidates)
	pm.best = nil // Remove candidates
	mockN.playPOICalled = false

	if job.PreparePOI(context.Background(), tel) {
		t.Error("PreparePOI returned true (no candidates), expected false")
	}

	// Check Boost
	val, _ := store.GetState(context.Background(), "visibility_boost")
	if val != "1.1" {
		t.Errorf("Expected visibility_boost 1.1, got %s", val)
	}
}

// TestPhase2_CanPrepareDebrief tests debrief eligibility.
func TestPhase2_CanPrepareDebrief(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Debrief.Enabled = true

	job := &NarrationJob{
		cfg:      cfg,
		narrator: &mockNarratorService{},
		poiMgr:   &mockPOIManager{lat: 48.0, lon: -123.0},
		sim:      &mockJobSimClient{state: sim.StateActive},
		lastTime: time.Time{}, // Not recently active
		// Simulating "Just Landed" state
		takeoffTime:    time.Now().Add(-1 * time.Hour), // Was airborne
		hasCheckedOnce: true,
	}

	// 1. Valid (Just Landed - requires tracking previous state, but CanPrepareDebrief
	// usually just checks if we show it now. State transition logic is in job loop usually,
	// but here we might check "IsOnGround" and "WasAirborne".
	// Since NarrationJob tracks takeoffTime, we can infer flight phase.

	// Assuming CanPrepareDebrief logic:
	// - Enabled
	// - On Ground
	// - Recent Takeoff (implies we just finished a flight)
	// - Not yet debriefed for this flight (Need a flag for this?)

	// For this test, we assume the method checks basic eligibility:
	// OnGround && Enabled && !Debriefed

	job.takeoffTime = time.Now().Add(-30 * time.Minute) // We took off 30m ago
	// And we are now on ground
	tel := &sim.Telemetry{IsOnGround: true, Latitude: 48.0, Longitude: -123.0}

	// We need a flag in NarrationJob to say "Flight Active" vs "Flight Ended"?
	// Or we rely on the trigger being edge-based in the Main Loop?
	// If CanPrepareDebrief is POLL based, it needs state.
	// Let's assume we add `hasDebriefed` to NarrationJob.

	job.hasDebriefed = false
	if !job.CanPrepareDebrief(tel) {
		t.Error("Expected CanPrepareDebrief to be true (Landed, Not Debriefed)")
	}

	// 2. Already Debriefed
	job.hasDebriefed = true
	if job.CanPrepareDebrief(tel) {
		t.Error("Expected CanPrepareDebrief to be false (Already Debriefed)")
	}

	// 3. Not Flown (e.g. just started on ground)
	job.hasDebriefed = false
	job.takeoffTime = time.Time{} // Reset
	if job.CanPrepareDebrief(tel) {
		t.Error("Expected CanPrepareDebrief to be false (No Flight Detected)")
	}
}

// TestPhase2_PrepareDebrief verifies PrepareDebrief triggering.
func TestPhase2_PrepareDebrief(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.Debrief.Enabled = true
	mockN := &mockPhase2NarratorService{}

	job := &NarrationJob{
		cfg:      cfg,
		narrator: mockN,
		poiMgr:   &mockPOIManager{lat: 48.0, lon: -123.0},
		sim:      &mockJobSimClient{state: sim.StateActive},
	}

	tel := &sim.Telemetry{IsOnGround: true, Latitude: 48.0, Longitude: -123.0}

	// Success
	job.PrepareDebrief(context.Background(), tel)

	if !mockN.playDebriefCalled {
		t.Error("PlayDebrief not called")
	}
	if !job.hasDebriefed {
		t.Error("Expected hasDebriefed to be true after PrepareDebrief")
	}
}

// --- Helper Types ---
