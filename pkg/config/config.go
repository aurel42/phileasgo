package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Request  RequestConfig  `yaml:"request"`
	TTS      TTSConfig      `yaml:"tts"`
	Log      LogConfig      `yaml:"log"`
	DB       DBConfig       `yaml:"db"`
	Server   ServerConfig   `yaml:"server"`
	Ticker   TickerConfig   `yaml:"ticker"`
	Triggers TriggersConfig `yaml:"triggers"`
	Wikidata WikidataConfig `yaml:"wikidata"`
	Terrain  TerrainConfig  `yaml:"terrain"`
	Scorer   ScorerConfig   `yaml:"scorer"`
	LLM      LLMConfig      `yaml:"llm"`
	Narrator NarratorConfig `yaml:"narrator"`
	Sim      SimConfig      `yaml:"sim"`
	Beacon   BeaconConfig   `yaml:"beacon"`
	Overlay  OverlayConfig  `yaml:"overlay"`
}

// OverlayConfig holds settings for the overlay UI.
type OverlayConfig struct {
	MapBox  bool `yaml:"map_box"`
	POIInfo bool `yaml:"poi_info"`
	InfoBar bool `yaml:"info_bar"`
	LogLine bool `yaml:"log_line"`
}

// RequestConfig holds HTTP request settings.
type RequestConfig struct {
	Retries int           `yaml:"retries"`
	Timeout Duration      `yaml:"timeout"`
	Backoff BackoffConfig `yaml:"backoff"`
}

// BackoffConfig holds exponential backoff settings.
type BackoffConfig struct {
	BaseDelay Duration `yaml:"base_delay"`
	MaxDelay  Duration `yaml:"max_delay"`
}

// SimConfig holds settings for the simulation connection.
type SimConfig struct {
	Provider          string        `yaml:"provider"` // "simconnect", "mock"
	TeleportThreshold Distance      `yaml:"teleport_distance"`
	Mock              MockSimConfig `yaml:"mock"`
}

// MockSimConfig holds settings for the mock simulation.
type MockSimConfig struct {
	StartLat       float64  `yaml:"start_lat"`
	StartLon       float64  `yaml:"start_lon"`
	StartAlt       float64  `yaml:"start_alt"`
	StartHeading   *float64 `yaml:"start_heading"`
	DurationParked Duration `yaml:"duration_parked"`
	DurationTaxi   Duration `yaml:"duration_taxi"`
	DurationHold   Duration `yaml:"duration_hold"`
}

// BeaconConfig holds settings for the beacon guidance system.
type BeaconConfig struct {
	Enabled           bool     `yaml:"enabled"`
	FormationEnabled  bool     `yaml:"formation_enabled"`
	FormationDistance Distance `yaml:"formation_distance"`
	FormationCount    int      `yaml:"formation_count"`
	MinSpawnAltitude  Distance `yaml:"min_spawn_altitude"`
	AltitudeFloor     Distance `yaml:"altitude_floor"`
}

// LLMConfig holds settings for the Large Language Model providers.
type LLMConfig struct {
	Providers map[string]ProviderConfig `yaml:"providers"` // Map of named providers
	Fallback  []string                  `yaml:"fallback"`  // Ordered list of providers for failover
}

// ProviderConfig holds configuration for a single LLM provider.
type ProviderConfig struct {
	Type     string            `yaml:"type"`      // "gemini", "groq", "openai"
	Key      string            `yaml:"-"`         // API Key (Loaded from Env)
	Profiles map[string]string `yaml:"profiles"`  // Map of intent -> model
	FreeTier bool              `yaml:"free_tier"` // Whether this is a free tier (usually shared)
}

// EdgeTTSConfig holds settings for Edge TTS.
type EdgeTTSConfig struct {
	VoiceID  string `yaml:"voice"`     // e.g. "en-US-AvaMultilingualNeural"
	FreeTier bool   `yaml:"free_tier"` // Default true for Edge
}

