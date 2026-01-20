package narrator

import (
	"log/slog"

	"phileasgo/pkg/tts"
	"phileasgo/pkg/tts/edgetts"
)

// activateFallback switches to edge-tts for the remainder of this session.
// Called when Azure TTS returns a fatal error (429, 5xx, etc.)
func (s *AIService) activateFallback() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.useFallbackTTS {
		return // Already activated
	}

	slog.Warn("Narrator: Activating edge-tts fallback for this session")
	s.fallbackTTS = edgetts.NewProvider(s.fallbackTracker) // With tracker for stats
	s.useFallbackTTS = true
}

// getTTSProvider returns the active TTS provider (fallback if activated).
func (s *AIService) getTTSProvider() tts.Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.useFallbackTTS && s.fallbackTTS != nil {
		return s.fallbackTTS
	}
	return s.tts
}

// getVoiceID returns the configured voice ID for the active TTS engine.
func (s *AIService) getVoiceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If using fallback (EdgeTTS), use its config
	if s.useFallbackTTS {
		return s.cfg.TTS.EdgeTTS.VoiceID
	}

	// Otherwise check the primary engine
	switch s.cfg.TTS.Engine {
	case "azure-speech":
		return s.cfg.TTS.AzureSpeech.VoiceID
	case "fish-audio":
		return s.cfg.TTS.FishAudio.VoiceID
	case "edge-tts":
		return s.cfg.TTS.EdgeTTS.VoiceID
	default:
		return ""
	}
}

// isUsingFallbackTTS returns true if fallback TTS is active.
func (s *AIService) isUsingFallbackTTS() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.useFallbackTTS
}
