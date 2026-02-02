package groq

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/openai"
	"phileasgo/pkg/request"
)

const (
	groqBaseURL = "https://api.groq.com/openai/v1"
)

// NewClient creates a new Groq client using the generic OpenAI provider.
func NewClient(cfg config.ProviderConfig, rc *request.Client) (*openai.Client, error) {
	return openai.NewClient(cfg, groqBaseURL, rc)
}
