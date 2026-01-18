package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/iterator"
	"google.golang.org/genai"

	"phileasgo/pkg/config"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

// Client implements llm.Provider for Google Gemini.
type Client struct {
	genaiClient *genai.Client
	apiKey      string
	modelName   string
	profiles    map[string]string // Map intent -> modelName
	rc          *request.Client
	tracker     *tracker.Tracker
	logPath     string

	// Temperature settings for narration (base + jitter with bell curve)
	temperatureBase   float32
	temperatureJitter float32

	mu sync.RWMutex
}

// NewClient creates a new Gemini client.
func NewClient(cfg config.LLMConfig, logPath string, rc *request.Client, t *tracker.Tracker) (*Client, error) {
	c := &Client{rc: rc, tracker: t, logPath: logPath}
	if err := c.Configure(cfg); err != nil {
		return nil, err
	}
	return c, nil
}

// Configure updates the client with new settings.
func (c *Client) Configure(cfg config.LLMConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.apiKey = cfg.Key
	c.modelName = cfg.Model
	c.profiles = cfg.Profiles

	if c.modelName == "" {
		c.modelName = "gemini-2.0-flash"
	}

	if c.apiKey == "" {
		// Can't initialize without key.
		c.genaiClient = nil
		return nil
	}

	// Create new client
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: c.apiKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create genai client: %w", err)
	}
	c.genaiClient = client

	// Validate Model Availability
	if err := c.validateModel(context.Background()); err != nil {
		slog.Warn("Gemini model validation failed (proceeding anyway)", "error", err)
		// We do NOT return error here, to allow startup even if API is flaky/rate-limited.
		// If the key/model is truly invalid, actual generation calls will fail later.
	}

	return nil
}

// Close cleans up resources.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.genaiClient = nil
}

// SetTemperature configures temperature settings for narration prompts.
// Uses base + jitter with bell curve (normal distribution).
func (c *Client) SetTemperature(base, jitter float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.temperatureBase = base
	c.temperatureJitter = jitter
}

// GenerateText sends a prompt and returns the text response.
func (c *Client) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	c.mu.RLock()
	client := c.genaiClient
	c.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("gemini client not configured")
	}

	// Determine model based on intent/profile
	modelName, config := c.resolveModel(name)

	// Create content part for prompt
	resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), config)

	if err != nil {
		c.logPrompt(name, prompt, fmt.Sprintf("ERROR: %v", err))
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", fmt.Errorf("generate text error: %w", err)
	}

	// Check for Search/Grounding Metadata
	if len(resp.Candidates) > 0 {
		logGoogleSearchUsage(name, resp.Candidates[0].GroundingMetadata)
	}

	text, err := getResponseText(resp)
	if err != nil {
		c.logPrompt(name, prompt, fmt.Sprintf("TEXT_PARSE_ERROR: %v", err))
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", err
	}

	c.logPrompt(name, prompt, text)
	if c.tracker != nil {
		c.tracker.TrackAPISuccess("gemini")
	}
	return text, nil
}

// GenerateJSON sends a prompt and unmarshals the response into the target struct.
func (c *Client) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	c.mu.RLock()
	client := c.genaiClient
	c.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("gemini client not configured")
	}

	// Determine model based on intent/profile
	modelName, config := c.resolveModel(name)
	config.ResponseMIMEType = "application/json"

	resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), config)
	if err != nil {
		c.logPrompt(name, prompt, fmt.Sprintf("ERROR: %v", err))
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return fmt.Errorf("generate json error: %w", err)
	}

	text, err := getResponseText(resp)
	if err != nil {
		c.logPrompt(name, prompt, fmt.Sprintf("TEXT_PARSE_ERROR: %v", err))
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return err
	}

	// Sanitize Markdown JSON blocks if present
	cleaned := cleanJSONBlock(text)
	c.logPrompt(name, prompt, cleaned)

	if err := json.Unmarshal([]byte(cleaned), target); err != nil {
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return fmt.Errorf("failed to unmarshal JSON response: %w. Response: %s", err, cleaned)
	}

	if c.tracker != nil {
		c.tracker.TrackAPISuccess("gemini")
	}
	return nil

}

// GenerateImageText sends a prompt + image and returns the text response.
func (c *Client) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	c.mu.RLock()
	client := c.genaiClient
	c.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("gemini client not configured")
	}

	// Read image file
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	// Determine MIME type (simple heuristic)
	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
		mimeType = "image/png"
	}

	// Determine model based on intent/profile
	modelName, config := c.resolveModel(name)

	// Combine Text + Image manually since genai.ImageData helper might not exist
	// We assume genai.Part has Text and InlineData fields.
	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{
			MIMEType: mimeType,
			Data:     imgData,
		}},
	}

	contents := []*genai.Content{
		{Parts: parts},
	}

	resp, err := client.Models.GenerateContent(ctx, modelName, contents, config)
	if err != nil {
		c.logPrompt(name, prompt+" [IMAGE: "+imagePath+"]", fmt.Sprintf("ERROR: %v", err))
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", fmt.Errorf("generate image text error: %w", err)
	}

	text, err := getResponseText(resp)
	if err != nil {
		c.logPrompt(name, prompt+" [IMAGE: "+imagePath+"]", fmt.Sprintf("TEXT_PARSE_ERROR: %v", err))
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", err
	}

	c.logPrompt(name, prompt+" [IMAGE: "+imagePath+"]", text)
	if c.tracker != nil {
		c.tracker.TrackAPISuccess("gemini")
	}
	return text, nil
}

