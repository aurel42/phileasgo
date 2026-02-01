package announcement

import (
	"context"
	"fmt"
	"log/slog"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/watcher"
)

type Screenshot struct {
	*Base
	cfg         *config.Config
	watcher     *watcher.Service
	provider    DataProvider
	currentPath string
}

func NewScreenshot(cfg *config.Config, watcher *watcher.Service, dp DataProvider, events EventRecorder) *Screenshot {
	return &Screenshot{
		Base:     NewBase("screenshot", model.NarrativeTypeScreenshot, false, dp, events),
		cfg:      cfg,
		watcher:  watcher,
		provider: dp,
	}
}

func (s *Screenshot) ShouldGenerate(t *sim.Telemetry) bool {
	if s.watcher == nil || !s.cfg.Narrator.Screenshot.Enabled {
		return false
	}

	if path, ok := s.watcher.CheckNew(); ok {
		slog.Info("Screenshot: New detected", "path", path)
		s.currentPath = path

		// Reset state to ensure clean generation
		s.Reset()

		return true
	}
	return false
}

func (s *Screenshot) GetPromptData(t *sim.Telemetry) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("telemetry is nil")
	}

	ctx := context.Background()
	data := s.provider.AssembleGeneric(ctx, t)

	// Location Context
	loc := s.provider.GetLocation(t.Latitude, t.Longitude)
	data["City"] = loc.CityName
	data["Region"] = loc.Admin1Name
	data["Country"] = loc.CountryCode

	// Formatted Telemetry
	data["Lat"] = fmt.Sprintf("%.3f", t.Latitude)
	data["Lon"] = fmt.Sprintf("%.3f", t.Longitude)
	data["Alt"] = fmt.Sprintf("%.0f", t.AltitudeAGL)

	// Length: Use Short Words setting
	// Ideally we would apply the multiplier here, but we'll use the base config for simplicity
	// until we expose the multiplier logic in DataProvider.
	shortLimit := s.cfg.Narrator.NarrationLengthShortWords
	if shortLimit <= 0 {
		shortLimit = 50
	}
	data["MaxWords"] = shortLimit

	return data, nil
}

func (s *Screenshot) ShouldPlay(t *sim.Telemetry) bool {
	return true
}

func (s *Screenshot) Title() string {
	return "Photograph"
}

func (s *Screenshot) ImagePath() string {
	// The frontend API expects a generic path that the backend serves
	// The legacy code used: "/api/images/serve?path=" + job.ImagePath
	if s.currentPath == "" {
		return ""
	}
	return "/api/images/serve?path=" + s.currentPath
}

// Reset clears the current path
func (s *Screenshot) Reset() {
	s.Base.Reset()
	// We don't clear currentPath immediately here because it might be needed for Playback/UI
	// after generation. It will be overwritten on next ShouldGenerate.
}
