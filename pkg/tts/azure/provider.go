package azure

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"phileasgo/pkg/config"
	"phileasgo/pkg/tts"
)

// Provider implements tts.Provider for Azure Speech.
type Provider struct {
	key     string
	region  string
	voiceID string
	client  *http.Client
	url     string
}

// NewProvider creates a new Azure Speech TTS provider.
func NewProvider(cfg config.AzureSpeechConfig) *Provider {
	url := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", cfg.Region)
	return &Provider{
		key:     cfg.Key,
		region:  cfg.Region,
		voiceID: cfg.VoiceID,
		client:  &http.Client{},
		url:     url,
	}
}

// Synthesize generates speech from text using Azure Speech.
func (p *Provider) Synthesize(ctx context.Context, text, voiceID, outputPath string) (string, error) {
	// 1. Determine Voice ID
	vid := p.voiceID
	if voiceID != "" {
		vid = voiceID
	}
	if vid == "" {
		return "", fmt.Errorf("no voice ID configured for Azure Speech")
	}

	// 2. Build SSML
	// We use "narration-friendly" style as requested.
	// We do NOT escape XML here because the LLM is instructed to produce valid SSML/XML-safe text.
	// UPDATE: Removed mstts:express-as because it interferes with nested <lang> tags for multilingual voices.
	// See: https://learn.microsoft.com/en-us/azure/ai-services/speech-service/speech-synthesis-markup-voice#speaking-styles
	ssml := fmt.Sprintf(
		`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='https://www.w3.org/2001/mstts' xml:lang='en-US'><voice name='%s'>%s</voice></speak>`,
		vid, text,
	)

	// 3. Validate SSML (catch LLM errors)
	// 3. Validate SSML (catch LLM errors)
	if err := validateSSML(ssml); err != nil {
		// Fallback: escape text and rebuild minimal SSML
		escapedText := escapeXML(text)
		ssml = fmt.Sprintf(
			`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='https://www.w3.org/2001/mstts' xml:lang='en-US'><voice name='%s'>%s</voice></speak>`,
			vid, escapedText,
		)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.url, bytes.NewBufferString(ssml))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Ocp-Apim-Subscription-Key", p.key)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-24khz-160kbitrate-mono-mp3")
	req.Header.Set("User-Agent", "PhileasGo")

	// 3. Execute Request
	resp, err := p.client.Do(req)
	if err != nil {
		tts.Log("AZURE", ssml, 0, err)
		return "", fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tts.Log("AZURE", ssml, resp.StatusCode, nil)
		body, err := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if err != nil {
			bodyStr = fmt.Sprintf("[failed to read body: %v]", err)
		}
		if bodyStr == "" {
			bodyStr = "[empty body]"
		}

		return "", fmt.Errorf("azure speech api error (status %d): %s", resp.StatusCode, bodyStr)
	}

	// 4. Save Output
	tts.Log("AZURE", ssml, 200, nil)
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

// Voices returns a list of available voices.
func (p *Provider) Voices(ctx context.Context) ([]tts.Voice, error) {
	return []tts.Voice{
		{
			ID:       p.voiceID,
			Name:     "Configured Azure Voice",
			Language: "en-US",
			IsNeural: true,
		},
	}, nil
}

// validateSSML checks if the SSML string is well-formed XML.
func validateSSML(ssml string) error {
	decoder := xml.NewDecoder(bytes.NewReader([]byte(ssml)))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// escapeXML escapes special XML characters in text.
func escapeXML(text string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(text)); err != nil {
		return text // fallback to original on error
	}
	return buf.String()
}