// FishAudioConfig holds settings for Fish Audio TTS.
type FishAudioConfig struct {
	Key      string `yaml:"-"`         // API Key
	VoiceID  string `yaml:"voice"`     // Reference ID
	Model    string `yaml:"model"`     // Model ID (e.g. "s1")
	FreeTier bool   `yaml:"free_tier"` // Default depends on plan
}

// AzureSpeechConfig holds settings for Azure Speech TTS.
type AzureSpeechConfig struct {
	Key      string `yaml:"-"`
	Region   string `yaml:"-"`
	VoiceID  string `yaml:"voice"`
	FreeTier bool   `yaml:"free_tier"`
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
	Enabled            bool     `yaml:"enabled"`
	DelayBetweenEssays Duration `yaml:"delay_between_essays"`
	DelayBeforeEssay   Duration `yaml:"delay_before_essay"`
	ScoreThreshold     float64  `yaml:"score_threshold"`
}

// AudioEffectsConfig holds settings for audio post-processing.
type AudioEffectsConfig struct {
	Headset    bool    `yaml:"headset"`
	LowCutoff  float64 `yaml:"low_cutoff"`
	HighCutoff float64 `yaml:"high_cutoff"`
}

// NarratorConfig holds settings for the AI narrator.
type NarratorConfig struct {
	AutoNarrate               bool               `yaml:"auto_narrate"`
	MinScoreThreshold         float64            `yaml:"min_score_threshold"`
	Frequency                 int                `yaml:"frequency"` // 1=Rarely...5=Constant
	PauseDuration             Duration           `yaml:"pause_between_narrations"`
	RepeatTTL                 Duration           `yaml:"repeat_ttl"`
	TargetLanguage            string             `yaml:"target_language"`
	Units                     string             `yaml:"units"`
	NarrationLengthShortWords int                `yaml:"narration_length_short_words"` // Target for short narrations (default 50)
	NarrationLengthLongWords  int                `yaml:"narration_length_long_words"`  // Target for long narrations (default 200)
	SummaryMaxWords           int                `yaml:"summary_max_words"`            // Max words for the trip summary (default 500)
	TemperatureBase           float32            `yaml:"temperature_base"`             // Base temperature (default 1.0)
	TemperatureJitter         float32            `yaml:"temperature_jitter"`           // Jitter range (bell curve distribution)
	Essay                     EssayConfig        `yaml:"essay"`
	Debrief                   DebriefConfig      `yaml:"debrief"`
	Screenshot                ScreenshotConfig   `yaml:"screenshot"`
	AudioEffects              AudioEffectsConfig `yaml:"audio_effects"`
}

// DebriefConfig holds settings for landing debriefs.
type DebriefConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ScreenshotConfig holds settings for screenshot monitoring.
type ScreenshotConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"` // Defaults to user's Pictures/Screenshots if empty
}

// LogConfig holds logging settings.
type LogConfig struct {
	Server   LogSettings `yaml:"server"`
	Requests LogSettings `yaml:"requests"`
	LLM      LogSettings `yaml:"llm"`
	TTS      LogSettings `yaml:"tts"`
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
	Area          AreaConfig `yaml:"area"`
	FetchInterval Duration   `yaml:"fetch_interval"`
}

// TerrainConfig holds terrain and line-of-sight settings.
type TerrainConfig struct {
	LineOfSight   bool   `yaml:"line_of_sight"`
	ElevationFile string `yaml:"elevation_file"`
}

// AreaConfig holds settings for area-based Wikidata queries.
type AreaConfig struct {
	MaxArticles int      `yaml:"max_articles"`
	MaxDist     Distance `yaml:"max_dist"`
}

// ScorerConfig holds settings for the POI scorer.
type ScorerConfig struct {
	VarietyPenaltyFirst float64 `yaml:"variety_penalty_first"`
	VarietyPenaltyLast  float64 `yaml:"variety_penalty_last"`
	VarietyPenaltyNum   int     `yaml:"variety_penalty_num"`
	NoveltyBoost        float64 `yaml:"novelty_boost"`
	GroupPenalty        float64 `yaml:"group_penalty"`
}

