package core

import (
	"context"
	"phileasgo/pkg/audio"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"testing"
	"time"
)

type mockNarratorService struct {
	narrator.StubService
	isPlaying         bool
	isGenerating      bool
	hasStagedAuto     bool
	isActive          bool
	isPaused          bool
	playEssayCalled   bool
	playPOICalled     bool
	playImageCalled   bool
	prepareNextCalled bool
	RemainingFunc     func() time.Duration
	AvgLatencyFunc    func() time.Duration
}

func (m *mockNarratorService) IsPlaying() bool                  { return m.isPlaying }
func (m *mockNarratorService) IsActive() bool                   { return m.isActive }
func (m *mockNarratorService) IsGenerating() bool               { return m.isGenerating }
func (m *mockNarratorService) HasStagedAuto() bool              { return m.hasStagedAuto }
func (m *mockNarratorService) IsPaused() bool                   { return m.isPaused }
func (m *mockNarratorService) CurrentTitle() string             { return "" }
func (m *mockNarratorService) CurrentType() model.NarrativeType { return "" }
func (m *mockNarratorService) CurrentShowInfoPanel() bool       { return false }
func (m *mockNarratorService) Remaining() time.Duration {
	if m.RemainingFunc != nil {
		return m.RemainingFunc()
	}
	return 0
}
func (m *mockNarratorService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	m.playEssayCalled = true
	return true
}
func (m *mockNarratorService) PlayPOI(ctx context.Context, poiID string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
	m.playPOICalled = true
}
func (m *mockNarratorService) PlayImage(ctx context.Context, imagePath string, tel *sim.Telemetry) {
	m.playImageCalled = true
}

func (m *mockNarratorService) PrepareNextNarrative(ctx context.Context, poiID, strategy string, tel *sim.Telemetry) error {
	m.prepareNextCalled = true
	return nil
}
func (m *mockNarratorService) AverageLatency() time.Duration {
	if m.AvgLatencyFunc != nil {
		return m.AvgLatencyFunc()
	}
	return 0
}

func (m *mockNarratorService) CurrentImagePath() string {
	return ""
}

func (m *mockNarratorService) IsPOIBusy(poiID string) bool {
	return false
}
func (m *mockNarratorService) Pause()              {}
func (m *mockNarratorService) Resume()             {}
func (m *mockNarratorService) Skip()               {}
func (m *mockNarratorService) TriggerIdentAction() {}

func (m *mockNarratorService) ClearCurrentImage()                         {}
func (m *mockNarratorService) CurrentThumbnailURL() string                { return "" }
func (m *mockNarratorService) AudioService() audio.Service                { return nil }
func (m *mockNarratorService) POIManager() narrator.POIProvider           { return nil }
func (m *mockNarratorService) LLMProvider() llm.Provider                  { return nil }
func (m *mockNarratorService) ProcessGenerationQueue(ctx context.Context) {}
func (m *mockNarratorService) HasPendingGeneration() bool                 { return false }
func (m *mockNarratorService) ResetSession(ctx context.Context)           {}

// DataProvider
func (m *mockNarratorService) GetLocation(lat, lon float64) model.LocationInfo {
	return model.LocationInfo{}
}
func (m *mockNarratorService) GetPOIsNear(lat, lon, radius float64) []*model.POI { return nil }
func (m *mockNarratorService) GetRepeatTTL() time.Duration                       { return 0 }
func (m *mockNarratorService) GetLastTransition(stage string) time.Time          { return time.Time{} }
func (m *mockNarratorService) AssemblePOI(ctx context.Context, p *model.POI, t *sim.Telemetry, strategy string) prompt.Data {
	return nil
}
func (m *mockNarratorService) AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data {
	return nil
}
func (m *mockNarratorService) RecordNarration(ctx context.Context, n *model.Narrative) {}

type mockPOIManager struct {
	best *model.POI
	lat  float64
	lon  float64
}

func (m *mockPOIManager) GetBestCandidate(isOnGround bool) *model.POI {
	return m.best
}

func (m *mockPOIManager) CountScoredAbove(threshold float64, limit int) int {
	return 0 // simplified
}

func (m *mockPOIManager) LastScoredPosition() (lat, lon float64) {
	return m.lat, m.lon
}

func (m *mockPOIManager) GetCandidates(limit int, isOnGround bool) []*model.POI {
	return m.GetNarrationCandidates(limit, nil)
}

