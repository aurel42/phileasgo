package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"phileasgo/pkg/tts"
)

func (s *AIService) generateScript(ctx context.Context, prompt string) (string, error) {
	script, err := s.llm.GenerateText(ctx, "narration", prompt)
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
