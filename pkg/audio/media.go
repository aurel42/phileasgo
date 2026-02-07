package audio

import (
	"time"
)

// GetDuration returns the duration of the audio file at the given path.
// It opens the file, decodes it, and calculates the duration based on its sample length.
func GetDuration(path string) (time.Duration, error) {
	streamer, format, err := DecodeMedia(path)
	if err != nil {
		return 0, err
	}
	defer streamer.Close()

	return format.SampleRate.D(streamer.Len()), nil
}
