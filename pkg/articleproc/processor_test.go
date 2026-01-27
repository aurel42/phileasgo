package articleproc

import (
	"strings"
	"testing"
)

func TestExtractProse(t *testing.T) {
	tests := []struct {
		name          string
		html          string
		wantWordCount int
		contains      []string
		notContains   []string
	}{
		{
			name: "Basic Article",
			html: `<html><body><div class="mw-parser-output">
				<p>Hello world. This is a test.</p>
				<p>Second paragraph here.</p>
				<h2><span class="mw-headline" id="References">References</span></h2>
				<div class="reflist">
					<ol class="references"><li>Ref 1</li></ol>
				</div>
				<p>This should be ignored.</p>
			</div></body></html>`,
			wantWordCount: 9,
			contains:      []string{"Hello world", "Second paragraph"},
			notContains:   []string{"This should be ignored"},
		},
		{
			name: "With Infobox and Citations",
			html: `<div class="mw-parser-output">
				<table>Infobox content</table>
				<p>The Eiffel Tower<sup>[1]</sup> is in Paris.</p>
				<p>It was built in 1889.</p>
				<style>.some-css {}</style>
				<h2><span class="mw-headline" id="History">History</span></h2>
				<p>More history.</p>
				<h2><span class="mw-headline" id="Sources">Sources</span></h2>
				<table class="navbox"><tr><td>Links</td></tr></table>
				<p>Ignored.</p>
			</div>`,
			wantWordCount: 13,
			contains:      []string{"The Eiffel Tower is in Paris", "built in 1889", "More history"},
			notContains:   []string{"Infobox", "[1]", ".some-css"},
		},
		{
			name:          "Empty Article",
			html:          `<div class="mw-parser-output"></div>`,
			wantWordCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ExtractProse(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("ExtractProse failed: %v", err)
			}

			if info.WordCount != tt.wantWordCount {
				t.Errorf("WordCount = %d, want %d", info.WordCount, tt.wantWordCount)
			}

			for _, c := range tt.contains {
				if !strings.Contains(info.Prose, c) {
					t.Errorf("Prose missing expected content: %q", c)
				}
			}

			for _, nc := range tt.notContains {
				if strings.Contains(info.Prose, nc) {
					t.Errorf("Prose contains unexpected content: %q", nc)
				}
			}
		})
	}
}
