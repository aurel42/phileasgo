package perplexity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

func TestNewClient(t *testing.T) {
	t.Run("requires api key", func(t *testing.T) {
		cfg := config.ProviderConfig{
			Profiles: map[string]string{"narration": "sonar"},
		}
		_, err := NewClient(&cfg, nil)
		if err == nil {
			t.Error("expected error for missing api key")
		}
	})

	t.Run("creates client with key", func(t *testing.T) {
		cfg := config.ProviderConfig{
			Key:      "test-key",
			Profiles: map[string]string{"narration": "sonar"},
		}
		c, err := NewClient(&cfg, nil)
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
	c, _ := NewClient(&cfg, nil)

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
	c, _ := NewClient(&cfg, nil)

	_, err := c.GenerateImageText(context.Background(), "narration", "test", "/path/to/image.jpg")
	if err == nil {
		t.Error("expected error for image text generation")
	}
}

func TestGenerateText(t *testing.T) {
	tr := tracker.New()
	rc := request.New(nil, tr, request.ClientConfig{})

	t.Run("Happy Path", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("expected bearer token, got %s", r.Header.Get("Authorization"))
			}

			var req sonarRequest
			json.NewDecoder(r.Body).Decode(&req)
			if len(req.Messages) == 0 || req.Messages[0].Content != "hello world" {
				t.Errorf("expected query 'hello world'")
			}

			fmt.Fprint(w, `{"choices": [{"message": {"content": "This is the answer."}}]}`)
		}))
		defer ts.Close()

		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  ts.URL,
			Profiles: map[string]string{"narration": "sonar"},
		}
		c, _ := NewClient(&cfg, rc)

		res, err := c.GenerateText(context.Background(), "narration", "hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "This is the answer." {
			t.Errorf("expected 'This is the answer.', got %q", res)
		}
	})

	t.Run("ResolvePrompt Interaction", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req sonarRequest
			json.NewDecoder(r.Body).Decode(&req)
			if len(req.Messages) == 0 || req.Messages[0].Content != "resolved prompt" {
				t.Errorf("expected 'resolved prompt'")
			}

			fmt.Fprint(w, `{"choices": [{"message": {"content": "ok"}}]}`)
		}))
		defer ts.Close()

		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  ts.URL,
			Profiles: map[string]string{"narration": "sonar"},
		}
		c, _ := NewClient(&cfg, rc)
		c.SetLabel("my-perplexity")

		mp := llm.MultiPrompt{"my-perplexity": "resolved prompt"}
		ctx := llm.WithMultiPrompt(context.Background(), mp)

		_, err := c.GenerateText(ctx, "narration", "default prompt")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
