package llm

import (
	"testing"
)

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WordWrap(tt.input, tt.width); got != tt.want {
				t.Errorf("WordWrap() = %q, want %q", got, tt.want)
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
			name:  "Markdown json block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanJSONBlock(tt.input)
			if got != tt.want {
				t.Errorf("CleanJSONBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}
