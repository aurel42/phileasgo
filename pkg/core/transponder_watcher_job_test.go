package core

import (
	"context"
	"os"
	"phileasgo/pkg/config"
	"phileasgo/pkg/db"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"testing"
)

func TestTransponderWatcherJob_handleSquawkChange(t *testing.T) {
	dbPath := "test_transponder_job.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	database, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	st := store.NewSQLiteStore(database)
	cfg := config.DefaultConfig()
	n := narrator.NewStubService()
	job := NewTransponderWatcherJob(cfg, n, st, nil)

	tests := []struct {
		name         string
		squawk       int
		wantFreq     string
		wantLen      string
		wantBoost    string
		expectPaused bool
	}{
		{
			name:      "Standard Control Squawk (7331)",
			squawk:    7331,
			wantFreq:  "3",
			wantLen:   "3",
			wantBoost: "1.00",
		},
		{
			name:      "High Limits (7555)",
			squawk:    7555,
			wantFreq:  "5",
			wantLen:   "5",
			wantBoost: "2.00",
		},
		{
			name:      "Low Limits (7111)",
			squawk:    7111,
			wantFreq:  "1",
			wantLen:   "1",
			wantBoost: "1.00", // 1.0 + (1-1)*0.25 = 1.0
		},
		{
			name:         "Pause (7031)",
			squawk:       7031,
			wantFreq:     "", // Should not update frequency if 0 (pause)
			wantLen:      "3",
			wantBoost:    "1.00",
			expectPaused: true,
		},
		{
			name:      "Invalid prefix ignored (1200)",
			squawk:    1200,
			wantFreq:  "",
			wantLen:   "",
			wantBoost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset state store for each test case if needed,
			// but here we just check if it UPDATED to the expected value.
			// Actually, let's just clear the persistent_state table.
			_, _ = database.Exec("DELETE FROM persistent_state")

			tel := &sim.Telemetry{Squawk: tt.squawk}
			job.Run(context.Background(), tel)

			if tt.wantFreq != "" {
				val, _ := st.GetState(context.Background(), "narration_frequency")
				if val != tt.wantFreq {
					t.Errorf("narration_frequency = %v, want %v", val, tt.wantFreq)
				}
			}
			if tt.wantLen != "" {
				val, _ := st.GetState(context.Background(), "text_length")
				if val != tt.wantLen {
					t.Errorf("text_length = %v, want %v", val, tt.wantLen)
				}
			}
			if tt.wantBoost != "" {
				val, _ := st.GetState(context.Background(), "visibility_boost")
				if val != tt.wantBoost {
					t.Errorf("visibility_boost = %v, want %v", val, tt.wantBoost)
				}
			}
		})
	}
}

func TestTransponderWatcherJob_ShouldFire(t *testing.T) {
	job := NewTransponderWatcherJob(nil, nil, nil, nil)
	job.lastSquawk = 1200
	job.lastIdent = false

	tests := []struct {
		name   string
		squawk int
		ident  bool
		want   bool
	}{
		{"No Change", 1200, false, false},
		{"Squawk Change", 7000, false, true},
		{"Ident Change", 1200, true, true},
		{"Both Change", 7001, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tel := &sim.Telemetry{Squawk: tt.squawk, Ident: tt.ident}
			if got := job.ShouldFire(tel); got != tt.want {
				t.Errorf("ShouldFire() = %v, want %v", got, tt.want)
			}
			// Simulate job update of state at end of Run
			job.lastSquawk = tt.squawk
			job.lastIdent = tt.ident
		})
	}
}

// MockNarrator for tracking calls
type MockNarrator struct {
	*narrator.StubService
	paused  bool
	skipped bool
	stopped bool
}

func (m *MockNarrator) Pause() {
	m.paused = true
}
func (m *MockNarrator) Resume() {
	m.paused = false
}
func (m *MockNarrator) IsPaused() bool {
	return m.paused
}
func (m *MockNarrator) Skip() {
	m.skipped = true
}
func (m *MockNarrator) Stop() {
	m.stopped = true
}

func TestTransponderWatcherJob_handleIdentTrigger(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		initialPause bool
		wantPaused   bool // check paused state after toggle
		wantSkipped  bool
		wantStopped  bool
	}{
		{
			name:         "Pause Toggle (Resume)",
			action:       "pause_toggle",
			initialPause: true,
			wantPaused:   false,
		},
		{
			name:         "Pause Toggle (Pause)",
			action:       "pause_toggle",
			initialPause: false,
			wantPaused:   true,
		},
		{
			name:        "Skip",
			action:      "skip",
			wantSkipped: true,
		},
		{
			name:        "Stop",
			action:      "stop",
			wantStopped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Transponder.IdentAction = tt.action

			mn := &MockNarrator{StubService: narrator.NewStubService()}
			mn.paused = tt.initialPause

			job := NewTransponderWatcherJob(cfg, mn, nil, nil)

			// Simulate trigger
			tel := &sim.Telemetry{Ident: true}
			job.lastIdent = false // Ensure rising edge
			job.Run(context.Background(), tel)

			if tt.action == "pause_toggle" {
				if mn.paused != tt.wantPaused {
					t.Errorf("paused = %v, want %v", mn.paused, tt.wantPaused)
				}
			}
			if tt.wantSkipped && !mn.skipped {
				t.Error("expected Skip() to be called")
			}
			if tt.wantStopped && !mn.stopped {
				t.Error("expected Stop() to be called")
			}
		})
	}
}
