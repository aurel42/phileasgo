package tts

import (
	"regexp"
)

var speakerLabelRegex = regexp.MustCompile(`(?m)^[A-Za-z]+(\s*\([^)]+\))?:\s*`)

// StripSpeakerLabels removes speaker labels like "Luna:" or "Aria (female):" from scripts.
func StripSpeakerLabels(script string) string {
	return speakerLabelRegex.ReplaceAllString(script, "")
}
