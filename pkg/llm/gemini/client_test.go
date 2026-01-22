package gemini

import (
	"context"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
)

func TestHealthCheck(t *testing.T) {
	// HealthCheck mostly validates config presence in this context without a live API.
	// We can test the API key validation logic.

	tests := []struct {
		name      string
		apiKey    string
		testMode  string
		wantError bool
	}{
		{
			name:      "No API Key",
			apiKey:    "",
			testMode:  "",
			wantError: true,
		},
		{
			name:      "With API Key (Real Call would fail without mock, but check passes validation step)",
			apiKey:    "dummy_key",
			testMode:  "true", // Mocked success
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.testMode != "" {
				t.Setenv("TEST_MODE", tt.testMode)
			}

			// Minimal config for test
			cfg := config.ProviderConfig{
				Key:   tt.apiKey,
				Model: "gemini-pro",
				Type:  "gemini",
			}

			// We need a dummy tracker and cache for the deps
			// passing nil as we aren't testing the client creation logic deeply here, just the struct state
			// actually NewClient requires valid deps.
			// Let's manually construct the client struct to test HealthCheck logic if possible,
			// OR use NewClient. NewClient doesn't error, it returns a client.

			c, _ := NewClient(cfg, nil)
			err := c.HealthCheck(context.Background())

			if (err != nil) != tt.wantError {
				t.Errorf("HealthCheck() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCleanJSONBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No markdown",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "Markdown json block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "Markdown block no lang",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "Surrounding text",
			input: "Here is json:\n```json\n{\"key\": \"value\"}\n```\nThanks",
			want:  `{"key": "value"}`,
		},
		{
			name:  "Malformed/Incomplete block start (should be treated as plain)",
			input: "```json{\"key\": \"val\"}",
			want:  `{"key": "val"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := llm.CleanJSONBlock(tt.input)
			if got != tt.want {
				t.Errorf("cleanJSONBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}
