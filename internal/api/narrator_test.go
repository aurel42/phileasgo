package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
)

// MockStore implements store.Store with no-ops
type MockStore struct {
	store.Store
}

func (m *MockStore) GetState(ctx context.Context, key string) (string, bool) {
	return "", false
}

// ... other methods as needed? No, NewNarratorHandler needs the interface.
// Since we only call GetState, we can just embed Store and override GetState.

// MockAudioService matches simple interface needed by NarratorHandler
type MockAudioService struct {
	playing    bool
	busy       bool
	userPaused bool
}

func (m *MockAudioService) IsPlaying() bool    { return m.playing }
func (m *MockAudioService) IsBusy() bool       { return m.busy }
func (m *MockAudioService) IsUserPaused() bool { return m.userPaused }
func (m *MockAudioService) ResetUserPause()    {}
func (m *MockAudioService) Resume()            {}

// MockNarratorService matches simple interface needed by NarratorHandler
type MockNarratorService struct {
	active       bool
	generating   bool
	currentPOI   *model.POI
	currentTitle string
	narrated     int
	stats        map[string]any
}

func (m *MockNarratorService) IsActive() bool     { return m.active }
func (m *MockNarratorService) IsGenerating() bool { return m.generating }
func (m *MockNarratorService) PlayPOI(ctx context.Context, id string, manual, enqueueIfBusy bool, tel *sim.Telemetry, strategy string) {
}
func (m *MockNarratorService) CurrentPOI() *model.POI           { return m.currentPOI }
func (m *MockNarratorService) GetPreparedPOI() *model.POI       { return nil }
func (m *MockNarratorService) CurrentTitle() string             { return m.currentTitle }
func (m *MockNarratorService) CurrentThumbnailURL() string      { return "" }
func (m *MockNarratorService) CurrentImagePath() string         { return "" }
func (m *MockNarratorService) ClearCurrentImage()               {} // Added
func (m *MockNarratorService) NarratedCount() int               { return m.narrated }
func (m *MockNarratorService) Stats() map[string]any            { return m.stats }
func (m *MockNarratorService) CurrentType() model.NarrativeType { return "" }

func TestNarratorHandler_HandleStatus_Logging(t *testing.T) {
	// Setup log capture
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	// Enable trace for this test
	originalTrace := logging.EnableTrace
	logging.EnableTrace = true
	defer func() { logging.EnableTrace = originalTrace }()

	mockAudio := &MockAudioService{}
	mockNarrator := &MockNarratorService{}
	mockStore := &MockStore{}
	h := NewNarratorHandler(mockAudio, mockNarrator, mockStore)

	// Helper to make a request
	makeReq := func() {
		req := httptest.NewRequest("GET", "/api/narrator/status", http.NoBody)
		w := httptest.NewRecorder()
		h.HandleStatus(w, req)
	}

	// 1. Initial state: idle
	mockNarrator.active = false
	makeReq()

	// 2. Change state to playing
	// Should log
	logBuf.Reset()
	mockNarrator.active = true
	mockAudio.playing = true
	makeReq()

	if !strings.Contains(logBuf.String(), "Narrator state changed") {
		t.Errorf("Expected log message for state change to playing, got: %s", logBuf.String())
	}

	// 3. Same state (playing)
	// Should NOT log
	logBuf.Reset()
	makeReq()

	if logBuf.Len() > 0 {
		t.Errorf("Expected no log message for same state, got: %s", logBuf.String())
	}

	// 4. Change internal valid (e.g. CurrentTitle changes)
	// Should log
	logBuf.Reset()
	mockNarrator.currentTitle = "New Title"
	makeReq()

	if !strings.Contains(logBuf.String(), "Narrator state changed") {
		t.Errorf("Expected log message for title change, got: %s", logBuf.String())
	}

	// 5. Change back to idle
	logBuf.Reset()
	mockNarrator.active = false
	mockAudio.playing = false
	mockNarrator.currentTitle = ""
	makeReq()

	if !strings.Contains(logBuf.String(), "Narrator state changed") {
		t.Errorf("Expected log message for state change to idle, got: %s", logBuf.String())
	}
}
