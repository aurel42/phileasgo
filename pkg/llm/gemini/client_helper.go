package gemini

import (
	"log/slog"

	"google.golang.org/genai"
)

// logGoogleSearchUsage logs the usage of the Google Search tool.
// It is extracted for unit testing and nil-safety.
func logGoogleSearchUsage(name string, meta *genai.GroundingMetadata) {
	used := false
	query := ""
	snippets := 0

	if meta != nil {
		snippets = len(meta.GroundingChunks)
		if len(meta.WebSearchQueries) > 0 {
			used = true
			query = meta.WebSearchQueries[0]
		}
		if meta.SearchEntryPoint != nil {
			used = true
			if query == "" {
				query = "[embedded in RenderedContent]"
			}
		}
		if snippets > 0 {
			used = true
		}
	}

	if used {
		slog.Info("Gemini: Google Search used",
			"snippets", snippets,
			"search_query", query)
	}
}
