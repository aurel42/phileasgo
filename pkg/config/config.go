package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	TTS      TTSConfig      `yaml:"tts"`
	Log      LogConfig      `yaml:"log"`
	DB       DBConfig       `yaml:"db"`
	Server   ServerConfig   `yaml:"server"`
	Ticker   TickerConfig   `yaml:"ticker"`
	Triggers TriggersConfig `yaml:"triggers"`
	Wikidata WikidataConfig `yaml:"wikidata"`
	Scorer   ScorerConfig   `yaml:"scorer"`
	LLM      LLMConfig      `yaml:"llm"`
	Narrator NarratorConfig `yaml:"narrator"`
	Sim      SimConfig      `yaml:"sim"`
}

// SimConfig holds settings for the simulation connection.
type SimConfig struct {
	Provider string `yaml:"provider"` // "simconnect", "mock"
}

// LLMConfig holds settings for the Large Language Model provider.
type LLMConfig struct {
	Provider string            `yaml:"provider"` // "gemini", "mock", etc.
	Model    string            `yaml:"model"`    // "gemini-2.0-flash"
	Key      string            `yaml:"key"`      // API Key
	Profiles map[string]string `yaml:"profiles"` // Map of intent -> model
}

// EdgeTTSConfig holds settings for Edge TTS.
type EdgeTTSConfig struct {
	VoiceID string `yaml:"voice"` // e.g. "en-US-AvaMultilingualNeural"
}

// FishAudioConfig holds settings for Fish Audio TTS.
type FishAudioConfig struct {
	Key     string `yaml:"key"`   // API Key
	VoiceID string `yaml:"voice"` // Reference ID
}

// AzureSpeechConfig holds settings for Azure Speech TTS.
type AzureSpeechConfig struct {
	Key     string `yaml:"key"`
	Region  string `yaml:"region"` // e.g., "eastus"
	VoiceID string `yaml:"voice"`
}

// TTSConfig holds Text-To-Speech settings.
type TTSConfig struct {
	Engine      string            `yaml:"engine"`
	EdgeTTS     EdgeTTSConfig     `yaml:"edge_tts"`
	FishAudio   FishAudioConfig   `yaml:"fish_audio"`
	AzureSpeech AzureSpeechConfig `yaml:"azure_speech"`
}

// EssayConfig holds settings for essay narration.
type EssayConfig struct {
	Cooldown Duration `yaml:"cooldown"`
}

// NarratorConfig holds settings for the AI narrator.
type NarratorConfig struct {
	AutoNarrate        bool        `yaml:"auto_narrate"`
	MinScoreThreshold  float64     `yaml:"min_score_threshold"`
	CooldownMin        Duration    `yaml:"cooldown_min"`
	CooldownMax        Duration    `yaml:"cooldown_max"`
	RepeatTTL          Duration    `yaml:"repeat_ttl"`
	TargetLanguage     string      `yaml:"target_language"`
	Units              string      `yaml:"units"`
	NarrationLengthMin int         `yaml:"narration_length_min"` // Random range min (default 400)
	NarrationLengthMax int         `yaml:"narration_length_max"` // Random range max (default 600)
	TemperatureBase    float32     `yaml:"temperature_base"`     // Base temperature (default 1.0)
	TemperatureJitter  float32     `yaml:"temperature_jitter"`   // Jitter range (bell curve distribution)
	Essay              EssayConfig `yaml:"essay"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Server   LogSettings `yaml:"server"`
	Requests LogSettings `yaml:"requests"`
	Gemini   LogSettings `yaml:"gemini"`
}

// DBConfig holds database settings.
type DBConfig struct {
	Path string `yaml:"path"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Address string `yaml:"address"`
}

// TickerConfig holds ticker settings.
type TickerConfig struct {
	TelemetryLoop Duration `yaml:"telemetry_loop"`
}

// TriggersConfig holds job scheduling thresholds.
type TriggersConfig struct {
	Distance Distance `yaml:"distance"`
	Time     Duration `yaml:"time"`
}

// WikidataConfig holds Wikidata-specific settings.
type WikidataConfig struct {
	Area AreaConfig `yaml:"area"`
}

// AreaConfig holds settings for area-based Wikidata queries.
type AreaConfig struct {
	MaxArticles int     `yaml:"max_articles"`
	MaxDist     float64 `yaml:"max_dist_km"`
}

