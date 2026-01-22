package llm

import (
	"context"
)

// Provider defines the interface for interacting with LLM services.
type Provider interface {
	// GenerateText sends a prompt and returns the text response.
	GenerateText(ctx context.Context, name, prompt string) (string, error)

	// GenerateJSON sends a prompt and unmarshals the response into the target struct.
	GenerateJSON(ctx context.Context, name, prompt string, target any) error

	// GenerateImageText sends a prompt + image path and returns the text response.
	GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error)

	// HealthCheck verifies that the provider is configured and reachable.
	HealthCheck(ctx context.Context) error

	// HasProfile checks if the provider has a specific profile configured.
	HasProfile(name string) bool
}
