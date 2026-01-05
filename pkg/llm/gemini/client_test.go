package gemini

import (
	"strings"
	"testing"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateParagraphs(tt.input, tt.maxLen)
			// Normalized line endings for comparison just in case
			got = strings.ReplaceAll(got, "\r\n", "\n")
			want := strings.ReplaceAll(tt.want, "\r\n", "\n")

			if got != want {
				t.Errorf("truncateParagraphs() =\n%q\nwant:\n%q", got, want)
			}
		})
	}
}
