package gemini

import (
	"testing"

	"phileasgo/pkg/llm"
)

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
