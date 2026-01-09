package llm

import (
	"context"

	"phileasgo/pkg/config"
)

// Provider defines the interface for interacting with LLM services.
type Provider interface {
	// GenerateText sends a prompt and returns the text response.
	GenerateText(ctx context.Context, name, prompt string) (string, error)

	// GenerateJSON sends a prompt and unmarshals the response into the target struct.
	GenerateJSON(ctx context.Context, name, prompt string, target any) error

	// Configure updates the provider with new settings (e.g. API key).
	Configure(cfg config.LLMConfig) error

	// HealthCheck verifies that the provider is configured and reachable.
	HealthCheck(ctx context.Context) error
}
