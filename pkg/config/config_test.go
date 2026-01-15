package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "phileas.yaml")

	tests := []struct {
		name          string
		setup         func()
		validate      func(*testing.T, *Config)
		checkFile     func(*testing.T)
		expectedError bool
	}{
		{
			name:  "NewFile_Defaults",
			setup: func() {}, // No file
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TTS.Engine != "windows-sapi" {
					t.Errorf("expected default TTS engine 'windows-sapi', got '%s'", cfg.TTS.Engine)
				}
				if cfg.Narrator.NarrationLengthShortWords != 50 {
					t.Errorf("expected ShortWords default 50, got %d", cfg.Narrator.NarrationLengthShortWords)
				}
			},
			checkFile: func(t *testing.T) {
				content, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("failed to read config file: %v", err)
				}
				if !strings.Contains(string(content), "engine: windows-sapi") {
					t.Error("config file missing default values")
				}
				if !strings.Contains(string(content), "narration_length_short_words: 50") {
					t.Error("config file missing narration_length_short_words default")
				}
			},
		},
		{
			name: "ExistingFile_Override",
			setup: func() {
				// Pre-create file with custom value
				err := os.WriteFile(configPath, []byte("tts:\n  engine: google\nnarrator:\n  summary_max_words: 300\n  narration_length_long_words: 999\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TTS.Engine != "google" {
					t.Errorf("expected TTS engine 'google', got '%s'", cfg.TTS.Engine)
				}
				if cfg.Narrator.SummaryMaxWords != 300 {
					t.Errorf("expected SummaryMaxWords 300, got %d", cfg.Narrator.SummaryMaxWords)
				}
				if cfg.Narrator.NarrationLengthLongWords != 999 {
					t.Errorf("expected LongWords 999, got %d", cfg.Narrator.NarrationLengthLongWords)
				}
			},
			checkFile: func(t *testing.T) {
				content, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("failed to read config file: %v", err)
				}
				if !strings.Contains(string(content), "engine: google") {
					t.Error("config file should persist custom value")
				}
				if !strings.Contains(string(content), "summary_max_words: 300") {
					t.Error("config file missing summary_max_words")
				}
			},
		},
		{
			name: "NewField_Persistence",
			setup: func() {
				err := os.WriteFile(configPath, []byte("narrator:\n  summary_max_words: 750\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Narrator.SummaryMaxWords != 750 {
					t.Errorf("expected SummaryMaxWords 750, got %d", cfg.Narrator.SummaryMaxWords)
				}
			},
			checkFile: func(t *testing.T) {
				content, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("failed to read config file: %v", err)
				}
				if !strings.Contains(string(content), "summary_max_words: 750") {
					t.Error("config file should persist summary_max_words")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cleanup for fresh run if needed, but we rely on tempDir state progression or overwrite?
			// Ideally each test case should run in isolation or cleanup.
			// However `Load` overwrites. But to be safe let's remove file before `setup`.
			os.Remove(configPath)
			tt.setup()

			cfg, err := Load(configPath)
			if (err != nil) != tt.expectedError {
				t.Fatalf("Load() error = %v, expectedError %v", err, tt.expectedError)
			}
			if err == nil {
				tt.validate(t, cfg)
				tt.checkFile(t)
			}
		})
	}
}