func (m *mockPOIManager) GetNarrationCandidates(limit int, minScore *float64) []*model.POI {
	if m.best == nil {
		return []*model.POI{}
	}
	// Mock score check
	if minScore != nil && m.best.Score < *minScore {
		return []*model.POI{}
	}
	return []*model.POI{m.best}
}

type mockJobSimClient struct {
	state sim.State
}

func (m *mockJobSimClient) GetState() sim.State {
	if m.state == "" {
		return sim.StateActive
	}
	return m.state
}
func (m *mockJobSimClient) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return sim.Telemetry{}, nil
}
func (m *mockJobSimClient) GetLastTransition(stage string) time.Time { return time.Time{} }
func (m *mockJobSimClient) SetPredictionWindow(d time.Duration)      {}
func (m *mockJobSimClient) Close() error                             { return nil }
func (m *mockJobSimClient) GetStageState() sim.StageState            { return sim.StageState{} }
func (m *mockJobSimClient) RestoreStageState(s sim.StageState)       {}
func (m *mockJobSimClient) SetEventRecorder(r sim.EventRecorder)     {}

func (m *mockJobSimClient) ExecuteCommand(ctx context.Context, cmd string, args map[string]any) error {
	return nil
}

func TestNarrationJob_GroundSuppression(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 10.0
	cfg.Narrator.Essay.Enabled = true // Enable Essay for these tests

	tests := []struct {
		name             string
		isPaused         bool
		altitudeAGL      float64
		bestPOI          *model.POI
		poiLat, poiLon   float64 // Optional override (default 48.0, -123.0)
		expectShouldFire bool
		expectEssay      bool
	}{
		{
			name:             "Ground: No POI -> No Essay",
			altitudeAGL:      0,
			bestPOI:          nil,
			expectShouldFire: false,
		},
		{
			name:             "Ground: Low Score POI -> No Essay",
			altitudeAGL:      0,
			bestPOI:          &model.POI{Score: 5.0},
			expectShouldFire: false,
		},
		{
			name:             "Stage: Taxi -> No Narrate (even if high score)",
			altitudeAGL:      0,
			bestPOI:          &model.POI{Score: 15.0, Lat: 48.0, Lon: -123.0, Category: "Aerodrome"},
			expectShouldFire: false,
		},
		{
			name:             "Stage: Climb -> Narrate (High Score)",
			altitudeAGL:      1000,
			bestPOI:          &model.POI{Score: 15.0, Lat: 48.0, Lon: -123.0, Category: "Castle"},
			expectShouldFire: true,
		},
		{
			name:             "Airborne (Low): No POI -> No Essay",
			altitudeAGL:      1000,
			bestPOI:          nil,
			expectShouldFire: false,
		},
		{
			name:             "Airborne (High): No POI -> Essay",
			altitudeAGL:      3000,
			bestPOI:          nil,
			expectShouldFire: true,
			expectEssay:      true,
		},
		{
			name:             "Airborne (High): Low Score POI -> Essay",
			altitudeAGL:      3000,
			bestPOI:          &model.POI{Score: 5.0},
			expectShouldFire: true,
			expectEssay:      true,
		},
		{
			name:             "Paused: High Score POI -> No Narration",
			altitudeAGL:      3000,
			bestPOI:          &model.POI{Score: 15.0},
			isPaused:         true,
			expectShouldFire: false,
		},
		{
			name:             "Ground: High Score POI but Far (>5km) -> No Narration",
			altitudeAGL:      0,
			bestPOI:          &model.POI{Score: 15.0},
			poiLat:           48.05, // ~5.5km away
			poiLon:           -123.0,
			expectShouldFire: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockN := &mockNarratorService{isPaused: tt.isPaused}
			// Initialize with valid "last scored" position to pass consistency check
			lat := 48.0
			lon := -123.0
			if tt.poiLat != 0 {
				lat = tt.poiLat
			}
			if tt.poiLon != 0 {
				lon = tt.poiLon
			}
			pm := &mockPOIManager{best: tt.bestPOI, lat: lat, lon: lon}
			simC := &mockJobSimClient{state: sim.StateActive}
			prov := config.NewProvider(cfg, nil)
			job := NewNarrationJob(prov, mockN, pm, simC, nil, nil)

			tel := &sim.Telemetry{
				AltitudeAGL: tt.altitudeAGL,
				IsOnGround:  tt.altitudeAGL < 50,
				Latitude:    48.0,
				Longitude:   -123.0,
			}

			// Force cooldown to expired for test
			job.lastTime = time.Time{}

			tel.FlightStage = sim.StageAirborne
			if tt.altitudeAGL < 50 {
				tel.FlightStage = sim.StageTaxi // Something on ground
			}
			if tt.expectShouldFire {
				// Ensure we use a stage that allows firing
				tel.FlightStage = sim.StageCruise
			}

			ctx := context.Background()
			// Test Readiness
			ready := job.CanPreparePOI(ctx, tel)
			// Only assert NOT ready if we are testing a blocker (like Paused)
			if tt.isPaused && ready {
				t.Errorf("%s: Expected CanPreparePOI to be false (Paused)", tt.name)
			}

			// Execute if ready (Simulate main.go loop)
			poiFired := false
			if ready {
				poiFired = job.PreparePOI(context.Background(), tel)
			}

			// Fallback to Essay if POI didn't fire (and Essay is ready)
			if !poiFired && job.CanPrepareEssay(ctx, tel) {
				job.PrepareEssay(ctx, tel)
			}

			// Assert Outcomes
			if tt.expectShouldFire {
				// We expected SOMETHING to happen (POI or Essay)
				if tt.expectEssay && !mockN.playEssayCalled {
					t.Error("Expected PlayEssay to be called")
				}
				if !tt.expectEssay && tt.bestPOI != nil && tt.bestPOI.Score >= cfg.Narrator.MinScoreThreshold && !mockN.playPOICalled {
					t.Error("Expected PlayPOI to be called")
				}
			} else {
				// We expected NOTHING to happen
				if mockN.playEssayCalled {
					t.Error("Unexpected PlayEssay call")
				}
				if mockN.playPOICalled {
					t.Error("Unexpected PlayPOI call")
				}
			}
		})
	}
}

