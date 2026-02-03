package narrator

import (
	"phileasgo/pkg/config"
	"testing"
)

func TestAIService_TTSFallback(t *testing.T) {
	svc := &AIService{
		cfg: config.NewProvider(&config.Config{
			TTS: config.TTSConfig{
				Engine: "azure-speech",
				AzureSpeech: config.AzureSpeechConfig{
					VoiceID: "en-US-JennyNeural",
				},
				EdgeTTS: config.EdgeTTSConfig{
					VoiceID: "en-US-JennyNeural-Fallback",
				},
			},
		}, nil),
		tts: &MockTTS{},
	}

	// 1. Initial state
	if svc.isUsingFallbackTTS() {
		t.Error("should not be using fallback initially")
	}
	if p := svc.getTTSProvider(); p == nil {
		t.Error("expected non-nil provider")
	}

	// 2. Activate fallback
	svc.activateFallback()
	if !svc.isUsingFallbackTTS() {
		t.Error("expected fallback active")
	}
	if p := svc.getTTSProvider(); p == nil {
		t.Error("expected fallback provider")
	}

	// 3. Voice ID reflects the engine (Edge in fallback)
	if v := svc.getVoiceID(); v != "en-US-JennyNeural-Fallback" {
		t.Errorf("expected Edge fallback voice ID, got %s", v)
	}
}
