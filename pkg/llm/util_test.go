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
func TestUnmarshalFlexible(t *testing.T) {
	type Target struct {
		Title string `json:"title"`
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "Direct object",
			input:   `{"title": "Test"}`,
			want:    "Test",
			wantErr: false,
		},
		{
			name:    "Array with one object",
			input:   `[{"title": "Test"}]`,
			want:    "Test",
			wantErr: false,
		},
		{
			name:    "Array with multiple objects (should fail)",
			input:   `[{"title": "Test 1"}, {"title": "Test 2"}]`,
			wantErr: true,
		},
		{
			name:    "Invalid JSON",
			input:   `{"title": "Test`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target Target
			err := UnmarshalFlexible([]byte(tt.input), &target)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalFlexible() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && target.Title != tt.want {
				t.Errorf("UnmarshalFlexible() got = %v, want %v", target.Title, tt.want)
			}
		})
	}
}
