package edgetts

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
)

const (
	edgeBaseURL = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	edgeOrigin  = "chrome-extension://jdmojkciocjibebeunonbhnbn"
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
		voice = "en-US-AvaMultilingualNeural"
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
	header := http.Header{}
	header.Set("Origin", edgeOrigin)
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	var conn *websocket.Conn
	var dialErr error
	for i := 0; i < 3; i++ {
		conn, _, dialErr = websocket.DefaultDialer.DialContext(ctx, edgeBaseURL, header)
		if dialErr == nil {
			return conn, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("websocket dial failed after retries: %w", dialErr)
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
