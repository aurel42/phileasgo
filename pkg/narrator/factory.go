package narrator

import (
	"fmt"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/failover"
	"phileasgo/pkg/llm/gemini"
	"phileasgo/pkg/llm/openai"
	"phileasgo/pkg/llm/perplexity"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
	"phileasgo/pkg/tts/azure"
	"phileasgo/pkg/tts/edgetts"
	"phileasgo/pkg/tts/fishaudio"
	"phileasgo/pkg/tts/sapi"
	"time"
)

// labelable is implemented by LLM clients that support an explicit provider label for tracking.
type labelable interface {
	SetLabel(string)
}

// NewLLMProvider returns an LLM provider based on configuration, wrapped in a failover chain.
func NewLLMProvider(cfg config.LLMConfig, hCfg config.HistorySettings, rc *request.Client, t *tracker.Tracker) (llm.Provider, error) {
	if len(cfg.Fallback) == 0 {
		return nil, fmt.Errorf("no llm providers configured in fallback list")
	}

	var providers []llm.Provider
	var names []string
	var timeouts []time.Duration

	for _, name := range cfg.Fallback {
		pCfg, ok := cfg.Providers[name]
		if !ok {
			return nil, fmt.Errorf("provider %q not found in config", name)
		}

		sub, err := buildProvider(pCfg, name, rc, t)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize provider %q: %w", name, err)
		}

		// Ensure the provider knows its identity for stats/tracking
		if l, ok := sub.(labelable); ok {
			l.SetLabel(name)
		}

		providers = append(providers, sub)
		names = append(names, name)

		timeout := time.Duration(pCfg.Timeout)
		if timeout == 0 {
			timeout = 90 * time.Second
		}
		timeouts = append(timeouts, timeout)

		if t != nil {
			t.SetFreeTier(name, pCfg.FreeTier)
		}
	}

	// Wrap in Failover Provider with unified logging and names
	return failover.New(providers, names, timeouts, hCfg.Path, hCfg.Enabled, t)
}

// buildProvider constructs a single LLM provider from its configuration.
func buildProvider(pCfg config.ProviderConfig, name string, rc *request.Client, t *tracker.Tracker) (llm.Provider, error) {
	switch pCfg.Type {
	case "gemini":
		return gemini.NewClient(pCfg, rc, t)
	case "openai", "groq", "nvidia", "deepseek":
		return openai.NewClient(pCfg, "", rc)
	case "perplexity":
		return perplexity.NewClient(pCfg, rc)
	default:
		return nil, fmt.Errorf("unknown llm provider type: %s", pCfg.Type)
	}
}

// NewTTSProvider returns a TTS provider based on configuration.
// langProv provides dynamic access to the target language (for providers that need it).
func NewTTSProvider(cfg *config.TTSConfig, langProv tts.LanguageProvider, t *tracker.Tracker) (tts.Provider, error) {
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
		prov = azure.NewProvider(cfg.AzureSpeech, langProv, t)
		free = cfg.AzureSpeech.FreeTier
	default:
		return nil, fmt.Errorf("unknown tts engine: %s", cfg.Engine)
	}

	if t != nil {
		t.SetFreeTier(cfg.Engine, free)
	}

	return prov, err
}
