package llm

import (
	"strings"
)

// WordWrap wraps text at the specified width.
func WordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		currentLineLength := 0
		for j, word := range words {
			if j > 0 {
				if currentLineLength+len(word)+1 > width {
					result.WriteString("\n")
					currentLineLength = 0
				} else {
					result.WriteString(" ")
					currentLineLength++
				}
			}
			result.WriteString(word)
			currentLineLength += len(word)
		}
	}

	return result.String()
}

// TruncateParagraphs truncates lines within Wikipedia article blocks (or similar) to maxLen
// and removes empty lines within that block. This is primarily used for logging prompts.
func TruncateParagraphs(text string, maxLen int) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	var result []string
	inWikiBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Heuristic to detect Wikipedia article content block in prompt
		if strings.Contains(trimmed, "WIKIPEDIA ARTICLE:") ||
			strings.Contains(trimmed, "WP ARTICLE:") ||
			strings.Contains(trimmed, "<start of Wikipedia article>") {
			inWikiBlock = true
			result = append(result, line)
			continue
		}

		// Detect end of block
		if inWikiBlock && (strings.Contains(trimmed, "INSTRUCTIONS:") ||
			strings.Contains(trimmed, "PROMPT:") ||
			strings.Contains(trimmed, "<end of Wikipedia article>")) {
			inWikiBlock = false
			if strings.Contains(trimmed, "<end of Wikipedia article>") {
				result = append(result, line)
				continue
			}
		}

		if inWikiBlock {
			if trimmed == "" {
				continue // Skip empty lines in wiki block
			}
			runes := []rune(trimmed)
			if len(runes) > maxLen {
				result = append(result, string(runes[:maxLen])+"...")
			} else {
				result = append(result, trimmed)
			}
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// CleanJSONBlock removes markdown code blocks from a JSON string if present.
func CleanJSONBlock(text string) string {
	text = strings.TrimSpace(text)

	// Look for ```json start
	start := strings.Index(text, "```json")
	if start != -1 {
		text = text[start+len("```json"):]
		// Find end of block
		end := strings.LastIndex(text, "```")
		if end != -1 {
			text = text[:end]
		}
		return strings.TrimSpace(text)
	}

	// Look for generic ``` start
	start = strings.Index(text, "```")
	if start != -1 {
		text = text[start+len("```"):]
		// Find end of block
		end := strings.LastIndex(text, "```")
		if end != -1 {
			text = text[:end]
		}
		return strings.TrimSpace(text)
	}

	return strings.TrimSpace(text)
}