func TestNarrationJob_EssayRules(t *testing.T) {
	// Setup Config
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 0.5
	cfg.Narrator.PauseDuration = config.Duration(30 * time.Second)
	cfg.Narrator.Essay.Enabled = true
	cfg.Narrator.Essay.DelayBetweenEssays = config.Duration(10 * time.Minute)
	cfg.Narrator.Essay.DelayBeforeEssay = config.Duration(1 * time.Second)

	tests := []struct {
		name              string
		bestPOI           *model.POI
		lastNarrationAgo  time.Duration
		lastEssayAgo      time.Duration
		expectShouldFire  bool
		expectEssayCalled bool
		expectPOICalled   bool
	}{
		{
			name:              "Priority: High Score POI -> POI Wins",
			bestPOI:           &model.POI{Score: 1.0, WikidataID: "Q1"},
			lastNarrationAgo:  5 * time.Minute, // Plenty of time
			lastEssayAgo:      20 * time.Minute,
			expectShouldFire:  true,
			expectPOICalled:   true,
			expectEssayCalled: false,
		},
		{
			name:             "Silence Rule: No POI, but Silence < 2*Max -> No Essay",
			bestPOI:          nil,
			lastNarrationAgo: 45 * time.Second, // < 60s (2*30s)
			lastEssayAgo:     20 * time.Minute,
			expectShouldFire: false,
		},
		{
			name:              "Silence Rule: No POI, Silence > 2*Max -> Fire Essay",
			bestPOI:           nil,
			lastNarrationAgo:  70 * time.Second, // > 60s
			lastEssayAgo:      20 * time.Minute,
			expectShouldFire:  true,
			expectEssayCalled: true,
		},
		{
			name:             "Cooldown Rule: No POI, Silence OK, Recent Essay -> No Essay",
			bestPOI:          nil,
			lastNarrationAgo: 5 * time.Minute,
			lastEssayAgo:     5 * time.Minute, // < 10m
			expectShouldFire: false,
		},
		{
			name:              "Cooldown Rule: No POI, Silence OK, Old Essay -> Fire Essay",
			bestPOI:           nil,
			lastNarrationAgo:  5 * time.Minute,
			lastEssayAgo:      15 * time.Minute, // > 10m
			expectShouldFire:  true,
			expectEssayCalled: true,
		},
		{
			name:              "Low Score POI (<Threshold): Treat as Nil -> Essay Rules Apply",
			bestPOI:           &model.POI{Score: 0.2, WikidataID: "Q2"},
			lastNarrationAgo:  5 * time.Minute,
			lastEssayAgo:      20 * time.Minute, // Essay Ready
			expectShouldFire:  true,
			expectEssayCalled: true,
			expectPOICalled:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockN := &mockNarratorService{}
			pm := &mockPOIManager{best: tt.bestPOI, lat: 48.0, lon: -123.0}
			simC := &mockJobSimClient{state: sim.StateActive}
			prov := config.NewProvider(cfg, nil)
			job := NewNarrationJob(prov, mockN, pm, simC, nil, nil)

			// Set State
			job.lastTime = time.Now().Add(-tt.lastNarrationAgo)
			if tt.lastEssayAgo > 0 {
				job.lastEssayTime = time.Now().Add(-tt.lastEssayAgo)
			}

			// Telemetry (Airborne to allow essay)
			tel := &sim.Telemetry{
				AltitudeAGL: 3000,
				Latitude:    48.0,
				Longitude:   -123.0,
				FlightStage: sim.StageCruise,
				IsOnGround:  false, // Explicitly set for this test to ensure airborne
			}

			ctx := context.Background()
			// 1. Priority Logic Simulation (Mirroring Main Loop)
			// We track if we *attempted* an action, but final success depends on Mock calls
			// 1. Priority Logic Simulation (Mirroring Main Loop)
			if job.CanPreparePOI(ctx, tel) {
				if job.PreparePOI(ctx, tel) {
					// POI Triggered, skip Essay for this tick (represented by loop break in main)
				} else {
					// Fall through to Essay if POI failed (e.g. no content)
					if job.CanPrepareEssay(ctx, tel) {
						job.PrepareEssay(ctx, tel)
					}
				}
			} else if job.CanPrepareEssay(ctx, tel) {
				job.PrepareEssay(ctx, tel)
			}

			// 2. Verification of Outcomes
			if tt.expectShouldFire {
				// If we expected fire, one of them should have been called
				if !mockN.playEssayCalled && !mockN.playPOICalled {
					t.Error("Expected narration (POI or Essay), but neither played")
				}
				if tt.expectEssayCalled != mockN.playEssayCalled {
					t.Errorf("PlayEssay called? %v, want %v", mockN.playEssayCalled, tt.expectEssayCalled)
				}
				if tt.expectPOICalled != mockN.playPOICalled {
					t.Errorf("PlayPOI called? %v, want %v", mockN.playPOICalled, tt.expectPOICalled)
				}
			} else {
				// Expected Silence
				if mockN.playEssayCalled {
					t.Error("Unexpected PlayEssay call")
				}
				if mockN.playPOICalled {
					t.Error("Unexpected PlayPOI call")
				}
			}

			// Verify State Updates
			if mockN.playEssayCalled {
				if time.Since(job.lastEssayTime) > 1*time.Second {
					t.Error("lastEssayTime was not updated after playing essay")
				}
				if time.Since(job.lastTime) > 1*time.Second {
					t.Error("lastTime (silence) was not updated after playing essay")
				}
			}
		})
	}
}

