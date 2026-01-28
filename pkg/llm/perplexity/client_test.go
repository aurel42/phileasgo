package perplexity

import (
	"context"
	"testing"

	"phileasgo/pkg/config"
)

func TestNewClient(t *testing.T) {
	t.Run("requires api key", func(t *testing.T) {
		cfg := config.ProviderConfig{
			Profiles: map[string]string{"narration": "sonar"},
		}
		_, err := NewClient(cfg, nil)
		if err == nil {
			t.Error("expected error for missing api key")
		}
	})

	t.Run("creates client with key", func(t *testing.T) {
		cfg := config.ProviderConfig{
			Key:      "test-key",
			Profiles: map[string]string{"narration": "sonar"},
		}
		c, err := NewClient(cfg, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c == nil {
			t.Error("expected client to be created")
		}
	})
}

func TestHasProfile(t *testing.T) {
	cfg := config.ProviderConfig{
		Key: "test-key",
		Profiles: map[string]string{
			"narration": "sonar",
			"essay":     "sonar-pro",
		},
	}
	c, _ := NewClient(cfg, nil)

	if !c.HasProfile("narration") {
		t.Error("expected HasProfile to return true for narration")
	}
	if !c.HasProfile("essay") {
		t.Error("expected HasProfile to return true for essay")
	}
	if c.HasProfile("unknown") {
		t.Error("expected HasProfile to return false for unknown")
	}
}

func TestGenerateImageTextNotSupported(t *testing.T) {
	cfg := config.ProviderConfig{
		Key:      "test-key",
		Profiles: map[string]string{"narration": "sonar"},
	}
	c, _ := NewClient(cfg, nil)

	_, err := c.GenerateImageText(context.Background(), "narration", "test", "/path/to/image.jpg")
	if err == nil {
		t.Error("expected error for image text generation")
	}
}
