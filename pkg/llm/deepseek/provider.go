package deepseek

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/openai"
	"phileasgo/pkg/request"
)

const (
	deepseekBaseURL = "https://api.deepseek.com/chat/completions"
)

// NewClient creates a new DeepSeek client using the generic OpenAI provider.
func NewClient(cfg config.ProviderConfig, rc *request.Client) (*openai.Client, error) {
	return openai.NewClient(cfg, deepseekBaseURL, rc)
}
