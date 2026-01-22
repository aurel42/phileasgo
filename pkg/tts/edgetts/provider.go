package edgetts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
)

// Provider implements tts.Provider for Microsoft Edge TTS.
type Provider struct {
	tracker *tracker.Tracker
}

// NewProvider creates a new Edge TTS provider.
func NewProvider(t *tracker.Tracker) *Provider {
	return &Provider{tracker: t}
}

// Synthesize generates an .mp3 file using Edge TTS.
func (p *Provider) Synthesize(ctx context.Context, text, voice, outputPath string) (string, error) {
	if voice == "" {
		return "", fmt.Errorf("voice ID is required")
	}

	text = tts.StripSpeakerLabels(text)

	fullPath := outputPath
	if !strings.HasSuffix(strings.ToLower(fullPath), ".mp3") {
		fullPath += ".mp3"
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	conn, err := p.dial(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := p.sendConfig(conn); err != nil {
		return "", err
	}

	requestID := strings.ReplaceAll(uuid.New().String(), "-", "")
	if err := p.sendSSML(conn, voice, text, requestID); err != nil {
		return "", err
	}

	if err := p.consumeResponses(ctx, conn, file); err != nil {
		if p.tracker != nil {
			p.tracker.TrackAPIFailure("edge-tts")
		}
		return "", err
	}

	if p.tracker != nil {
		p.tracker.TrackAPISuccess("edge-tts")
	}
	return "mp3", nil
}

func (p *Provider) dial(ctx context.Context) (*websocket.Conn, error) {
	edgeOrigin := os.Getenv("EDGE_TTS_ORIGIN")
	if edgeOrigin == "" {
		return nil, fmt.Errorf("EDGE_TTS_ORIGIN environment variable is required")
	}

	header := http.Header{}
	header.Set("Origin", edgeOrigin)
	header.Set("Pragma", "no-cache")
	header.Set("Cache-Control", "no-cache")

	// User-Agent from env (required)
	userAgent := os.Getenv("EDGE_TTS_USER_AGENT")
	if userAgent == "" {
		return nil, fmt.Errorf("EDGE_TTS_USER_AGENT environment variable is required")
	}
	header.Set("User-Agent", userAgent)
	header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	header.Set("Accept-Language", "en-US,en;q=0.9")

	// MUID Cookie
	muid := strings.ReplaceAll(uuid.New().String(), "-", "")
	header.Set("Cookie", fmt.Sprintf("muid=%s", muid))

	// Sec-MS-GEC Generation
	trustedClientToken := os.Getenv("EDGE_TTS_TRUSTED_CLIENT_TOKEN")
	if trustedClientToken == "" {
		return nil, fmt.Errorf("EDGE_TTS_TRUSTED_CLIENT_TOKEN environment variable is required")
	}
	token := p.generateSecMSGec(trustedClientToken)
	version := os.Getenv("EDGE_TTS_SEC_MS_GEC_VERSION")
	if version == "" {
		return nil, fmt.Errorf("EDGE_TTS_SEC_MS_GEC_VERSION environment variable is required")
	}

	edgeBaseURL := os.Getenv("EDGE_TTS_BASE_URL")
	if edgeBaseURL == "" {
		return nil, fmt.Errorf("EDGE_TTS_BASE_URL environment variable is required")
	}

	url := fmt.Sprintf("%s?TrustedClientToken=%s&Sec-MS-GEC=%s&Sec-MS-GEC-Version=%s",
		edgeBaseURL, trustedClientToken, token, version)

	var conn *websocket.Conn
	var dialErr error
	for i := 0; i < 3; i++ {
		var resp *http.Response
		conn, resp, dialErr = websocket.DefaultDialer.DialContext(ctx, url, header)
		if dialErr == nil {
			return conn, nil
		}
		if resp != nil {
			slog.Warn("EdgeTTS: specific handshake failure", "status", resp.Status, "status_code", resp.StatusCode)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("websocket dial failed after retries: %w", dialErr)
}

func (p *Provider) generateSecMSGec(trustedClientToken string) string {
	// Logic from python's drm.py:
	// ticks = unix_timestamp + 11644473600
	// ticks -= ticks % 300
	// ticks *= 1e7

	nowSec := float64(time.Now().Unix())
	// NOTE: Python uses dt.now(tz.utc).timestamp(). If local time is significantly off, we might fail.

	ticks := nowSec + 11644473600
	ticks -= float64(int64(ticks) % 300)
	ticks *= 1e7

	strToHash := fmt.Sprintf("%.0f%s", ticks, trustedClientToken)

	hash := sha256.Sum256([]byte(strToHash))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

func (p *Provider) sendConfig(conn *websocket.Conn) error {
	configMsg := "Content-Type:application/json; charset=utf-8\r\nPath:speech.config\r\n\r\n{\"context\":{\"synthesis\":{\"audio\":{\"metadataoptions\":{\"sentenceBoundaryEnabled\":\"false\",\"wordBoundaryEnabled\":\"false\"},\"outputFormat\":\"audio-24khz-48kbitrate-mono-mp3\"}}}}"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(configMsg)); err != nil {
		return fmt.Errorf("failed to send speech.config: %w", err)
	}
	return nil
}

func (p *Provider) sendSSML(conn *websocket.Conn, voice, text, requestID string) error {
	ssml := buildSSML(voice, text)
	tts.Log("EDGETTS", ssml, 0, nil)

	ssmlMsg := fmt.Sprintf("X-RequestId:%s\r\nContent-Type:application/ssml+xml\r\nPath:ssml\r\n\r\n%s", requestID, ssml)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(ssmlMsg)); err != nil {
		return fmt.Errorf("failed to send ssml: %w", err)
	}
	return nil
}

func buildSSML(voice, text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	escapedText := replacer.Replace(text)
	return fmt.Sprintf("<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'><voice name='%s'>%s</voice></speak>", voice, escapedText)
}

func (p *Provider) consumeResponses(ctx context.Context, conn *websocket.Conn, file *os.File) error {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message failed: %w", err)
		}

		if msgType == websocket.TextMessage {
			if strings.Contains(string(data), "Path:turn.end") {
				return nil
			}
		} else if msgType == websocket.BinaryMessage {
			if err := p.handleBinaryMessage(data, file); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (p *Provider) handleBinaryMessage(data []byte, file *os.File) error {
	if len(data) < 2 {
		return nil
	}
	headerLength := int(uint16(data[0])<<8 | uint16(data[1]))
	if len(data) < 2+headerLength {
		return nil
	}
	audioData := data[2+headerLength:]
	if len(audioData) > 0 {
		if _, err := file.Write(audioData); err != nil {
			return fmt.Errorf("write audio data failed: %w", err)
		}
	}
	return nil
}

// Voices returns a list of high-quality neural voices.
func (p *Provider) Voices(ctx context.Context) ([]tts.Voice, error) {
	return []tts.Voice{
		{ID: "en-US-AvaMultilingualNeural", Name: "Ava (Multilingual)", Language: "en-US", IsNeural: true},
		{ID: "en-US-AndrewMultilingualNeural", Name: "Andrew (Multilingual)", Language: "en-US", IsNeural: true},
		{ID: "en-GB-SoniaNeural", Name: "Sonia (UK)", Language: "en-GB", IsNeural: true},
		{ID: "fr-FR-VivienneNeural", Name: "Vivienne (France)", Language: "fr-FR", IsNeural: true},
		{ID: "de-DE-SeraphinaNeural", Name: "Seraphina (Germany)", Language: "de-DE", IsNeural: true},
	}, nil
}