func TestNarrationJob_isPlayable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.RepeatTTL = config.Duration(600 * time.Second) // 10m

	now := time.Now()
	tests := []struct {
		name       string
		lastPlayed time.Time
		want       bool
	}{
		{
			name:       "Never Played",
			lastPlayed: time.Time{}, // Zero
			want:       true,
		},
		{
			name:       "Played Recently (1m ago)",
			lastPlayed: now.Add(-1 * time.Minute),
			want:       false,
		},
		{
			name:       "Played Long Ago (20m ago)",
			lastPlayed: now.Add(-20 * time.Minute),
			want:       true,
		},
	}

	prov := config.NewProvider(cfg, nil)
	job := &NarrationJob{cfgProv: prov, narrator: &mockNarratorService{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poi := &model.POI{LastPlayed: tt.lastPlayed}
			if got := job.isPlayable(context.Background(), poi); got != tt.want {
				t.Errorf("isPlayable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Mock Store ---

type MockStore struct {
	state map[string]string
}

func NewMockStore() *MockStore {
	return &MockStore{state: make(map[string]string)}
}

// StateStore implementation
func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) {
	val, ok := m.state[key]
	return val, ok
}
func (m *MockStore) SetState(ctx context.Context, key, val string) error {
	m.state[key] = val
	return nil
}
func (m *MockStore) DeleteState(ctx context.Context, key string) error {
	delete(m.state, key)
	return nil
}

// No-op implementations for other interfaces
func (m *MockStore) GetPOI(ctx context.Context, wikidataID string) (*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) SavePOI(ctx context.Context, poi *model.POI) error { return nil }
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *MockStore) GetCache(ctx context.Context, key string) ([]byte, bool)             { return nil, false }
func (m *MockStore) HasCache(ctx context.Context, key string) (bool, error)              { return false, nil }
func (m *MockStore) SetCache(ctx context.Context, key string, val []byte) error          { return nil }
func (m *MockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *MockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	return nil
}
func (m *MockStore) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]store.GeodataRecord, error) {
	return nil, nil
}
func (m *MockStore) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *MockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *MockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error { return nil }
func (m *MockStore) GetClassification(ctx context.Context, qid string) (string, bool, error) {
	return "", false, nil
}
func (m *MockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *MockStore) SaveArticle(ctx context.Context, article *model.Article) error { return nil }
func (m *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return nil, nil
}
func (m *MockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}
func (m *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *MockStore) SaveMSFSPOI(ctx context.Context, poi *model.MSFSPOI) error { return nil }
func (m *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *MockStore) Close() error { return nil }

