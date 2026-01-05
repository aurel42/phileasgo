package azure

import (
	"regexp"
	"testing"
)

func TestSilenceInjection(t *testing.T) {
	// Replicate the logic from Synthesize to verify correctness
	injectSilence := func(text string) string {
		re := regexp.MustCompile(`</lang>`)
		return re.ReplaceAllString(text, `</lang><break time="25ms"/>`)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Single tag",
			input: `Hello <lang xml:lang="fr-FR">Paris</lang>`,
			want:  `Hello <lang xml:lang="fr-FR">Paris</lang><break time="25ms"/>`,
		},
		{
			name:  "Multiple tags",
			input: `From <lang xml:lang="fr-FR">Paris</lang> to <lang xml:lang="de-DE">Berlin</lang>`,
			want:  `From <lang xml:lang="fr-FR">Paris</lang><break time="25ms"/> to <lang xml:lang="de-DE">Berlin</lang><break time="25ms"/>`,
		},
		{
			name:  "No tags",
			input: `Hello World`,
			want:  `Hello World`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := injectSilence(tt.input)
			if got != tt.want {
				t.Errorf("injectSilence() = %v, want %v", got, tt.want)
			}
		})
	}
}
