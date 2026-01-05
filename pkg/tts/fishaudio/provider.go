package fishaudio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"phileasgo/pkg/config"
	"phileasgo/pkg/tts"
)

const (
	apiURL = "https://api.fish.audio/v1/tts"
)

// Provider implements tts.Provider for Fish Audio.
type Provider struct {
	apiKey  string
	voiceID string // Default voice ID (reference_id)
	client  *http.Client
}

// NewProvider creates a new Fish Audio TTS provider.
func NewProvider(cfg config.FishAudioConfig) *Provider {
	return &Provider{
		apiKey:  cfg.Key,
		voiceID: cfg.VoiceID,
		client:  &http.Client{},
	}
}

// requestBody represents the JSON payload for Fish Audio TTS.
type requestBody struct {
	Text        string `json:"text"`
	ReferenceID string `json:"reference_id"`
	Format      string `json:"format"`
	Mp3Bitrate  int    `json:"mp3_bitrate,omitempty"`
	OpusBitrate int    `json:"opus_bitrate,omitempty"`
	Latency     string `json:"latency,omitempty"`
}

// Synthesize generates speech from text using Fish Audio.
func (p *Provider) Synthesize(ctx context.Context, text, voiceID, outputPath string) (string, error) {
	// 1. Determine Voice ID
	vid := p.voiceID
	if voiceID != "" {
		vid = voiceID
	}
	if vid == "" {
		return "", fmt.Errorf("no voice ID configured for Fish Audio")
	}

	// 2. Prepare Request
	reqData := requestBody{
		Text:        text,
		ReferenceID: vid,
		Format:      "mp3",
		Mp3Bitrate:  128, // Standard quality
		Latency:     "normal",
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 3. Execute Request
	resp, err := p.client.Do(req)
	if err != nil {
		tts.Log("FISH", text, 0, err)
		return "", fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tts.Log("FISH", text, resp.StatusCode, nil)
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("fish audio api error (status %d): %s", resp.StatusCode, string(body))
	}

	// 4. Save Output
	tts.Log("FISH", text, 200, nil)
	// Ensure extension
	ext := "mp3"
	filename := outputPath
	if filepath.Ext(filename) != "."+ext {
		filename = filename + "." + ext
	}

	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write audio to file: %w", err)
	}

	return ext, nil
}

// Voices returns a list of available voices (mocked or fetched).
// Fish Audio has thousands of community voices, so we might just return the configured one or a static list.
func (p *Provider) Voices(ctx context.Context) ([]tts.Voice, error) {
	return []tts.Voice{
		{
			ID:       p.voiceID,
			Name:     "Configured Fish Audio Voice",
			Language: "en-US",
			IsNeural: true,
		},
	}, nil
}
