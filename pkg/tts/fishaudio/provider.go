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
	"strings"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
)

const (
	apiURL = "https://api.fish.audio/v1/tts"
)

// Provider implements tts.Provider for Fish Audio.
type Provider struct {
	apiKey  string
	voiceID string // Default voice ID (reference_id)
	modelID string // Model ID (e.g. "s1")
	client  *http.Client
	tracker *tracker.Tracker
}

// NewProvider creates a new Fish Audio TTS provider.
func NewProvider(cfg config.FishAudioConfig, t *tracker.Tracker) *Provider {
	return &Provider{
		apiKey:  cfg.Key,
		voiceID: cfg.VoiceID,
		modelID: cfg.Model,
		client:  &http.Client{},
		tracker: t,
	}
}

// requestBody represents the JSON payload for Fish Audio TTS.
type requestBody struct {
	Text        string `json:"text"`
	ReferenceID string `json:"reference_id"`
	ModelID     string `json:"model,omitempty"` // Added model field
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

	// 2. Prepare Request Data
	reqData := requestBody{
		Text:        text,
		ReferenceID: vid,
		ModelID:     p.modelID,
		Format:      "mp3",
		Mp3Bitrate:  128, // Standard quality
		Latency:     "normal",
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 3. Execute with Retry
	return p.executeWithRetry(ctx, jsonData, text, outputPath)
}

func (p *Provider) executeWithRetry(ctx context.Context, jsonData []byte, text, outputPath string) (string, error) {
	maxRetries := 2 // Total 3 attempts
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Delay between retries
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(500 * time.Millisecond):
				tts.Log("FISH", fmt.Sprintf("Retrying request (attempt %d/%d)...", attempt+1, maxRetries+1), 0, lastErr)
			}
		}

		ext, retry, err := p.executeAttempt(ctx, jsonData, text, outputPath)
		if err == nil {
			// Success!
			if p.tracker != nil {
				p.tracker.TrackAPISuccess("fish-audio")
			}
			return ext, nil
		}

		if !retry {
			return "", err // Fatal error
		}

		lastErr = err
	}

	// All retries failed
	if p.tracker != nil {
		p.tracker.TrackAPIFailure("fish-audio")
	}

	// Wrap in FatalError if it wasn't already, to trigger fallback if appropriate
	return "", tts.NewFatalError(500, fmt.Sprintf("Fish Audio failed after %d attempts: %v", maxRetries+1, lastErr))
}

func (p *Provider) executeAttempt(ctx context.Context, jsonData []byte, text, outputPath string) (ext string, retry bool, err error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Debug info
	var headerLog strings.Builder
	for k, v := range req.Header {
		headerLog.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
	}
	logContent := fmt.Sprintf("HEADERS:\n%s\nPAYLOAD:\n%s", headerLog.String(), text)

	resp, err := p.client.Do(req)
	if err != nil {
		tts.Log("FISH", logContent, 0, err)
		return "", true, err // Retry on network error
	}

	// Check Status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		tts.Log("FISH", logContent, resp.StatusCode, nil)

		// Fast Fail on Auth Errors
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", false, tts.NewFatalError(resp.StatusCode, fmt.Sprintf("Fish Audio Auth Failed: %s", string(body)))
		}

		return "", true, fmt.Errorf("fish audio api error (status %d): %s", resp.StatusCode, string(body))
	}

	// Save Output
	ext = "mp3"
	filename := outputPath
	if filepath.Ext(filename) != "."+ext {
		filename = filename + "." + ext
	}

	f, err := os.Create(filename)
	if err != nil {
		resp.Body.Close()
		return "", false, fmt.Errorf("failed to create output file: %w", err)
	}

	// Copy and check size
	written, err := io.Copy(f, resp.Body)
	resp.Body.Close()
	f.Close() // Close to flush

	if err != nil {
		tts.Log("FISH", logContent, 200, err)
		os.Remove(filename)
		return "", true, fmt.Errorf("failed to write audio to file: %w", err)
	}

	if written == 0 {
		tts.Log("FISH", "Received empty audio file (0 bytes)", 200, nil)
		os.Remove(filename)
		return "", true, fmt.Errorf("received empty audio from fish audio")
	}

	tts.Log("FISH", logContent, 200, nil)
	return ext, false, nil
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
