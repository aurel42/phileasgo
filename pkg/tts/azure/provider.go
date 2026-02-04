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
	"regexp"

	"phileasgo/pkg/config"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
)

// Provider implements tts.Provider for Azure Speech.
type Provider struct {
	key      string
	region   string
	voiceID  string
	langProv tts.LanguageProvider
	client   *http.Client
	url      string
	tracker  *tracker.Tracker
}

// NewProvider creates a new Azure Speech TTS provider.
func NewProvider(cfg config.AzureSpeechConfig, langProv tts.LanguageProvider, t *tracker.Tracker) *Provider {
	url := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", cfg.Region)
	return &Provider{
		key:      cfg.Key,
		region:   cfg.Region,
		voiceID:  cfg.VoiceID,
		langProv: langProv,
		client:   &http.Client{},
		url:      url,
		tracker:  t,
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

	// 2. Build & Validate SSML
	ssml := p.buildSSML(ctx, vid, text)

	// 3. Execute Request

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
		if p.tracker != nil {
			p.tracker.TrackAPIFailure("azure-speech")
		}
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

		if p.tracker != nil {
			p.tracker.TrackAPIFailure("azure-speech")
		}

		// Return FatalError for 4xx/5xx to trigger fallback
		errMsg := fmt.Sprintf("azure speech api error (status %d): %s", resp.StatusCode, bodyStr)
		return "", tts.NewFatalError(resp.StatusCode, errMsg)
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
		if p.tracker != nil {
			p.tracker.TrackAPIFailure("azure-speech")
		}
		return "", fmt.Errorf("failed to write audio to file: %w", err)
	}

	if p.tracker != nil {
		p.tracker.TrackAPISuccess("azure-speech")
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

func (p *Provider) buildSSML(ctx context.Context, vid, text string) string {
	// 0. Pre-emptive Repair: Fix common hallucinations (e.g. xml:ID)
	text = repairSSML(text)

	// Get language dynamically from provider (allows runtime changes via config dialog)
	language := p.langProv.ActiveTargetLanguage(ctx)

	// We use "narration-friendly" style as requested.
	// We do NOT escape XML here because the LLM is instructed to produce valid SSML/XML-safe text.
	// UPDATE: Removed mstts:express-as because it interferes with nested <lang> tags for multilingual voices.
	// UPDATE: Workaround for word truncation (e.g. "Seepyramide" -> "Se").
	// Injecting punctuation inside the tag forces the model to complete the phonetic sequence.
	// We use a comma for a less invasive pause than a period.

	// Regex matches closing tag, capturing the content before it if it lacks punctuation.
	// However, complex regex replacement with Go's re2 is limited (no lookbehind).
	// Safer approach: Find `</lang>` and look backwards? Or Iterate?
	// Simplest robust method for now: Replace `</lang>` with `,</lang>`?
	// But we shouldn't add double punctuation if it ends with `.` or `,`.
	// Let's use ReplaceAllStringFunc.

	reLangEnd := regexp.MustCompile(`([^.?!,])</lang>`)
	processedText := reLangEnd.ReplaceAllString(text, `$1,</lang>`)

	ssml := fmt.Sprintf(
		`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='https://www.w3.org/2001/mstts' xml:lang='%s'><voice name='%s'>%s</voice></speak>`,
		language, vid, processedText,
	)

	// Validate SSML (catch LLM errors like malformed tags)
	if err := validateSSML(ssml); err != nil {
		// Fallback: Strip tags to prevent reading out "less than..." and just read plain text.
		cleanText := stripTags(text)
		return fmt.Sprintf(
			`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='https://www.w3.org/2001/mstts' xml:lang='%s'><voice name='%s'>%s</voice></speak>`,
			language, vid, cleanText,
		)
	}
	return ssml
}

// repairSSML attempts to fix common SSML malformations from LLMs.
func repairSSML(text string) string {
	// 1. Remove invalid "xml:ID" attributes which Gemini sometimes hallucinates.
	// Matches: xml:ID">, xml:ID="foo", xml:ID
	reID := regexp.MustCompile(`\s+xml:ID[^>]*`)
	text = reID.ReplaceAllString(text, "")

	// 2. Remove any <speak> or </speak> tags that the LLM may have generated.
	// These will be added by buildSSML, so duplicates cause Azure 400 errors.
	reSpeakOpen := regexp.MustCompile(`(?i)<speak[^>]*>`)
	reSpeakClose := regexp.MustCompile(`(?i)</speak>`)
	text = reSpeakOpen.ReplaceAllString(text, "")
	text = reSpeakClose.ReplaceAllString(text, "")

	// 3. Remove any <voice> or </voice> tags.
	reVoiceOpen := regexp.MustCompile(`(?i)<voice[^>]*>`)
	reVoiceClose := regexp.MustCompile(`(?i)</voice>`)
	text = reVoiceOpen.ReplaceAllString(text, "")
	text = reVoiceClose.ReplaceAllString(text, "")

	return text
}

// stripTags removes all XML/HTML tags from the text.
func stripTags(text string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(text, "")
}