// LogSettings holds settings for a specific logger.
type LogSettings struct {
	Path  string `yaml:"path"`
	Level string `yaml:"level"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Request: RequestConfig{
			Retries: 5,
			Timeout: Duration(300 * time.Second),
			Backoff: BackoffConfig{
				BaseDelay: Duration(1 * time.Second),
				MaxDelay:  Duration(60 * time.Second),
			},
		},
		TTS: TTSConfig{
			Engine: "windows-sapi",
			EdgeTTS: EdgeTTSConfig{
				VoiceID:  "en-US-AvaMultilingualNeural",
				FreeTier: true,
			},
			FishAudio: FishAudioConfig{
				VoiceID: "e58b0d7efca34eb38d5c4985e378abcb",
			},
			AzureSpeech: AzureSpeechConfig{
				VoiceID: "en-US-AvaMultilingualNeural",
			},
		},
		Log: LogConfig{
			Server: LogSettings{
				Path:  "./logs/server.log",
				Level: "INFO",
			},
			Requests: LogSettings{
				Path:  "./logs/requests.log",
				Level: "INFO",
			},
			LLM: LogSettings{
				Path:  "./logs/llm.log",
				Level: "INFO",
			},
			TTS: LogSettings{
				Path:  "./logs/tts.log",
				Level: "INFO",
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
				MaxArticles: 500,
				MaxDist:     Distance(80000), // 80km
			},
			FetchInterval: Duration(5 * time.Second),
		},
		Terrain: TerrainConfig{
			LineOfSight:   true,
			ElevationFile: "data/etopo1/etopo1_ice_g_i2.bin",
		},
		Scorer: ScorerConfig{
			VarietyPenaltyFirst: 0.1,
			VarietyPenaltyLast:  0.5,
			VarietyPenaltyNum:   3,
			NoveltyBoost:        1.3,
			GroupPenalty:        0.5,
		},
		LLM: LLMConfig{
			Providers: map[string]ProviderConfig{
				"gemini": {
					Type: "gemini",
					Key:  "",
					Profiles: map[string]string{
						"essay":          "gemini-2.5-flash",
						"narration":      "gemini-2.5-flash-lite",
						"dynamic_config": "gemini-2.5-flash",
						"script_rescue":  "gemini-2.5-flash-lite",
						"thumbnails":     "gemini-2.5-flash-lite",
						"screenshot":     "gemini-2.5-flash-lite",
					},
					FreeTier: true,
				},
			},
			Fallback: []string{"gemini"},
		},
		Narrator: NarratorConfig{
			AutoNarrate:               true,
			MinScoreThreshold:         0.5,
			Frequency:                 3, // Active
			PauseDuration:             Duration(4 * time.Second),
			RepeatTTL:                 Duration(30 * 24 * time.Hour), // 30d
			TargetLanguage:            "en-US",
			Units:                     "hybrid",
			NarrationLengthShortWords: 50,
			NarrationLengthLongWords:  200,
			SummaryMaxWords:           500,
			TemperatureBase:           1.0,
			TemperatureJitter:         0.3,
			Essay: EssayConfig{
				Enabled:            true,
				DelayBetweenEssays: Duration(10 * time.Minute),
				DelayBeforeEssay:   Duration(2 * time.Minute),
				ScoreThreshold:     2.0,
			},
			Debrief: DebriefConfig{
				Enabled: true,
			},
			Screenshot: ScreenshotConfig{
				Enabled: true,
				Path:    "", // Auto-detect
			},
			AudioEffects: AudioEffectsConfig{
				Headset:    false,
				LowCutoff:  400.0,
				HighCutoff: 3500.0,
			},
		},
		Sim: SimConfig{
			Provider:          "simconnect",
			TeleportThreshold: Distance(80000), // 80km
			Mock: MockSimConfig{
				StartLat: 51.6845,
				StartLon: 14.4234,
				StartAlt: 285.0,
				// StartHeading defaults to nil (random)
				DurationParked: Duration(120 * time.Second),
				DurationTaxi:   Duration(120 * time.Second),
				DurationHold:   Duration(30 * time.Second),
			},
		},
		Beacon: BeaconConfig{
			Enabled:           true,
			FormationEnabled:  true,
			FormationDistance: Distance(2000), // 2km
			FormationCount:    3,
			MinSpawnAltitude:  Distance(304.8), // 1000ft
			AltitudeFloor:     Distance(609.6), // 2000ft
		},
		Overlay: OverlayConfig{
			MapBox:  true,
			POIInfo: true,
			InfoBar: true,
			LogLine: true,
		},
	}
}

// Load loads the configuration from the given path.
// If the file does not exist, it creates it with default values.
// If the file exists, it merges defaults with existing values but does NOT save back to disk (to preserve user formatting and comments).
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

		// Load .env files (local first, then default)
		// We ignore errors here because it's valid to rely solely on system env vars
		_ = godotenv.Load(".env.local", ".env")

		// Load secrets from Env
		loadSecretsFromEnv(cfg)

		return cfg, nil
	}

	// If file does not exist, save defaults
	if err := Save(path, cfg); err != nil {
		return nil, fmt.Errorf("failed to save config file: %w", err)
	}

	// Validate TargetLanguage format (xx-YY)
	if !isValidLocale(cfg.Narrator.TargetLanguage) {
		return nil, fmt.Errorf("invalid target_language format '%s': must be 'xx-YY' (e.g. 'en-US', 'de-DE')", cfg.Narrator.TargetLanguage)
	}

	return cfg, nil
}

func isValidLocale(s string) bool {
	matched, _ := regexp.MatchString(`^[a-z]{2}-[A-Z]{2}$`, s)
	return matched
}

// Save writes the configuration to the path.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	header := []byte(`# PhileasGo Configuration
# ---------------------
# Supported Units:
#   Duration: ns, us (or µs), ms, s, m, h, d (day), w (week)
#   Distance: m (meters), km (kilometers), nm (nautical miles)

`)
	data = append(header, data...)

	// Inject comments for Enum fields
	// We use regex to find the keys with indentation to ensure we place comments correctly.

	// TTS Engine Options
	reEngine := regexp.MustCompile(`(?m)^(\s+)engine:`)
	data = reEngine.ReplaceAll(data, []byte("${1}# Options: windows-sapi, edge-tts, fish-audio, azure-speech\n${1}engine:"))

	// Narrator Units Options
	reUnits := regexp.MustCompile(`(?m)^(\s+)units:`)
	data = reUnits.ReplaceAll(data, []byte("${1}# Options: metric, imperial, hybrid\n${1}units:"))

	// Temperature Jitter Comment
	reTemp := regexp.MustCompile(`(?m)^(\s+)temperature_jitter:`)
	data = reTemp.ReplaceAll(data, []byte("${1}# Bell curve: most likely 1.0, range [0.7, 1.3]\n${1}temperature_jitter:"))

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

func loadSecretsFromEnv(cfg *Config) {
	// LLM Providers
	// We iterate over the configured providers and look for specific Env Vars
	for name, p := range cfg.LLM.Providers {
		switch p.Type {
		case "gemini":
			if key := os.Getenv("GEMINI_API_KEY"); key != "" {
				p.Key = key
			}
		case "groq":
			if key := os.Getenv("GROQ_API_KEY"); key != "" {
				p.Key = key
			}
		}
		// Update the map because p is a copy
		cfg.LLM.Providers[name] = p
	}

	// TTS - Fish Audio
	if key := os.Getenv("FISH_API_KEY"); key != "" {
		cfg.TTS.FishAudio.Key = key
	}

	// TTS - Azure Speech
	if key := os.Getenv("SPEECH_KEY"); key != "" {
		cfg.TTS.AzureSpeech.Key = key
	}
	if region := os.Getenv("SPEECH_REGION"); region != "" {
		cfg.TTS.AzureSpeech.Region = region
	}
}
