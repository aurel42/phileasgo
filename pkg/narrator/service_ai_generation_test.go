package narrator

import (
	"strings"
	"testing"
)

func TestAIService_ExtractTitleFromScript(t *testing.T) {
	s := &AIService{}

	tests := []struct {
		name       string
		script     string
		wantTitle  string
		wantScript string
	}{
		{
			name:       "Standard Title",
			script:     "TITLE: The Great Land\nThis is the content.",
			wantTitle:  "The Great Land",
			wantScript: "This is the content.",
		},
		{
			name:       "Markdown Bold Title",
			script:     "**TITLE:** Mount McKinley\n**The mountains are high.**",
			wantTitle:  "Mount McKinley",
			wantScript: "**The mountains are high.**",
		},
		{
			name:       "Case Insensitive Title",
			script:     "Title : Flying over Alaska\nLow clouds today.",
			wantTitle:  "Flying over Alaska",
			wantScript: "Low clouds today.",
		},
		{
			name:       "No Title",
			script:     "Just some narration\nwithout a title.",
			wantTitle:  "",
			wantScript: "Just some narration\nwithout a title.",
		},
		{
			name:       "Title Only",
			script:     "TITLE: Only Title",
			wantTitle:  "Only Title",
			wantScript: "",
		},
		{
			name:       "Indented Title",
			script:     "  **TITLE: ** Indented\nNext line",
			wantTitle:  "Indented",
			wantScript: "Next line",
		},
		{
			name:       "Title with asterisk suffix",
			script:     "**TITLE: Bold Title**\nStory starts here",
			wantTitle:  "Bold Title",
			wantScript: "Story starts here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotScript := s.extractTitleFromScript(tt.script)
			if gotTitle != tt.wantTitle {
				t.Errorf("extractTitleFromScript() gotTitle = %v, want %v", gotTitle, tt.wantTitle)
			}
			if strings.TrimSpace(gotScript) != strings.TrimSpace(tt.wantScript) {
				t.Errorf("extractTitleFromScript() gotScript = %v, want %v", gotScript, tt.wantScript)
			}
		})
	}
}
