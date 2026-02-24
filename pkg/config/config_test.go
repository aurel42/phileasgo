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
			name: "LLM_From_Sibling",
			setup: func() {
				// Main config has no LLM block; sibling llm.yaml provides it
				err := os.WriteFile(configPath, []byte("tts:\n  engine: edge-tts\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
				llmPath := filepath.Join(filepath.Dir(configPath), "llm.yaml")
				err = os.WriteFile(llmPath, []byte("llm:\n  providers:\n    groq:\n      type: openai\n      base_url: https://api.groq.com/openai/v1\n      profiles:\n        narration: llama-3.1-8b-instant\n  fallback: [\"groq\"]\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup llm.yaml: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.LLM.Providers) != 1 {
					t.Fatalf("expected 1 provider from sibling, got %d", len(cfg.LLM.Providers))
				}
				groq, ok := cfg.LLM.Providers["groq"]
				if !ok {
					t.Fatal("expected groq provider from sibling")
				}
				if groq.Profiles["narration"] != "llama-3.1-8b-instant" {
					t.Errorf("expected groq narration model, got %s", groq.Profiles["narration"])
				}
			},
			checkFile: func(t *testing.T) {},
		},
		{
			name: "LLM_PhileasOverride",
			setup: func() {
				// Main config HAS an LLM block — should win over sibling
				err := os.WriteFile(configPath, []byte("llm:\n  providers:\n    gemini:\n      type: gemini\n      profiles:\n        narration: gemini-pro\n  fallback: [\"gemini\"]\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
				// Sibling also exists but should be ignored
				llmPath := filepath.Join(filepath.Dir(configPath), "llm.yaml")
				err = os.WriteFile(llmPath, []byte("llm:\n  providers:\n    groq:\n      type: openai\n      profiles:\n        narration: llama\n  fallback: [\"groq\"]\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup llm.yaml: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				// Should have gemini from phileas.yaml, NOT groq from sibling
				if len(cfg.LLM.Providers) != 1 {
					t.Fatalf("expected 1 provider, got %d", len(cfg.LLM.Providers))
				}
				if _, ok := cfg.LLM.Providers["gemini"]; !ok {
					t.Error("expected gemini from phileas.yaml override, not sibling")
				}
			},
			checkFile: func(t *testing.T) {},
		},
		{
			name: "LLM_Env_Override",
			setup: func() {
				t.Setenv("GEMINI_API_KEY", "env_secret_key")
				// Provide config with empty key for gemini via sibling
				err := os.WriteFile(configPath, []byte("tts:\n  engine: edge-tts\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
				llmPath := filepath.Join(filepath.Dir(configPath), "llm.yaml")
				err = os.WriteFile(llmPath, []byte("llm:\n  providers:\n    gemini:\n      type: gemini\n      key: \"\"\n  fallback: [\"gemini\"]\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup llm.yaml: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				p, ok := cfg.LLM.Providers["gemini"]
				if !ok {
					t.Fatal("provider gemini missing")
				}
				if p.Key != "env_secret_key" {
					t.Errorf("expected Key 'env_secret_key', got '%s'", p.Key)
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
				err := os.WriteFile(configPath, []byte("db:\n  path: \"$PHILEAS_HOME/db.sqlite\"\nnarrator:\n  screenshot:\n    paths: [\"%APP_DATA%/screenshots\"]\n"), 0o644)
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
				if len(cfg.Narrator.Screenshot.Paths) == 0 || cfg.Narrator.Screenshot.Paths[0] != expectedScreenshot {
					t.Errorf("expected Screenshot path '%s', got '%v'", expectedScreenshot, cfg.Narrator.Screenshot.Paths)
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
				err := os.WriteFile(configPath, []byte("narrator:\n  active_target_language: invalid\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				// This shouldn't be reached as Load should return error
			},
			expectedError: true,
		},
		{
			name: "Secrets_Env_Override",
			setup: func() {
				t.Setenv("FISH_API_KEY", "fish_secret")
				t.Setenv("SPEECH_KEY", "azure_secret")
				t.Setenv("SPEECH_REGION", "eastus")
				err := os.WriteFile(configPath, []byte("tts:\n  engine: edge-tts\n"), 0o644)
				if err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TTS.FishAudio.Key != "fish_secret" {
					t.Errorf("expected FishAudio Key 'fish_secret', got '%s'", cfg.TTS.FishAudio.Key)
				}
				if cfg.TTS.AzureSpeech.Key != "azure_secret" {
					t.Errorf("expected AzureSpeech Key 'azure_secret', got '%s'", cfg.TTS.AzureSpeech.Key)
				}
				if cfg.TTS.AzureSpeech.Region != "eastus" {
					t.Errorf("expected AzureSpeech Region 'eastus', got '%s'", cfg.TTS.AzureSpeech.Region)
				}
			},
			checkFile: func(t *testing.T) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cleanup for fresh run if needed, but we rely on tempDir state progression or overwrite?
			// Ideally each test case should run in isolation or cleanup.
			// However `Load` overwrites. But to be safe let's remove file before `setup`.
			os.Remove(configPath)
			os.Remove(filepath.Join(filepath.Dir(configPath), "llm.yaml"))
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
