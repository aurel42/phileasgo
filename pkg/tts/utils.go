package tts

import (
	"fmt"
	"os"
	"regexp"
)

var speakerLabelRegex = regexp.MustCompile(`(?m)^[A-Za-z]+(\s*\([^)]+\))?:\s*`)

// VerifyAudioFile checks if the audio file at path exists and is larger than MinAudioSize.
func VerifyAudioFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("audio file does not exist: %s", path)
		}
		return fmt.Errorf("failed to stat audio file: %w", err)
	}

	if info.Size() < MinAudioSize {
		return fmt.Errorf("audio file too small (%d bytes), minimum is %d bytes", info.Size(), MinAudioSize)
	}

	return nil
}

// StripSpeakerLabels removes speaker labels like "Luna:" or "Aria (female):" from scripts.
func StripSpeakerLabels(script string) string {
	return speakerLabelRegex.ReplaceAllString(script, "")
}
