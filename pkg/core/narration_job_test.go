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

func (m *mockPOIManager) GetBestCandidate() *model.POI {
	return m.best
}

func (m *mockPOIManager) CountScoredAbove(threshold float64, limit int) int {
	return 0 // simplified
}

func (m *mockPOIManager) LastScoredPosition() (lat, lon float64) {
	return m.lat, m.lon
}

func (m *mockPOIManager) GetCandidates(limit int) []*model.POI {
	if m.best == nil {
		return []*model.POI{}
	}
	return []*model.POI{m.best}
}

func TestNarrationJob_GroundSuppression(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Narrator.AutoNarrate = true
	cfg.Narrator.MinScoreThreshold = 10.0

	tests := []struct {
		name             string
		isPaused         bool
		altitudeAGL      float64
		bestPOI          *model.POI
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
			name:             "Ground: High Score POI -> Narrate",
			altitudeAGL:      0,
			bestPOI:          &model.POI{Score: 15.0},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockN := &mockNarratorService{isPaused: tt.isPaused}
			// Initialize with valid "last scored" position to pass consistency check
			pm := &mockPOIManager{best: tt.bestPOI, lat: 48.0, lon: -123.0}
			job := NewNarrationJob(cfg, mockN, pm, nil)

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
	job := NewNarrationJob(cfg, mockN, pm, nil)

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
			job := NewNarrationJob(cfg, mockN, pm, nil)

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
