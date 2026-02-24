package nvidia

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/openai"
	"phileasgo/pkg/request"
)

const baseURL = "https://integrate.api.nvidia.com/v1"

// NewClient creates a new Nvidia client using the generic OpenAI provider.
func NewClient(cfg config.ProviderConfig, rc *request.Client) (*openai.Client, error) {
	return openai.NewClient(cfg, baseURL, rc)
}
