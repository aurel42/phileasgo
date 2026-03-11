package llm

import (
	"context"
)

// Provider defines the interface for interacting with LLM services.
type Provider interface {
	// GenerateText sends a prompt and returns the text response.
	GenerateText(ctx context.Context, profile, prompt string) (string, error)

	// GenerateJSON sends a prompt and unmarshals the response into the target struct.
	GenerateJSON(ctx context.Context, profile, prompt string, target any) error

	// GenerateImageText sends a prompt + image path and returns the text response.
	GenerateImageText(ctx context.Context, profile, prompt, imagePath string) (string, error)

	// GenerateImageJSON sends a prompt + image path and unmarshals the response into the target struct.
	GenerateImageJSON(ctx context.Context, profile, prompt, imagePath string, target any) error

	// ValidateModels checks if the configured models are available.
	ValidateModels(ctx context.Context) error

	// HasProfile checks if the provider has a specific profile configured.
	HasProfile(profile string) bool

	// Name returns the provider's identifier (as defined in config).
	Name() string
}
