package narrator

import (
	"os"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

func TestNewLLMProvider(t *testing.T) {
	// Create a dummy tracker instance
	tracker := tracker.New()
	rc := request.New(nil, tracker)
	tmpLog := os.TempDir()

	tests := []struct {
		name    string
		cfg     config.LLMConfig
		wantErr bool
	}{
		{
			name: "Gemini Provider",
			cfg: config.LLMConfig{
				Provider: "gemini",
				Key:      "dummy",
			},
			wantErr: false,
		},
		{
			name: "Unknown Provider",
			cfg: config.LLMConfig{
				Provider: "unknown",
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