func (c *Client) logPrompt(name, prompt, response string) {
	if c.logPath == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(c.logPath), 0o755); err != nil {
		return
	}

	f, err := os.OpenFile(c.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	wrappedResponse := wordWrap(response, 80)
	truncatedPrompt := truncateParagraphs(prompt, 80) // Truncate WP article paragraphs
	entry := fmt.Sprintf("[%s] PROMPT: %s\nPROMPT_TEXT:\n%s\n\nRESPONSE:\n%s\n%s\n",
		timestamp, name, truncatedPrompt, wrappedResponse, strings.Repeat("-", 80))

	_, _ = f.WriteString(entry)
}

func getResponseText(resp *genai.GenerateContentResponse) (string, error) {
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned")
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
	}
	return sb.String(), nil
}

func cleanJSONBlock(text string) string {
	text = strings.TrimSpace(text)

	// Look for ```json start
	start := strings.Index(text, "```json")
	if start != -1 {
		text = text[start+len("```json"):]
		// Find end of block
		end := strings.LastIndex(text, "```")
		if end != -1 {
			text = text[:end]
		}
		return strings.TrimSpace(text)
	}

	// Look for generic ``` start
	start = strings.Index(text, "```")
	if start != -1 {
		text = text[start+len("```"):]
		// Find end of block
		end := strings.LastIndex(text, "```")
		if end != -1 {
			text = text[:end]
		}
		return strings.TrimSpace(text)
	}

	return text
}

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		currentLineLength := 0
		for j, word := range words {
			if j > 0 {
				if currentLineLength+len(word)+1 > width {
					result.WriteString("\n")
					currentLineLength = 0
				} else {
					result.WriteString(" ")
					currentLineLength++
				}
			}
			result.WriteString(word)
			currentLineLength += len(word)
		}
	}
	return result.String()
}

// truncateParagraphs truncates each line at maxLen characters and filters empty lines.
// Used for logging prompts (WP articles) to gemini.log.
// truncateParagraphs truncates lines within the Wikipedia article block to maxLen
// and removes empty lines within that block. The rest of the prompt is preserved as-is.
func truncateParagraphs(text string, maxLen int) string {
	lines := strings.Split(text, "\n")
	var result []string
	inWikiBlock := false

	for _, line := range lines {
		// Detect block boundaries (case-insensitive just in case)
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "<start of wikipedia article>") {
			inWikiBlock = true
			result = append(result, line)
			continue
		}
		if strings.Contains(lowerLine, "<end of wikipedia article>") {
			inWikiBlock = false
			result = append(result, line)
			continue
		}

		if inWikiBlock {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue // Filter empty lines inside wiki block
			}
			runes := []rune(trimmed)
			if len(runes) > maxLen {
				trimmed = string(runes[:maxLen]) + "..."
			}
			result = append(result, trimmed)
		} else {
			// Outside wiki block: preserve line EXACTLY as is (including empty lines)
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// validateModel checks if the configured model is available for the API key.
func (c *Client) validateModel(ctx context.Context) error {
	// Ensure model name has 'models/' prefix
	name := c.modelName
	if !strings.HasPrefix(name, "models/") {
		name = "models/" + name
	}

	// Try to get the specific model (1 API call)
	_, err := c.genaiClient.Models.Get(ctx, name, nil)
	if err == nil {
		slog.Debug("Gemini model validation success", "model", c.modelName)
		return nil
	}

	slog.Warn("Gemini model validation failed, fetching available models...", "model", c.modelName, "error", err)

	// Fetch available models for recovery
	iter, listErr := c.genaiClient.Models.List(ctx, nil)
	if listErr != nil {
		slog.Warn("Failed to list models for recovery", "error", listErr)
		return nil // Proceed anyway
	}

	var availableModels []string
	for {
		resp, nextErr := iter.Next(ctx)
		if nextErr == iterator.Done {
			break
		}
		if nextErr != nil {
			break
		}
		if strings.Contains(strings.ToLower(resp.Name), "gemini") {
			availableModels = append(availableModels, resp.Name)
		}
	}

	slog.Error("Configured model not found", "configured", c.modelName)
	slog.Error("Available 'gemini' models for this key:")
	for _, m := range availableModels {
		slog.Error("- " + m)
	}

	return nil // Proceed anyway (lazy validation on generation)
}

// HealthCheck verifies that the provider is configured and reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if os.Getenv("TEST_MODE") == "true" {
		return nil
	}

	if c.genaiClient == nil {
		return fmt.Errorf("gemini client not initialized (missing API key?)")
	}

	// Verify that the specifically configured model is available.
	// This confirms API key validity, network connectivity, and model access permissions.
	name := c.modelName
	if !strings.HasPrefix(name, "models/") {
		name = "models/" + name
	}

	_, err := c.genaiClient.Models.Get(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("configured model %q unavailable or API unreachable: %w", c.modelName, err)
	}

	return nil
}
