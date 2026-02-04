package tts

import (
	"context"
)

const (
	// MinAudioSize is the minimum size of a synthesized audio file (1KB).
	// Files smaller than this are likely failed synthesis attempts.
	MinAudioSize = 1024
)

// LanguageProvider allows TTS providers to get the current target language dynamically.
// This enables language changes from the config dialog to take effect without restart.
type LanguageProvider interface {
	ActiveTargetLanguage(ctx context.Context) string
}

// Provider defines the interface for Text-To-Speech engines.
type Provider interface {
	// Synthesize generates audio from text and writes it to outputPath.
	// Returns the audio format ("mp3", "wav") and error.
	Synthesize(ctx context.Context, text, voice, outputPath string) (string, error)

	// Voices returns a list of available voices for the provider.
	// In the future we might return a rich model.Voice struct.
	// For now, simple string slice for IDs and names.
	Voices(ctx context.Context) ([]Voice, error)
}

// Voice represents an available TTS voice.
type Voice struct {
	ID       string
	Name     string
	Language string
	IsNeural bool
}

// FatalError represents a TTS error that should trigger fallback to another provider.
// Examples: rate limits (429), server errors (5xx), auth failures (401/403).
type FatalError struct {
	StatusCode int
	Message    string
}

func (e *FatalError) Error() string {
	return e.Message
}

// NewFatalError creates a new FatalError with the given status code and message.
func NewFatalError(statusCode int, message string) *FatalError {
	return &FatalError{StatusCode: statusCode, Message: message}
}

// IsFatalError checks if an error is a TTS fatal error that should trigger fallback.
func IsFatalError(err error) bool {
	_, ok := err.(*FatalError)
	return ok
}