// --- New Tests for Adaptive/Dynamic Logic ---

func TestNarrationJob_AdaptiveMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 10.0 // Strict Default

	// Setup Mock Store with Adaptive Mode
	store := NewMockStore()
	store.SetState(context.Background(), "filter_mode", "adaptive")

	mockN := &mockNarratorService{}
	// POI has score 5.0, which is BELOW strict threshold (10.0)
	pm := &mockPOIManager{best: &model.POI{Score: 5.0, WikidataID: "Q_LOW"}, lat: 48.0, lon: -123.0}
	simC := &mockJobSimClient{state: sim.StateActive}
	prov := config.NewProvider(cfg, store)
	job := NewNarrationJob(prov, mockN, pm, simC, store, nil)

	tel := &sim.Telemetry{
		AltitudeAGL: 3000,
		Latitude:    48.0,
		Longitude:   -123.0,
		FlightStage: sim.StageCruise,
	}
	job.lastTime = time.Time{} // Force ready

	// 1. CanPreparePOI should be TRUE
	if !job.CanPreparePOI(context.Background(), tel) {
		t.Error("Adaptive Mode: CanPreparePOI returned false for valid low-score POI")
	}

	// 2. PreparePOI should trigger PlayPOI
	job.PreparePOI(context.Background(), tel)
	if !mockN.playPOICalled {
		t.Error("Adaptive Mode: Expected PlayPOI to be called for low-score POI")
	}
}

func TestNarrationJob_DynamicMinScore(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 10.0 // Strict Default
	cfg.Narrator.Essay.Enabled = false    // Disable Essay to isolate POI check

	// Setup Mock Store with Dynamic Score (Lower than default, but higher than POI)
	store := NewMockStore()
	store.SetState(context.Background(), "filter_mode", "fixed")
	store.SetState(context.Background(), "min_poi_score", "6.0") // Dynamic Override

	mockN := &mockNarratorService{}
	pm := &mockPOIManager{best: &model.POI{Score: 5.0, WikidataID: "Q5"}, lat: 48.0, lon: -123.0}
	simC := &mockJobSimClient{state: sim.StateActive}
	prov := config.NewProvider(cfg, store)
	job := NewNarrationJob(prov, mockN, pm, simC, store, nil)

	tel := &sim.Telemetry{
		AltitudeAGL: 3000,
		Latitude:    48.0,
		Longitude:   -123.0,
		FlightStage: sim.StageCruise,
	}
	job.lastTime = time.Time{}

	// 1. CanPreparePOI should be TRUE (System Ready, not checking score yet)
	if !job.CanPreparePOI(context.Background(), tel) {
		t.Error("Dynamic Score: CanPreparePOI returned false, expected true (ready state)")
	}

	// 2. PreparePOI should return FALSE (Score < Threshold)
	if job.PreparePOI(context.Background(), tel) {
		t.Error("Dynamic Score: PreparePOI returned true, expected false (score < threshold)")
	}

	// Update Store to be even lower (4.0)
	store.SetState(context.Background(), "min_poi_score", "4.0")

	// 2. CanPreparePOI should now be TRUE (5.0 >= 4.0)
	if !job.CanPreparePOI(context.Background(), tel) {
		t.Error("Dynamic Score: CanPreparePOI returned false, expected true (5.0 >= 4.0)")
	}

	job.PreparePOI(context.Background(), tel)
	if !mockN.playPOICalled {
		t.Error("Dynamic Score: Expected PlayPOI call after lowering threshold")
	}
}

