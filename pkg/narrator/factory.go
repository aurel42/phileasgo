package narrator

import (
	"fmt"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/gemini"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
	"phileasgo/pkg/tts/azure"
	"phileasgo/pkg/tts/edgetts"
	"phileasgo/pkg/tts/fishaudio"
	"phileasgo/pkg/tts/sapi"
)

// NewLLMProvider returns an LLM provider based on configuration.
func NewLLMProvider(cfg config.LLMConfig, logPath string, rc *request.Client, t *tracker.Tracker) (llm.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return gemini.NewClient(cfg, logPath, rc, t)
	default:
		return nil, fmt.Errorf("unknown llm provider: %s", cfg.Provider)
	}
}

// NewTTSProvider returns a TTS provider based on configuration.
func NewTTSProvider(cfg *config.TTSConfig, targetLang string, t *tracker.Tracker) (tts.Provider, error) {
	switch cfg.Engine {
	case "sapi", "windows-sapi":
		return sapi.NewProvider(t), nil
	case "edge", "edge-tts":
		return edgetts.NewProvider(t), nil
	case "fish-audio", "fishaudio":
		return fishaudio.NewProvider(cfg.FishAudio, t), nil
	case "azure", "azure-speech":
		return azure.NewProvider(cfg.AzureSpeech, targetLang, t), nil
	default:
		return nil, fmt.Errorf("unknown tts engine: %s", cfg.Engine)
	}
}
