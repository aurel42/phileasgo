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
		{
			name: "LLM_Env_Override",
			setup: func() {
				t.Setenv("GEMINI_API_KEY", "env_secret_key")
				// Provide config with empty key for gemini
				err := os.WriteFile(configPath, []byte("llm:\n  providers:\n    p1:\n      type: gemini\n      key: \"\"\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				p1, ok := cfg.LLM.Providers["p1"]
				if !ok {
					t.Fatal("provider p1 missing")
				}
				if p1.Key != "env_secret_key" {
					t.Errorf("expected Key 'env_secret_key', got '%s'", p1.Key)
				}
			},
			checkFile: func(t *testing.T) {
				// Env overrides should NOT be saved to disk
				content, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("failed to read config file: %v", err)
				}
				if strings.Contains(string(content), "env_secret_key") {
					t.Error("environment secret should NOT be persisted to config file")
				}
			},
		},
		{
			name: "Path_Env_Expansion",
			setup: func() {
				t.Setenv("PHILEAS_HOME", "/home/phileas")
				t.Setenv("APP_DATA", "/app/data")
				err := os.WriteFile(configPath, []byte("db:\n  path: \"$PHILEAS_HOME/db.sqlite\"\nnarrator:\n  screenshot:\n    path: \"%APP_DATA%/screenshots\"\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				expectedDB := "/home/phileas/db.sqlite"
				if cfg.DB.Path != expectedDB {
					t.Errorf("expected DB path '%s', got '%s'", expectedDB, cfg.DB.Path)
				}
				expectedScreenshot := "/app/data/screenshots"
				if cfg.Narrator.Screenshot.Path != expectedScreenshot {
					t.Errorf("expected Screenshot path '%s', got '%s'", expectedScreenshot, cfg.Narrator.Screenshot.Path)
				}
			},
			checkFile: func(t *testing.T) {
				// Original raw paths with variables should be preserved on disk
				content, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatalf("failed to read config file: %v", err)
				}
				if !strings.Contains(string(content), "$PHILEAS_HOME") {
					t.Error("config file should persist raw $VAR path")
				}
				if !strings.Contains(string(content), "%APP_DATA%") {
					t.Error("config file should persist raw %VAR% path")
				}
			},
		},
		{
			name: "Invalid_YAML",
			setup: func() {
				err := os.WriteFile(configPath, []byte("narrator: [not a map]"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			expectedError: true,
		},
		{
			name: "Invalid_Locale",
			setup: func() {
				err := os.WriteFile(configPath, []byte("narrator:\n  target_language: invalid\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				// This shouldn't be reached as Load should return error
			},
			expectedError: true,
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

func TestGenerateDefault(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "default_config.yaml")

	err := GenerateDefault(configPath)
	if err != nil {
		t.Fatalf("GenerateDefault() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("GenerateDefault() did not create file")
	}

	// Running again should not fail
	err = GenerateDefault(configPath)
	if err != nil {
		t.Errorf("GenerateDefault() error on second run = %v", err)
	}
}
