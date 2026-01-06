package gemini

import (
	"testing"

	"google.golang.org/genai"
)

func TestLogGoogleSearchUsage(t *testing.T) {
	tests := []struct {
		name     string
		intent   string
		meta     *genai.GroundingMetadata
		wantLog  bool // We can't easily assert log output standardly, but we can ensure no panic
		panicMsg string
	}{
		{
			name:   "Nil Metadata - Narration",
			intent: "narration",
			meta:   nil,
		},
		{
			name:   "Nil Metadata - Other Intent",
			intent: "chat",
			meta:   nil,
		},
		{
			name:   "Valid Metadata with Entry Point",
			intent: "narration",
			meta: &genai.GroundingMetadata{
				GroundingChunks: []*genai.GroundingChunk{{}},
				SearchEntryPoint: &genai.SearchEntryPoint{
					RenderedContent: "<html>ignore me</html>",
				},
				WebSearchQueries: []string{"actual query"},
			},
		},
		{
			name:   "Valid Metadata with Nil Entry Point (Regression Case)",
			intent: "narration",
			meta: &genai.GroundingMetadata{
				GroundingChunks:  []*genai.GroundingChunk{{}},
				SearchEntryPoint: nil, // This previously caused panic
			},
		},
		{
			name:   "Empty Metadata",
			intent: "narration",
			meta: &genai.GroundingMetadata{
				GroundingChunks: []*genai.GroundingChunk{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("logGoogleSearchUsage() panicked: %v", r)
				}
			}()
			logGoogleSearchUsage(tt.intent, tt.meta)
		})
	}
}
