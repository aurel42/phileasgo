package openai

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
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

		resp := Response{}
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
		resp := Response{}
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

	// Create a valid PNG image
	tmpFile, _ := os.CreateTemp("", "test_img_*.png")
	defer os.Remove(tmpFile.Name())
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.White)
		}
	}
	_ = png.Encode(tmpFile, img)
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

func TestOpenAI_ResolveModel(t *testing.T) {
	cfg := config.ProviderConfig{
		Profiles: map[string]string{
			"narration": "pro-model",
		},
	}
	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(cfg, "http://localhost", rc)

	// Test with a known profile
	m, _ := c.ResolveModel("narration")
	if m != "pro-model" {
		t.Errorf("expected pro-model, got %s", m)
	}

	// Test with an unknown profile, should return error
	_, err := c.ResolveModel("other")
	if err == nil {
		t.Errorf("expected error for unknown profile, got nil")
	}

	// Test with an empty profile name, should return error
	_, err = c.ResolveModel("")
	if err == nil {
		t.Errorf("expected error for empty profile, got nil")
	}
}

func TestOpenAI_HasProfile(t *testing.T) {
	cfg := config.ProviderConfig{
		Profiles: map[string]string{
			"narration": "pro-model",
		},
	}
	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	c, _ := NewClient(cfg, "http://localhost", rc)

	if !c.HasProfile("narration") {
		t.Errorf("expected HasProfile to return true for 'narration'")
	}
	if c.HasProfile("vision") {
		t.Errorf("expected HasProfile to return false for 'vision'")
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

func TestOpenAI_ReasonerConstraints(t *testing.T) {
	var capturedFmt string
	var capturedTemp float32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		if req.ResponseFormat != nil {
			capturedFmt = req.ResponseFormat.Type
		} else {
			capturedFmt = "none"
		}
		capturedTemp = req.Temperature

		w.Write([]byte(`{"choices":[{"message":{"content":"{\"result\": \"ok\"}"}}]}`))
	}))
	defer server.Close()

	rc := request.New(nil, tracker.New(), request.ClientConfig{})
	cfg := config.ProviderConfig{
		Key: "key",
		Profiles: map[string]string{
			"std":      "gpt-4o",
			"reasoner": "o1-mini",
		},
	}
	c, _ := NewClient(cfg, server.URL, rc)

	// Case 1: Standard model
	var target struct{ Result string }
	_ = c.GenerateJSON(context.Background(), "std", "prompt", &target)
	if capturedFmt != "json_object" {
		t.Errorf("expected json_object format for std model, got %s", capturedFmt)
	}

	// Case 2: Reasoner model in GenerateJSON
	_ = c.GenerateJSON(context.Background(), "reasoner", "prompt", &target)
	if capturedFmt != "none" {
		t.Errorf("expected no response format for reasoner, got %s", capturedFmt)
	}
	if capturedTemp != 1.0 {
		t.Errorf("expected temperature 1.0 for reasoner, got %f", capturedTemp)
	}

	// Create a valid PNG image for vision tests
	tmpFile, _ := os.CreateTemp("", "test_img_reasoner_*.png")
	defer os.Remove(tmpFile.Name())
	_ = png.Encode(tmpFile, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	tmpFile.Close()

	// Case 3: Reasoner model in GenerateImageJSON
	_ = c.GenerateImageJSON(context.Background(), "reasoner", "prompt", tmpFile.Name(), &target)
	if capturedFmt != "none" {
		t.Errorf("expected no response format for vision reasoner, got %s", capturedFmt)
	}
	if capturedTemp != 1.0 {
		t.Errorf("expected temperature 1.0 for vision reasoner, got %f", capturedTemp)
	}
}