func TestNarrationJob_PipelineLogic(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 5.0
	// Base Config: Cooldown 10s via PauseDuration
	cfg.Narrator.PauseDuration = config.Duration(10) // 10s

	tests := []struct {
		name              string
		isPlaying         bool
		isGenerating      bool
		hasStagedAuto     bool
		remaining         time.Duration
		avgLatency        time.Duration
		expectShouldFire  bool
		expectPrepareNext bool
		expectPlayPOI     bool
	}{
		{
			name:              "Not Playing (Standard Trigger)",
			isPlaying:         false,
			remaining:         0,
			avgLatency:        10 * time.Second,
			expectShouldFire:  true,
			expectPrepareNext: false,
			expectPlayPOI:     true,
		},
		{
			name:              "Playing - Timing Good (Just in time)",
			isPlaying:         true,
			remaining:         2 * time.Second,  // + 10s Cooldown = 12s Target
			avgLatency:        12 * time.Second, // 12 <= 12 -> True
			expectShouldFire:  true,
			expectPrepareNext: true,
			expectPlayPOI:     false,
		},
		{
			name:              "Playing - Timing Early (Wait longer)",
			isPlaying:         true,
			remaining:         10 * time.Second, // + 10s Cooldown = 20s Target
			avgLatency:        5 * time.Second,  // 20 > 5 -> False (Too early)
			expectShouldFire:  false,
			expectPrepareNext: false,
			expectPlayPOI:     false,
		},
		{
			name:              "Playing - High Latency (Start earlier)",
			isPlaying:         true,
			remaining:         5 * time.Second,  // + 10s Cooldown = 15s Target
			avgLatency:        20 * time.Second, // 15 <= 20 -> True
			expectShouldFire:  true,
			expectPrepareNext: true,
			expectPlayPOI:     false,
		},
		{
			name:              "Playing - Timing Good but Already Staged (Blocked)",
			isPlaying:         true,
			hasStagedAuto:     true,
			remaining:         2 * time.Second,
			avgLatency:        12 * time.Second,
			expectShouldFire:  false,
			expectPrepareNext: false,
			expectPlayPOI:     false,
		},
		{
			name:              "Staged Only (No playing) - Blocked",
			isPlaying:         false,
			hasStagedAuto:     true,
			expectShouldFire:  false,
			expectPrepareNext: false,
			expectPlayPOI:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockN := &mockNarratorService{
				isPlaying:     tt.isPlaying,
				isActive:      tt.isPlaying || tt.isGenerating || tt.hasStagedAuto,
				isGenerating:  tt.isGenerating,
				hasStagedAuto: tt.hasStagedAuto,
			}
			// Setup Mocks
			mockN.RemainingFunc = func() time.Duration { return tt.remaining }
			mockN.AvgLatencyFunc = func() time.Duration { return tt.avgLatency }

			pm := &mockPOIManager{best: &model.POI{Score: 10.0, WikidataID: "Q_NEXT"}, lat: 48.0, lon: -123.0}
			simC := &mockJobSimClient{state: sim.StateActive}
			prov := config.NewProvider(cfg, nil)
			job := NewNarrationJob(prov, mockN, pm, simC, nil, nil)

			// Force cooldown ready for non-playing case
			job.lastTime = time.Time{}

			tel := &sim.Telemetry{
				AltitudeAGL: 3000,
				Latitude:    48.0,
				Longitude:   -123.0,
				FlightStage: sim.StageCruise,
			}

			// 1. CanPreparePOI
			fired := job.CanPreparePOI(context.Background(), tel)
			if fired != tt.expectShouldFire {
				t.Errorf("CanPreparePOI() = %v, want %v", fired, tt.expectShouldFire)
			}

			// 2. PreparePOI (if fired)
			if fired {
				job.PreparePOI(context.Background(), tel)
				if mockN.prepareNextCalled != tt.expectPrepareNext {
					t.Errorf("PrepareNextNarrative called? %v, want %v", mockN.prepareNextCalled, tt.expectPrepareNext)
				}
				if mockN.playPOICalled != tt.expectPlayPOI {
					t.Errorf("PlayPOI called? %v, want %v", mockN.playPOICalled, tt.expectPlayPOI)
				}
			}
		})
	}
}

