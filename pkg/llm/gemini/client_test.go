package gemini

import (
	"context"
	"strings"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
)

func TestTruncateParagraphs(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name: "no wiki block - should persist all lines including empty ones",
			input: `Line 1
Line 2

Line 3`,
			maxLen: 5,
			want: `Line 1
Line 2

Line 3`,
		},
		{
			name: "inside wiki block - should truncate and remove empty lines",
			input: `<start of Wikipedia article>
Short line

This line is definitely way too long for our limit
<end of Wikipedia article>`,
			maxLen: 10,
			want: `<start of Wikipedia article>
Short line
This line ...
<end of Wikipedia article>`,
		},
		{
			name: "mixed content - prompt instructions preserved, wiki truncated",
			input: `INSTRUCTION: Do something.

<start of Wikipedia article>
Valid line
Way too long line here
<end of Wikipedia article>

Back to instructions.`,
			maxLen: 10,
			want: `INSTRUCTION: Do something.

<start of Wikipedia article>
Valid line
Way too lo...
<end of Wikipedia article>

Back to instructions.`,
		},
		{
			name: "unicode truncation - should count characters not bytes",
			input: `<start of Wikipedia article>
aé
こんにちは
<end of Wikipedia article>`,
			maxLen: 2,
			want: `<start of Wikipedia article>
aé
こん...
<end of Wikipedia article>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := llm.TruncateParagraphs(tt.input, tt.maxLen)
			// Normalized line endings for comparison just in case
			got = strings.ReplaceAll(got, "\r\n", "\n")
			want := strings.ReplaceAll(tt.want, "\r\n", "\n")

			if got != want {
				t.Errorf("truncateParagraphs() =\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

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

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "No wrap needed",
			input: "Hello World",
			width: 20,
			want:  "Hello World",
		},
		{
			name:  "Simple wrap",
			input: "Hello World",
			width: 5,
			want:  "Hello\nWorld",
		},
		{
			name:  "Long word preserved",
			input: "Hello Superextralongword World",
			width: 10,
			want:  "Hello\nSuperextralongword\nWorld",
		},
		{
			name:  "Multiple lines input",
			input: "Line 1\nLine 2 is longer",
			width: 10,
			want:  "Line 1\nLine 2 is\nlonger",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := llm.WordWrap(tt.input, tt.width); got != tt.want {
				t.Errorf("wordWrap() = %q, want %q", got, tt.want)
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
			got := cleanJSONBlock(tt.input)
			if got != tt.want {
				t.Errorf("cleanJSONBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}