// ScorerConfig holds settings for the POI scorer.
type ScorerConfig struct {
	VarietyPenaltyFirst float64 `yaml:"variety_penalty_first"`
	VarietyPenaltyLast  float64 `yaml:"variety_penalty_last"`
	VarietyPenaltyNum   int     `yaml:"variety_penalty_num"`
}

// LogSettings holds settings for a specific logger.
type LogSettings struct {
	Path  string `yaml:"path"`
	Level string `yaml:"level"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		TTS: TTSConfig{
			Engine: "windows-sapi",
			EdgeTTS: EdgeTTSConfig{
				VoiceID: "en-US-AvaMultilingualNeural",
			},
			FishAudio: FishAudioConfig{
				VoiceID: "389274291", // Example ID, placeholder
			},
			AzureSpeech: AzureSpeechConfig{
				VoiceID: "en-US-AvaMultilingualNeural",
			},
		},
		Log: LogConfig{
			Server: LogSettings{
				Path:  "./logs/server.log",
				Level: "DEBUG",
			},
			Requests: LogSettings{
				Path:  "./logs/requests.log",
				Level: "INFO",
			},
			Gemini: LogSettings{
				Path:  "./logs/gemini.log",
				Level: "DEBUG",
			},
		},
		DB: DBConfig{
			Path: "./data/phileas.db",
		},
		Server: ServerConfig{
			Address: "localhost:1920",
		},
		Ticker: TickerConfig{
			TelemetryLoop: Duration(1 * time.Second),
		},
		Triggers: TriggersConfig{
			Distance: Distance(5000), // 5km
			Time:     Duration(30 * time.Second),
		},
		Wikidata: WikidataConfig{
			Area: AreaConfig{
				MaxArticles: 100,
				MaxDist:     100.0,
			},
		},
		Scorer: ScorerConfig{
			VarietyPenaltyFirst: 0.1,
			VarietyPenaltyLast:  0.5,
			VarietyPenaltyNum:   3,
		},
		LLM: LLMConfig{
			Provider: "gemini",
			Model:    "gemini-2.0-flash",
			Key:      "",
		},
		Narrator: NarratorConfig{
			AutoNarrate:        true,
			MinScoreThreshold:  0.5,
			CooldownMin:        Duration(30 * time.Second),
			CooldownMax:        Duration(60 * time.Second),
			RepeatTTL:          Duration(Week),
			TargetLanguage:     "en-US",
			Units:              "hybrid",
			NarrationLengthMin: 400,
			NarrationLengthMax: 600,
			TemperatureBase:    1.0,
			TemperatureJitter:  0.2,
			Essay: EssayConfig{
				Cooldown: Duration(15 * time.Minute),
			},
		},
		Sim: SimConfig{
			Provider: "simconnect",
		},
	}
}

// Load loads the configuration from the given path.
// If the file does not exist, it creates it with default values.
// If the file exists, it merges defaults with existing values and saves the result (to ensure new keys are added).
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing file if it exists
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		// Load from Env if empty (as a fallback, but do NOT save back to disk)
		if cfg.LLM.Key == "" {
			if key := os.Getenv("GEMINI_API_KEY"); key != "" {
				cfg.LLM.Key = key
			}
		}
		if cfg.TTS.FishAudio.Key == "" {
			if key := os.Getenv("FISH_AUDIO_API_KEY"); key != "" {
				cfg.TTS.FishAudio.Key = key
			}
		}
		if cfg.TTS.AzureSpeech.Key == "" {
			if key := os.Getenv("AZURE_SPEECH_KEY"); key != "" {
				cfg.TTS.AzureSpeech.Key = key
			}
		}
		if cfg.TTS.AzureSpeech.Region == "" {
			if region := os.Getenv("AZURE_SPEECH_REGION"); region != "" {
				cfg.TTS.AzureSpeech.Region = region
			}
		}

		return cfg, nil
	}

	// If file does not exist, save defaults
	if err := Save(path, cfg); err != nil {
		return nil, fmt.Errorf("failed to save config file: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to the path.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// GenerateDefault creates a default config file at the given path.
// Returns nil if the file already exists.
func GenerateDefault(path string) error {
	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, do nothing
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config
	return Save(path, DefaultConfig())
}