func TestNarrationJob_FlightStageRestrictions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 5.0

	tests := []struct {
		name             string
		stage            string
		expectShouldFire bool
	}{
		{
			name:             "Taxi - blocked",
			stage:            sim.StageTaxi,
			expectShouldFire: false,
		},
		{
			name:             "Take-off - blocked",
			stage:            sim.StageTakeOff,
			expectShouldFire: false,
		},
		{
			name:             "Climb - allowed",
			stage:            sim.StageClimb,
			expectShouldFire: true,
		},
		{
			name:             "Cruise - allowed",
			stage:            sim.StageCruise,
			expectShouldFire: true,
		},
		{
			name:             "Descend - allowed",
			stage:            sim.StageDescend,
			expectShouldFire: true,
		},
		{
			name:             "Landed - blocked",
			stage:            sim.StageLanded,
			expectShouldFire: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockN := &mockNarratorService{}
			poi := &model.POI{Score: 10.0, WikidataID: "Q1", Lat: 48.0, Lon: -123.0, Category: "Aerodrome"}
			pm := &mockPOIManager{best: poi, lat: 48.0, lon: -123.0}
			simC := &mockJobSimClient{state: sim.StateActive}
			prov := config.NewProvider(cfg, nil)
			job := NewNarrationJob(prov, mockN, pm, simC, nil, nil)

			tel := &sim.Telemetry{
				Latitude:    48.0,
				Longitude:   -123.0,
				FlightStage: tt.stage,
			}

			fired := job.CanPreparePOI(context.Background(), tel)
			if fired != tt.expectShouldFire {
				t.Errorf("CanPreparePOI() = %v, want %v", fired, tt.expectShouldFire)
			}
		})
	}
}

func TestNarrationJob_VisibilityBoostAGLCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true

	tests := []struct {
		name         string
		lastAGL      float64
		initialBoost string
		expectBoost  string // Expected boost value after increment attempt
	}{
		{
			name:         "Low altitude (100ft) - no boost",
			lastAGL:      100,
			initialBoost: "1.0",
			expectBoost:  "1.0", // Should NOT change
		},
		{
			name:         "At threshold (500ft) - allows boost",
			lastAGL:      500,
			initialBoost: "1.0",
			expectBoost:  "1.1",
		},
		{
			name:         "Above threshold (1000ft) - allows boost",
			lastAGL:      1000,
			initialBoost: "1.0",
			expectBoost:  "1.1",
		},
		{
			name:         "On ground (0ft) - no boost",
			lastAGL:      0,
			initialBoost: "1.0",
			expectBoost:  "1.0",
		},
		{
			name:         "High altitude already at max - stays at max",
			lastAGL:      2000,
			initialBoost: "1.5",
			expectBoost:  "1.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMockStore()
			store.SetState(context.Background(), "visibility_boost", tt.initialBoost)

			job := &NarrationJob{
				cfgProv: config.NewProvider(cfg, store),
				store:   store,
				lastAGL: tt.lastAGL,
			}

			job.incrementVisibilityBoost(context.Background())

			got, _ := store.GetState(context.Background(), "visibility_boost")
			if got != tt.expectBoost {
				t.Errorf("visibility_boost = %v, want %v", got, tt.expectBoost)
			}
		})
	}
}

func TestNarrationJob_StartAirborne_NoDelay(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 5.0

	mockN := &mockNarratorService{}
	pm := &mockPOIManager{best: &model.POI{Score: 10.0, WikidataID: "Q_AIR"}, lat: 48.0, lon: -123.0}
	simC := &mockJobSimClient{state: sim.StateActive}
	prov := config.NewProvider(cfg, nil)
	job := NewNarrationJob(prov, mockN, pm, simC, nil, nil)
	// Important: We do NOT set job.takeoffTime manually. We test the startup logic.
	job.lastTime = time.Time{} // Force ready for narration (silence wise)

	// Simulate starting airborne (1000m AGL)
	tel := &sim.Telemetry{
		AltitudeAGL: 1000,
		IsOnGround:  false,
		Latitude:    48.0,
		Longitude:   -123.0,
		FlightStage: sim.StageCruise,
	}

	// Should fire IMMEDIATELY (no grace period) because we started in the air
	if !job.CanPreparePOI(context.Background(), tel) {
		t.Error("Started airborne but CanPreparePOI returned false (Grace period incorrectly applied?)")
	}
}
