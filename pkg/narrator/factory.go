package narrator

import (
	"fmt"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/failover"
	"phileasgo/pkg/llm/gemini"
	"phileasgo/pkg/llm/groq"
	"phileasgo/pkg/llm/openai"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
	"phileasgo/pkg/tts/azure"
	"phileasgo/pkg/tts/edgetts"
	"phileasgo/pkg/tts/fishaudio"
	"phileasgo/pkg/tts/sapi"
)

// NewLLMProvider returns an LLM provider based on configuration, wrapped in a failover chain.
func NewLLMProvider(cfg config.LLMConfig, logPath string, rc *request.Client, t *tracker.Tracker) (llm.Provider, error) {
	if len(cfg.Fallback) == 0 {
		return nil, fmt.Errorf("no llm providers configured in fallback list")
	}

	var providers []llm.Provider
	var names []string

	for _, name := range cfg.Fallback {
		pCfg, ok := cfg.Providers[name]
		if !ok {
			return nil, fmt.Errorf("provider %q not found in config", name)
		}

		var sub llm.Provider
		var err error

		switch pCfg.Type {
		case "gemini":
			sub, err = gemini.NewClient(pCfg, rc)
		case "groq":
			sub, err = groq.NewClient(pCfg, rc)
		case "openai":
			// For generic openai, we use fixed URL for now.
			// Generic OpenAI support is primarily for self-hosted or other standard proxies.
			url := "https://api.openai.com/v1/chat/completions"
			sub, err = openai.NewClient(pCfg, url, rc)
		default:
			return nil, fmt.Errorf("unknown llm provider type: %s", pCfg.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to initialize provider %q: %w", name, err)
		}

		providers = append(providers, sub)
		names = append(names, name)
	}

	// Wrap in Failover Provider with unified logging and stats
	return failover.New(providers, names, logPath, t)
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
