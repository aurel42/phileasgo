package narrator

import (
	"os"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

func TestNewLLMProvider(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	// Create a dummy tracker instance
	tracker := tracker.New()
	rc := request.New(nil, tracker, request.ClientConfig{
		Retries:   2,
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  5 * time.Millisecond,
	})
	tmpLog := os.TempDir()

	tests := []struct {
		name    string
		cfg     config.LLMConfig
		wantErr bool
	}{
		{
			name: "Gemini Provider",
			cfg: config.LLMConfig{
				Providers: map[string]config.ProviderConfig{
					"gemini": {
						Type: "gemini",
						Key:  "dummy",
					},
				},
				Fallback: []string{"gemini"},
			},
			wantErr: false,
		},
		{
			name: "Unknown Provider",
			cfg: config.LLMConfig{
				Providers: map[string]config.ProviderConfig{
					"unknown": {
						Type: "unknown",
					},
				},
				Fallback: []string{"unknown"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLLMProvider(tt.cfg, tmpLog, rc, tracker)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLLMProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewTTSProvider(t *testing.T) {
	tracker := tracker.New()

	tests := []struct {
		name    string
		cfg     *config.TTSConfig
		wantErr bool
	}{
		{
			name: "SAPI Provider",
			cfg: &config.TTSConfig{
				Engine: "sapi",
			},
			wantErr: false,
		},
		{
			name: "Edge TTS Provider",
			cfg: &config.TTSConfig{
				Engine: "edge",
			},
			wantErr: false,
		},
		{
			name: "Fish Audio Provider",
			cfg: &config.TTSConfig{
				Engine: "fishaudio",
				FishAudio: config.FishAudioConfig{
					Key: "dummy",
				},
			},
			wantErr: false,
		},
		{
			name: "Azure Speech Provider",
			cfg: &config.TTSConfig{
				Engine: "azure",
				AzureSpeech: config.AzureSpeechConfig{
					Key:    "dummy",
					Region: "westus",
				},
			},
			wantErr: false,
		},
		{
			name: "Unknown Provider",
			cfg: &config.TTSConfig{
				Engine: "unknown",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTTSProvider(tt.cfg, "en-US", tracker)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTTSProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
