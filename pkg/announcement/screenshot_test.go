package announcement

import (
	"context"
	"os"
	"path/filepath"
	"phileasgo/pkg/config"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/watcher"
	"testing"
	"time"
)

func TestScreenshot_Lifecycle(t *testing.T) {
	// Create temp dir for screenshots
	tmpDir, err := os.MkdirTemp("", "phileas_screenshots")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	w, _ := watcher.NewService([]string{tmpDir})
	cfg := config.DefaultConfig()
	cfg.Narrator.Screenshot.Enabled = true
	dp := &mockDP{}

	s := NewScreenshot(cfg, w, dp, dp)

	t.Run("InitiallyIdle", func(t *testing.T) {
		if s.ShouldGenerate(&sim.Telemetry{}) {
			t.Error("ShouldGenerate should be false initially")
		}
		if s.ImagePath() != "" {
			t.Error("ImagePath should be empty initially")
		}
	})

	t.Run("NewScreenshotDetected", func(t *testing.T) {
		// Mock DataProvider
		dp.AssembleGenericFunc = func(ctx context.Context, tel *sim.Telemetry) prompt.Data {
			return prompt.Data{
				"City":    "TestCity",
				"Region":  "TestRegion",
				"Country": "TC",
			}
		}

		// Create a file
		imgPath := filepath.Join(tmpDir, "shot1.png")
		if err := os.WriteFile(imgPath, []byte("fake image"), 0644); err != nil {
			t.Fatalf("Failed to create mock image: %v", err)
		}
		// Force modTime to be after watcher started
		future := time.Now().Add(1 * time.Second)
		if err := os.Chtimes(imgPath, future, future); err != nil {
			t.Fatalf("Failed to change modTime: %v", err)
		}

		if !s.ShouldGenerate(&sim.Telemetry{}) {
			t.Error("ShouldGenerate should be true after new file")
		}

		if s.ImagePath() == "" {
			t.Error("ImagePath should not be empty after detection")
		}
		if s.RawPath() != imgPath {
			t.Errorf("RawPath mismatch: got %s, want %s", s.RawPath(), imgPath)
		}

		data, err := s.GetPromptData(&sim.Telemetry{Latitude: 10, Longitude: 20, AltitudeAGL: 1000})
		if err != nil {
			t.Fatalf("GetPromptData failed: %v", err)
		}
		m := data.(prompt.Data)
		if m["Lat"] != "10.000" {
			t.Errorf("Lat mismatch: got %v", m["Lat"])
		}
	})

	t.Run("Reset", func(t *testing.T) {
		s.Reset()
		// Reset doesn't clear currentPath immediately as it might be needed for playback UI,
		// but it should reset the Base status.
		if s.Status() != StatusIdle {
			t.Errorf("Status after reset: %v", s.Status())
		}
	})

	t.Run("Metadata", func(t *testing.T) {
		if !s.ShouldPlay(&sim.Telemetry{}) {
			t.Error("ShouldPlay should always be true for screenshots")
		}
		if s.Title() != "Photograph" {
			t.Errorf("Title mismatch: got %s", s.Title())
		}
	})
}
