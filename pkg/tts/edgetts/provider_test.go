package edgetts

import (
	"bytes"
	"context"
	"os"
	"phileasgo/pkg/tracker"
	"testing"
)

func TestHandleBinaryMessage(t *testing.T) {
	p := NewProvider(tracker.New())

	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test_audio_*.mp3")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 1. Valid message with header
	// Header length 4 bytes (0x00 0x04)
	header := []byte("info")
	audio := []byte{0x01, 0x02, 0x03, 0x04}
	data := append([]byte{0x00, 0x04}, header...)
	data = append(data, audio...)

	err = p.handleBinaryMessage(data, tmpFile)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify content
	content, _ := os.ReadFile(tmpFile.Name())
	if !bytes.Equal(content, audio) {
		t.Errorf("Expected audio data %v, got %v", audio, content)
	}

	// 2. Too short
	short := []byte{0x00}
	err = p.handleBinaryMessage(short, tmpFile)
	if err != nil {
		t.Errorf("Too short message should be ignored, got %v", err)
	}
}

func TestVoices(t *testing.T) {
	p := NewProvider(tracker.New())
	voices, err := p.Voices(context.TODO())
	if err != nil {
		t.Fatalf("Voices failed: %v", err)
	}
	if len(voices) == 0 {
		t.Error("Expected at least one voice")
	}
	found := false
	for _, v := range voices {
		if v.ID == "en-US-AvaMultilingualNeural" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Default voice not found in list")
	}
}

func TestGenerateSecMSGec(t *testing.T) {
	p := NewProvider(tracker.New())
	token := p.generateSecMSGec()
	if len(token) == 0 {
		t.Error("Generated token should not be empty")
	}
	// It should be a hex string
	if len(token) != 64 {
		// SHA256 hex string is 64 chars
		t.Errorf("Expected token length 64, got %d", len(token))
	}
}
