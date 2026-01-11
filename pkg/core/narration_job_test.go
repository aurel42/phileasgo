package core

import (
	"context"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"strings"
	"testing"
	"time"
)

type mockNarratorService struct {
	narrator.StubService
	isPlaying       bool
	isActive        bool
	isPaused        bool
	playEssayCalled bool
	playPOICalled   bool
}

func (m *mockNarratorService) IsPlaying() bool      { return m.isPlaying }
func (m *mockNarratorService) IsActive() bool       { return m.isActive }
func (m *mockNarratorService) IsGenerating() bool   { return false }
func (m *mockNarratorService) IsPaused() bool       { return m.isPaused }
func (m *mockNarratorService) CurrentTitle() string { return "" }
func (m *mockNarratorService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	m.playEssayCalled = true
	return true
}
func (m *mockNarratorService) PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry, strategy string) {
	m.playPOICalled = true
}

type mockPOIManager struct {
	best *model.POI
	lat  float64
	lon  float64
}

func (m *mockPOIManager) GetBestCandidate(isOnGround bool) *model.POI {
	if isOnGround && m.best != nil && !strings.EqualFold(m.best.Category, "aerodrome") {
		return nil
	}
	return m.best
}

func (m *mockPOIManager) CountScoredAbove(threshold float64, limit int) int {
	return 0 // simplified
}

func (m *mockPOIManager) LastScoredPosition() (lat, lon float64) {
	return m.lat, m.lon
}

func (m *mockPOIManager) GetCandidates(limit int, isOnGround bool) []*model.POI {
	return m.GetNarrationCandidates(limit, nil, isOnGround)
}

func (m *mockPOIManager) GetNarrationCandidates(limit int, minScore *float64, isOnGround bool) []*model.POI {
	if m.best == nil {
		return []*model.POI{}
	}
	if isOnGround && !strings.EqualFold(m.best.Category, "aerodrome") {
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
func (m *mockJobSimClient) SetPredictionWindow(d time.Duration) {}
func (m *mockJobSimClient) Close() error                        { return nil }

func TestNarrationJob_GroundSuppression(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 10.0

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
			name:             "Ground: High Score POI (Aerodrome) -> Narrate",
			altitudeAGL:      0,
			bestPOI:          &model.POI{Score: 15.0, Lat: 48.0, Lon: -123.0, Category: "Aerodrome"}, // Explicit Category
			expectShouldFire: true,
		},
		{
			name:             "Ground: High Score POI (Castle) -> No Narrate (Filter)",
			altitudeAGL:      0,
			bestPOI:          &model.POI{Score: 15.0, Lat: 48.0, Lon: -123.0, Category: "Castle"},
			expectShouldFire: false,
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
			job := NewNarrationJob(cfg, mockN, pm, simC, nil, nil)

			tel := &sim.Telemetry{
				AltitudeAGL: tt.altitudeAGL,
				IsOnGround:  tt.altitudeAGL < 50,
				Latitude:    48.0,
				Longitude:   -123.0,
			}

			// Force cooldown to expired for test
			job.lastTime = time.Time{}

			// Test ShouldFire
			if job.ShouldFire(tel) != tt.expectShouldFire {
				t.Errorf("%s: ShouldFire returned %v, expected %v", tt.name, !tt.expectShouldFire, tt.expectShouldFire)
			}

			if tt.expectShouldFire {
				job.Run(context.Background(), tel)
				if tt.expectEssay && !mockN.playEssayCalled {
					t.Error("Expected PlayEssay to be called")
				}
				if !tt.expectEssay && tt.bestPOI != nil && tt.bestPOI.Score >= cfg.Narrator.MinScoreThreshold && !mockN.playPOICalled {
					t.Error("Expected PlayPOI to be called")
				}
			}
		})
	}
}

