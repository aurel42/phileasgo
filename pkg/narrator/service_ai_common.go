package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/tts"
)

// checkSessionPersistence attempts to restore a previous session if airborne and nearby.
func (s *AIService) checkSessionPersistence(ctx context.Context, tel *sim.Telemetry) {
	// Only run once
	var ran bool
	s.persistOnce.Do(func() {
		ran = true
	})
	if !ran {
		return
	}

	// Must be airborne to restore (avoid restoring old flight on ground)
	if tel.IsOnGround {
		return
	}

	// Initial delay to ensure stability? Not strictly needed if airborne.

	val, found := s.st.GetState(ctx, "session_context")
	if !found || val == "" {
		return
	}

	// Check distance
	lat, lon, err := session.UnmarshalLocation([]byte(val))
	if err != nil {
		slog.Error("Narrator: Failed to unmarshal persistent session location", "error", err)
		return
	}

	dist := geo.Distance(geo.Point{Lat: lat, Lon: lon}, geo.Point{Lat: tel.Latitude, Lon: tel.Longitude})
	// 50 nautical miles ~= 92.6 km => 92600 meters
	if dist > 92600 {
		slog.Info("Narrator: Persistent session too far away, ignoring", "dist_m", dist)
		return
	}

	if err := s.session().Restore([]byte(val)); err != nil {
		slog.Error("Narrator: Failed to restore persisted session state", "error", err)
	} else {
		slog.Info("Narrator: Successfully restored persisted session state", "dist_m", dist)
	}
}

func (s *AIService) generateScript(ctx context.Context, profile, prompt string) (string, error) {
	script, err := s.llm.GenerateText(ctx, profile, prompt)
	if err != nil {
		return "", err
	}
	// Filter markdown artifacts that don't sound good in TTS
	script = strings.ReplaceAll(script, "*", "")
	return script, nil
}

func (s *AIService) synthesizeAudio(ctx context.Context, script, safeID string) (audioPath, format string, err error) {
	// Use system temp directory instead of persistent cache
	cacheDir := os.TempDir()

	// Use unique name to avoid conflicts and persistence
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("phileas_narration_%s_%d", safeID, time.Now().UnixNano()))

	ttsProvider := s.getTTSProvider()
	voiceID := s.getVoiceID()
	format, err = ttsProvider.Synthesize(ctx, script, voiceID, outputPath)
	if err != nil {
		return "", "", err
	}
	return outputPath, format, nil
}

func (s *AIService) handleTTSError(err error) {
	slog.Error("Narrator: TTS synthesis failed", "error", err)

	// Check if this is a fatal error that should trigger fallback
	if tts.IsFatalError(err) && !s.isUsingFallbackTTS() {
		s.activateFallback()
		slog.Warn("Narrator: Skipping current POI (script incompatible with fallback TTS). Will retry with next POI.")
	}

	if s.beaconSvc != nil {
		s.beaconSvc.Clear()
	}
	// Do NOT set LastPlayed - POI can be retried
}
