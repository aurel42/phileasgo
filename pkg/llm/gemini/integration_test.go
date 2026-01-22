package gemini_test

import (
	"context"
	"os"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm/gemini"
)

func TestIntegration_GenerateText(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("Skipping integration test: GEMINI_API_KEY not set")
	}

	c, err := gemini.NewClient(config.ProviderConfig{
		Key:   key,
		Model: "gemini-2.0-flash",
		Type:  "gemini",
	}, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	out, err := c.GenerateText(context.Background(), "IntegrationTest", "Say 'pong'")
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if out == "" {
		t.Error("got empty response")
	}
	t.Logf("Response: %s", out)
}

func TestIntegration_GenerateJSON(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("Skipping integration test: GEMINI_API_KEY not set")
	}

	c, err := gemini.NewClient(config.ProviderConfig{
		Key:   key,
		Model: "gemini-2.0-flash",
		Type:  "gemini",
	}, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	type Response struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	var resp Response
	prompt := "Return a JSON object with 'message'='hello' and 'count'=42."

	err = c.GenerateJSON(context.Background(), "IntegrationTest", prompt, &resp)
	if err != nil {
		t.Fatalf("GenerateJSON: %v", err)
	}

	if resp.Message != "hello" {
		t.Errorf("Expected message 'hello', got %q", resp.Message)
	}
	if resp.Count != 42 {
		t.Errorf("Expected count 42, got %d", resp.Count)
	}
}
