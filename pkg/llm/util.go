package llm

import (
	"encoding/json"
	"fmt"
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

// UnmarshalFlexible unmarshals JSON data into the target, but gracefully handles
// the case where a single object is wrapped in a JSON array (e.g. Gemini behavior).
func UnmarshalFlexible(data []byte, target any) error {
	// 1. Try direct unmarshal
	err := json.Unmarshal(data, target)
	if err == nil {
		return nil
	}

	// 2. If it failed, check if it's an array
	var raw []json.RawMessage
	if arrayErr := json.Unmarshal(data, &raw); arrayErr == nil {
		// If it's an array and has exactly one element, try unmarshaling that element
		if len(raw) == 1 {
			return json.Unmarshal(raw[0], target)
		}
	}

	// Return the original error if we couldn't handle it
	return fmt.Errorf("failed to unmarshal JSON: %w", err)
}
