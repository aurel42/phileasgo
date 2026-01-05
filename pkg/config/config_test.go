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
			},
			checkFile: func(t *testing.T) {
				content, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("failed to read config file: %v", err)
				}
				if !strings.Contains(string(content), "engine: windows-sapi") {
					t.Error("config file missing default values")
				}
			},
		},
		{
			name: "ExistingFile_Override",
			setup: func() {
				// Pre-create file with custom value
				err := os.WriteFile(configPath, []byte("tts:\n  engine: google\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TTS.Engine != "google" {
					t.Errorf("expected TTS engine 'google', got '%s'", cfg.TTS.Engine)
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
