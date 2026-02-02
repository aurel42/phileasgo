package narrator

import (
	"context"
	"phileasgo/pkg/model"
	"strings"
	"testing"
	"time"
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

func TestAIService_PerformRescueIfNeeded(t *testing.T) {
	s := &AIService{}

	tests := []struct {
		name     string
		maxWords int
		script   string
		wantLen  int // approx
	}{
		{
			name:     "No Rescue Needed",
			maxWords: 100,
			script:   "Short script.",
			wantLen:  2,
		},
		{
			name:     "Rescue Not Possible (No limit)",
			maxWords: 0,
			script:   "Long script that won't be rescued because maxWords is 0.",
			wantLen:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &GenerationRequest{MaxWords: tt.maxWords}
			got := s.performRescueIfNeeded(context.Background(), req, tt.script)
			gotLen := len(strings.Fields(got))
			if tt.wantLen > 0 && gotLen != tt.wantLen {
				t.Errorf("performRescueIfNeeded() got len = %d, want %d", gotLen, tt.wantLen)
			}
		})
	}
}

func TestAIService_ConstructNarrative(t *testing.T) {
	s := &AIService{}
	req := &GenerationRequest{
		Type:         model.NarrativeTypePOI,
		Title:        "Test POI",
		MaxWords:     50,
		ThumbnailURL: "http://thumb",
	}

	n := s.constructNarrative(req, "The script", "Extracted Title", "audio.mp3", "mp3", time.Now(), time.Second)

	if n.Title != "Test POI" {
		t.Errorf("Expected Title 'Test POI', got '%s'", n.Title)
	}
	if n.ThumbnailURL != "http://thumb" {
		t.Errorf("Expected ThumbnailURL 'http://thumb', got '%s'", n.ThumbnailURL)
	}

	// Test screenshot special case
	req.Type = model.NarrativeTypeScreenshot
	req.Title = ""
	req.ImagePath = "C:\\path\\to\\shot.png"
	n = s.constructNarrative(req, "The script", "Extracted Title", "audio.mp3", "mp3", time.Now(), time.Second)
	if n.Title != "Extracted Title" {
		t.Errorf("Expected Extracted Title, got '%s'", n.Title)
	}
	if n.ImagePath != "C:\\path\\to\\shot.png" {
		t.Errorf("Expected ImagePath to be preserved as raw path, got '%s'", n.ImagePath)
	}
}
