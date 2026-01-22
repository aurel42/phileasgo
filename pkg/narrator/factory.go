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
	if len(cfg.Fallback) == 0 {
		return nil, fmt.Errorf("no llm providers configured in fallback list")
	}

	// For now (Phase 1), just return the first one.
	// Later we will wrap it in FailoverProvider.
	firstName := cfg.Fallback[0]
	pCfg, ok := cfg.Providers[firstName]
	if !ok {
		return nil, fmt.Errorf("provider %q not found in config", firstName)
	}

	switch pCfg.Type {
	case "gemini":
		return gemini.NewClient(pCfg, logPath, rc, t)
	default:
		return nil, fmt.Errorf("unknown llm provider type: %s", pCfg.Type)
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
