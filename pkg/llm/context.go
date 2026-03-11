package llm

import (
	"context"
)

type multiPromptKey struct{}

// MultiPrompt is a map of provider names to their specific rendered prompts.
type MultiPrompt map[string]string

// WithMultiPrompt attaches a MultiPrompt map to the context.
func WithMultiPrompt(ctx context.Context, prompts MultiPrompt) context.Context {
	return context.WithValue(ctx, multiPromptKey{}, prompts)
}

// GetMultiPrompt retrieves the MultiPrompt map from the context.
func GetMultiPrompt(ctx context.Context) (MultiPrompt, bool) {
	mp, ok := ctx.Value(multiPromptKey{}).(MultiPrompt)
	return mp, ok
}

// ResolvePrompt attempts to find a provider-specific prompt in the context.
// If the profile is "pregrounding" and the prompt is missing, it returns an error.
func ResolvePrompt(ctx context.Context, providerName, profile, defaultPrompt string) (string, error) {
	mp, ok := GetMultiPrompt(ctx)
	if !ok {
		return defaultPrompt, nil
	}

	if specific, found := mp[providerName]; found {
		return specific, nil
	}

	// For pregrounding, missing specific template is a failure (no fallback)
	if profile == "pregrounding" {
		return "", newMissingTemplateError(providerName)
	}

	return defaultPrompt, nil
}

type MissingTemplateError struct {
	Provider string
}

func (e *MissingTemplateError) Error() string {
	return "missing required pregrounding template for provider: " + e.Provider
}

func newMissingTemplateError(provider string) error {
	return &MissingTemplateError{Provider: provider}
}
