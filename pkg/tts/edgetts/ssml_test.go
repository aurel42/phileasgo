package edgetts

import (
	"strings"
	"testing"
)

func TestBuildSSML(t *testing.T) {
	tests := []struct {
		name     string
		voice    string
		text     string
		expected []string // Substrings that must be present
	}{
		{
			name:     "Normal text",
			voice:    "en-US-AvaNeural",
			text:     "Hello world",
			expected: []string{"Hello world", "en-US-AvaNeural"},
		},
		{
			name:     "Text with ampersand",
			voice:    "en-US-AvaNeural",
			text:     "Ben & Jerry's",
			expected: []string{"Ben &amp; Jerry&apos;s"},
		},
		{
			name:     "Text with tags",
			voice:    "en-US-AvaNeural",
			text:     "<speak>Hello</speak>",
			expected: []string{"&lt;speak&gt;Hello&lt;/speak&gt;"},
		},
		{
			name:     "Text with quotes",
			voice:    "en-US-AvaNeural",
			text:     `She said "Hello"`,
			expected: []string{`She said &quot;Hello&quot;`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSSML(tt.voice, tt.text)
			for _, exp := range tt.expected {
				if !strings.Contains(got, exp) {
					t.Errorf("buildSSML() = %v, expected to contain %v", got, exp)
				}
			}
		})
	}
}
