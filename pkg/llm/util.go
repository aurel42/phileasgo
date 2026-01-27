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
