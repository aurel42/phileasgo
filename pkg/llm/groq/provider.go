package groq

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/openai"
	"phileasgo/pkg/request"
)

const (
	groqBaseURL = "https://api.groq.com/openai/v1/chat/completions"
)

// NewClient creates a new Groq client using the generic OpenAI provider.
func NewClient(cfg config.ProviderConfig, rc *request.Client) (*openai.Client, error) {
	if cfg.Model == "" {
		cfg.Model = "llama-3.3-70b-versatile"
	}
	return openai.NewClient(cfg, groqBaseURL, rc)
}
