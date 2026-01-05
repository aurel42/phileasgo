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
func (m *mockNarratorService) IsPaused() bool       { return m.isPaused }
func (m *mockNarratorService) CurrentTitle() string { return "" }
func (m *mockNarratorService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	m.playEssayCalled = true
	return true
}
func (m *mockNarratorService) PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry) {
	m.playPOICalled = true
}

type mockPOIManager struct {
	best *model.POI
}

func (m *mockPOIManager) GetBestCandidate() *model.POI {
	return m.best
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
			pm := &mockPOIManager{best: tt.bestPOI}
			job := NewNarrationJob(cfg, mockN, pm)

			tel := &sim.Telemetry{
				AltitudeAGL: tt.altitudeAGL,
				IsOnGround:  tt.altitudeAGL < 50,
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
	job := NewNarrationJob(cfg, mockN, pm)

	tel := &sim.Telemetry{AltitudeAGL: 3000}
	job.Run(context.Background(), tel)

	expected := 60 * time.Second // 2 * 30
	if job.nextFireDuration != expected {
		t.Errorf("Expected essay cooldown of %v, got %v", expected, job.nextFireDuration)
	}
}
