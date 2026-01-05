package tts

import (
	"context"
)

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