func TestNarrationJob_EssayCooldownMultiplier(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.CooldownMin = config.Duration(30 * time.Second)
	cfg.Narrator.CooldownMax = config.Duration(30 * time.Second) // Force fixed cooldown

	mockN := &mockNarratorService{}
	pm := &mockPOIManager{best: nil} // Force essay
	simC := &mockJobSimClient{state: sim.StateActive}
	job := NewNarrationJob(cfg, mockN, pm, simC, nil, nil)

	tel := &sim.Telemetry{AltitudeAGL: 3000}
	job.Run(context.Background(), tel)

	// Updated Logic: Essay logic now sets standard cooldown (1.0 multiplier)
	// because the specific Essay Cooldown is handled by `job.lastEssayTime`.
	expected := 30 * time.Second // 1 * 30
	if job.nextFireDuration != expected {
		t.Errorf("Expected essay cooldown of %v, got %v", expected, job.nextFireDuration)
	}
}

func TestNarrationJob_EssayRules(t *testing.T) {
	// Setup Config
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 0.5
	cfg.Narrator.CooldownMin = config.Duration(10 * time.Second)
	cfg.Narrator.CooldownMax = config.Duration(30 * time.Second)
	cfg.Narrator.Essay.Enabled = true
	cfg.Narrator.Essay.Cooldown = config.Duration(10 * time.Minute)

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
			job := NewNarrationJob(cfg, mockN, pm, simC, nil, nil)

			// Set State
			job.lastTime = time.Now().Add(-tt.lastNarrationAgo)
			if tt.lastEssayAgo > 0 {
				job.lastEssayTime = time.Now().Add(-tt.lastEssayAgo)
			}

			// Telemetry (Airborne to allow essay)
			tel := &sim.Telemetry{
				AltitudeAGL: 3000,
				IsOnGround:  false,
				Latitude:    48.0,
				Longitude:   -123.0,
			}

			// 1. ShouldFire Check
			fired := job.ShouldFire(tel)
			if fired != tt.expectShouldFire {
				t.Errorf("ShouldFire() = %v, want %v", fired, tt.expectShouldFire)
			}

			// 2. Run Check (only if expected to fire)
			if tt.expectShouldFire {
				job.Run(context.Background(), tel)

				if tt.expectEssayCalled != mockN.playEssayCalled {
					t.Errorf("PlayEssay called? %v, want %v", mockN.playEssayCalled, tt.expectEssayCalled)
				}
				if tt.expectPOICalled != mockN.playPOICalled {
					t.Errorf("PlayPOI called? %v, want %v", mockN.playPOICalled, tt.expectPOICalled)
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
			}
		})
	}
}

func TestNarrationJob_isPlayable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.RepeatTTL = config.Duration(10 * time.Minute)

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

	job := &NarrationJob{cfg: cfg}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poi := &model.POI{LastPlayed: tt.lastPlayed}
			if got := job.isPlayable(poi); got != tt.want {
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
func (m *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int) error {
	return nil
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
	job := NewNarrationJob(cfg, mockN, pm, simC, store, nil)

	tel := &sim.Telemetry{
		AltitudeAGL: 3000,
		Latitude:    48.0,
		Longitude:   -123.0,
	}
	job.lastTime = time.Time{} // Force ready

	// 1. ShouldFire should be TRUE because adaptive mode ignores the 10.0 threshold
	if !job.ShouldFire(tel) {
		t.Error("Adaptive Mode: ShouldFire returned false for valid low-score POI")
	}

	// 2. Run should trigger PlayPOI
	job.Run(context.Background(), tel)
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
	job := NewNarrationJob(cfg, mockN, pm, simC, store, nil)

	tel := &sim.Telemetry{
		AltitudeAGL: 3000,
		Latitude:    48.0,
		Longitude:   -123.0,
	}
	job.lastTime = time.Time{}

	// 1. ShouldFire should be FALSE (5.0 < 6.0)
	// (Even though default was 10.0, we override to 6.0. 5.0 is still < 6.0)
	if job.ShouldFire(tel) {
		t.Error("Dynamic Score: ShouldFire returned true, expected false (5.0 < 6.0)")
	}

	// Update Store to be even lower (4.0)
	store.SetState(context.Background(), "min_poi_score", "4.0")

	// 2. ShouldFire should now be TRUE (5.0 >= 4.0)
	if !job.ShouldFire(tel) {
		t.Error("Dynamic Score: ShouldFire returned false, expected true (5.0 >= 4.0)")
	}

	job.Run(context.Background(), tel)
	if !mockN.playPOICalled {
		t.Error("Dynamic Score: Expected PlayPOI call after lowering threshold")
	}
}
