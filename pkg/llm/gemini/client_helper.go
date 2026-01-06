package gemini

import (
	"log/slog"

	"google.golang.org/genai"
)

// logGoogleSearchUsage logs the usage of the Google Search tool.
// It is extracted for unit testing and nil-safety.
func logGoogleSearchUsage(name string, meta *genai.GroundingMetadata) {
	if meta != nil {
		snippets := len(meta.GroundingChunks)
		query := ""
		if meta.SearchEntryPoint != nil {
			query = meta.SearchEntryPoint.RenderedContent
		}
		slog.Info("Gemini: Google Search used",
			"intent", name,
			"snippets", snippets,
			"search_query", query)
	} else {
		// Explicitly log if NO search was used for narration/essay to help debug
		if name == "narration" || name == "essay" {
			slog.Warn("Gemini: Google Search tool configured but NOT used by model", "intent", name)
		}
	}
}
