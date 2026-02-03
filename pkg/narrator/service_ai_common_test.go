package narrator

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/tts"
	"testing"
)

func TestAIService_HandleTTSError(t *testing.T) {
	svc := &AIService{
		cfg: config.NewProvider(&config.Config{}, nil),
	}

	// Should switch to fallback on fatal error
	svc.handleTTSError(tts.NewFatalError(429, "rate limited"))

	if !svc.isUsingFallbackTTS() {
		t.Error("expected fallback after fatal error")
	}
}
