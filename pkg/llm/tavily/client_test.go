package tavily

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
	"testing"
)

func TestNewClient(t *testing.T) {
	t.Run("requires api key", func(t *testing.T) {
		cfg := config.ProviderConfig{
			Profiles: map[string]string{"pregrounding": "true"},
		}
		_, err := NewClient(&cfg, nil)
		if err == nil {
			t.Error("expected error for missing api key")
		}
	})

	t.Run("creates client with key and default baseURL", func(t *testing.T) {
		cfg := config.ProviderConfig{
			Key:      "test-key",
			Profiles: map[string]string{"pregrounding": "true"},
		}
		c, err := NewClient(&cfg, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.baseURL != defaultBaseURL {
			t.Errorf("expected default baseURL %s, got %s", defaultBaseURL, c.baseURL)
		}
	})

	t.Run("uses custom baseURL from config", func(t *testing.T) {
		customURL := "https://custom.tavily.com"
		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  customURL,
			Profiles: map[string]string{"pregrounding": "true"},
		}
		c, err := NewClient(&cfg, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.baseURL != customURL {
			t.Errorf("expected custom baseURL %s, got %s", customURL, c.baseURL)
		}
	})
}

func TestHasProfile(t *testing.T) {
	cfg := config.ProviderConfig{
		Key: "test-key",
		Profiles: map[string]string{
			"pregrounding": "true",
			"search":       "true",
		},
	}
	c, _ := NewClient(&cfg, nil)

	if !c.HasProfile("pregrounding") {
		t.Error("expected HasProfile to return true for pregrounding")
	}
	if !c.HasProfile("search") {
		t.Error("expected HasProfile to return true for search")
	}
	if c.HasProfile("unknown") {
		t.Error("expected HasProfile to return false for unknown")
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

			var req searchRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Query != "hello world" {
				t.Errorf("expected query 'hello world', got %s", req.Query)
			}
			if req.IncludeAnswer != "advanced" {
				t.Errorf("expected include_answer advanced, got %v", req.IncludeAnswer)
			}

			resp := searchResponse{
				Answer: "This is the answer.",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  ts.URL,
			Profiles: map[string]string{"pregrounding": "true"},
		}
		c, _ := NewClient(&cfg, rc)

		res, err := c.GenerateText(context.Background(), "pregrounding", "hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "This is the answer." {
			t.Errorf("expected 'This is the answer.', got %q", res)
		}
	})

	t.Run("ResolvePrompt Interaction", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req searchRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Query != "resolved prompt" {
				t.Errorf("expected 'resolved prompt', got %s", req.Query)
			}
			fmt.Fprint(w, `{"answer": "ok"}`)
		}))
		defer ts.Close()

		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  ts.URL,
			Profiles: map[string]string{"pregrounding": "true"},
		}
		c, _ := NewClient(&cfg, rc)
		c.SetLabel("my-tavily")

		mp := llm.MultiPrompt{"my-tavily": "resolved prompt"}
		ctx := llm.WithMultiPrompt(context.Background(), mp)

		_, err := c.GenerateText(ctx, "pregrounding", "default prompt")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Empty Answer Handling", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"answer": "", "results": []}`)
		}))
		defer ts.Close()

		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  ts.URL,
			Profiles: map[string]string{"pregrounding": "true"},
		}
		c, _ := NewClient(&cfg, rc)

		res, err := c.GenerateText(context.Background(), "pregrounding", "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if res != "" {
			t.Errorf("expected empty answer, got %q", res)
		}
	})

	t.Run("API Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error": "invalid key"}`)
		}))
		defer ts.Close()

		cfg := config.ProviderConfig{
			Key:      "test-key",
			BaseURL:  ts.URL,
			Profiles: map[string]string{"pregrounding": "true"},
		}
		c, _ := NewClient(&cfg, rc)

		_, err := c.GenerateText(context.Background(), "pregrounding", "test")
		if err == nil {
			t.Error("expected error for 401 response")
		}
	})
}

func TestUnsupportedMethods(t *testing.T) {
	cfg := config.ProviderConfig{
		Key:      "test-key",
		Profiles: map[string]string{"pregrounding": "true"},
	}
	c, _ := NewClient(&cfg, nil)

	t.Run("GenerateJSON error", func(t *testing.T) {
		err := c.GenerateJSON(context.Background(), "pregrounding", "test", nil)
		if err == nil {
			t.Error("expected error for GenerateJSON")
		}
	})

	t.Run("GenerateImageText error", func(t *testing.T) {
		_, err := c.GenerateImageText(context.Background(), "pregrounding", "test", "/path/to/image.jpg")
		if err == nil {
			t.Error("expected error for GenerateImageText")
		}
	})
}
