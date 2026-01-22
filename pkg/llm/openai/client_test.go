package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

func TestOpenAI_GenerateText(t *testing.T) {
	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header
		if r.Header.Get("Authorization") != "Bearer test_key" {
			t.Errorf("Expected Bearer test_key, got %s", r.Header.Get("Authorization"))
		}

		resp := openaiResponse{}
		resp.Choices = []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{Content: "pong"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tr := tracker.New()
	rc := request.New(nil, tr, request.ClientConfig{})
	cfg := config.ProviderConfig{Key: "test_key", Profiles: map[string]string{"test": "test_model"}}

	c, err := NewClient(cfg, server.URL, rc)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	res, err := c.GenerateText(context.Background(), "test", "ping")
	if err != nil {
		t.Fatalf("failed to generate text: %v", err)
	}

	if res != "pong" {
		t.Errorf("expected pong, got %s", res)
	}
}

func TestOpenAI_GenerateJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResponse{}
		resp.Choices = []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{Content: "{\"result\": \"ok\"}"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{Key: "key", Profiles: map[string]string{"test": "model"}}, server.URL, rc)

	var target struct {
		Result string `json:"result"`
	}
	err := c.GenerateJSON(context.Background(), "test", "prompt", &target)
	if err != nil {
		t.Fatalf("failed to generate json: %v", err)
	}

	if target.Result != "ok" {
		t.Errorf("expected ok, got %s", target.Result)
	}
}

func TestOpenAI_GenerateImageText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"image description"}}]}`))
	}))
	defer server.Close()

	// Create a dummy image
	tmpFile, _ := os.CreateTemp("", "test_img_*.png")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write([]byte("fake image content"))
	tmpFile.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{Key: "key", Profiles: map[string]string{"test": "model"}}, server.URL, rc)

	res, err := c.GenerateImageText(context.Background(), "test", "describe", tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to generate image text: %v", err)
	}

	if res != "image description" {
		t.Errorf("expected 'image description', got %s", res)
	}
}

func TestOpenAI_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return an OpenAI error
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "invalid model", "type": "invalid_request_error"}}`))
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{Key: "key", Profiles: map[string]string{"test": "model"}}, server.URL, rc)

	_, err := c.GenerateText(context.Background(), "test", "ping")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("expected error message containing 'status 400', got %v", err)
	}
}

func TestOpenAI_InternalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Some proxies return 200 but with an error body
		w.Write([]byte(`{"error": {"message": "internal limitation", "type": "proxy_error"}}`))
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{
		Key:      "key",
		Profiles: map[string]string{"test": "model"},
	}, server.URL, rc)

	_, err := c.GenerateText(context.Background(), "test", "ping")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "internal limitation") {
		t.Errorf("expected error message 'internal limitation', got %v", err)
	}
}

func TestOpenAI_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{
		Key:      "key",
		Profiles: map[string]string{"test": "model"},
	}, server.URL, rc)

	if err := c.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestOpenAI_ResolveModel(t *testing.T) {
	cfg := config.ProviderConfig{
		Profiles: map[string]string{
			"narration": "pro-model",
		},
	}
	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(cfg, "http://localhost", rc)

	// Test with a known profile
	m, _ := c.resolveModel("narration")
	if m != "pro-model" {
		t.Errorf("expected pro-model, got %s", m)
	}

	// Test with an unknown profile, should return error
	_, err := c.resolveModel("other")
	if err == nil {
		t.Errorf("expected error for unknown profile, got nil")
	}

	// Test with an empty profile name, should return error
	_, err = c.resolveModel("")
	if err == nil {
		t.Errorf("expected error for empty profile, got nil")
	}
}

func TestOpenAI_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{
		Key:      "key",
		Profiles: map[string]string{"test": "model"},
	}, server.URL, rc)

	_, err := c.GenerateText(context.Background(), "test", "ping")
	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got %v", err)
	}
}

func TestOpenAI_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(config.ProviderConfig{Key: "key", Profiles: map[string]string{"test": "model"}}, server.URL, rc)

	_, err := c.GenerateText(context.Background(), "test", "ping")
	if err == nil || !strings.Contains(err.Error(), "returned no choices") {
		t.Errorf("expected no choices error, got %v", err)
	}
}
