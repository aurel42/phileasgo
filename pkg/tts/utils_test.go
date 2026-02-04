package tts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyAudioFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("FileDoesNotExist", func(t *testing.T) {
		err := VerifyAudioFile(filepath.Join(tmpDir, "missing.mp3"))
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	t.Run("FileTooSmall", func(t *testing.T) {
		path := filepath.Join(tmpDir, "small.mp3")
		if err := os.WriteFile(path, make([]byte, 512), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		err := VerifyAudioFile(path)
		if err == nil {
			t.Error("expected error for small file, got nil")
		}
	})

	t.Run("FileValid", func(t *testing.T) {
		path := filepath.Join(tmpDir, "valid.mp3")
		if err := os.WriteFile(path, make([]byte, MinAudioSize+1), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		err := VerifyAudioFile(path)
		if err != nil {
			t.Errorf("expected no error for valid file, got: %v", err)
		}
	})
}
