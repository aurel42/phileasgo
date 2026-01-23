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
func NewLLMProvider(cfg config.LLMConfig, hCfg config.HistorySettings, rc *request.Client, t *tracker.Tracker) (llm.Provider, error) {
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
			sub, err = gemini.NewClient(pCfg, rc, t)
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

		if t != nil {
			t.SetFreeTier(name, pCfg.FreeTier)
		}
	}

	// Wrap in Failover Provider with unified logging and names
	return failover.New(providers, names, hCfg.Path, hCfg.Enabled, t)
}

// NewTTSProvider returns a TTS provider based on configuration.
func NewTTSProvider(cfg *config.TTSConfig, targetLang string, t *tracker.Tracker) (tts.Provider, error) {
	var prov tts.Provider
	var err error
	var free bool

	switch cfg.Engine {
	case "sapi", "windows-sapi":
		prov = sapi.NewProvider(t)
		free = true // Local is always free
	case "edge", "edge-tts":
		prov = edgetts.NewProvider(t)
		free = cfg.EdgeTTS.FreeTier
	case "fish-audio", "fishaudio":
		prov = fishaudio.NewProvider(cfg.FishAudio, t)
		free = cfg.FishAudio.FreeTier
	case "azure", "azure-speech":
		prov = azure.NewProvider(cfg.AzureSpeech, targetLang, t)
		free = cfg.AzureSpeech.FreeTier
	default:
		return nil, fmt.Errorf("unknown tts engine: %s", cfg.Engine)
	}

	if t != nil {
		t.SetFreeTier(cfg.Engine, free)
	}

	return prov, err
}
