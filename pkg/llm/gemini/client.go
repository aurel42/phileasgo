package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/iterator"
	"google.golang.org/genai"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

// Client implements llm.Provider for Google Gemini.
type Client struct {
	genaiClient *genai.Client
	apiKey      string
	profiles    map[string]string // Map intent -> modelName
	rc          *request.Client
	tracker     *tracker.Tracker

	// Temperature settings for narration (base + jitter with bell curve)
	temperatureBase   float32
	temperatureJitter float32

	mu sync.RWMutex
}

// NewClient creates a new Gemini client.
func NewClient(cfg config.ProviderConfig, rc *request.Client, t *tracker.Tracker) (*Client, error) {
	c := &Client{
		rc:                rc,
		tracker:           t,
		apiKey:            cfg.Key,
		profiles:          cfg.Profiles,
		temperatureBase:   1.0, // Defaults
		temperatureJitter: 0.3,
	}

	if c.apiKey != "" {
		// Create new client
		client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey: c.apiKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create genai client: %w", err)
		}
		c.genaiClient = client

		// Validate Model Availability
		if err := c.ValidateModels(context.Background()); err != nil {
			if os.Getenv("TEST_MODE") == "true" {
				slog.Warn("Gemini model validation failed (proceeding due to TEST_MODE)", "error", err)
			} else {
				return nil, fmt.Errorf("gemini model validation failed: %w", err)
			}
		}
	}

	return c, nil
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

func (c *Client) getTemperature() *float32 {
	// Simple randomization within range
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	val := c.temperatureBase + (r.Float32()-0.5)*c.temperatureJitter
	if val < 0 {
		val = 0
	}
	if val > 1.0 {
		val = 1.0
	}
	return &val
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
	// Determine model based on intent/profile
	modelName, config, err := c.resolveModel(name)
	if err != nil {
		return "", err
	}

	// Create content part for prompt
	resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), config)
	if err != nil {
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
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", err
	}

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
	// Determine model based on intent/profile
	modelName, config, err := c.resolveModel(name)
	if err != nil {
		return fmt.Errorf("gemini resolve model error: %w", err)
	}
	config.ResponseMIMEType = "application/json"

	resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), config)
	if err != nil {
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return fmt.Errorf("generate json error: %w", err)
	}

	text, err := getResponseText(resp)
	if err != nil {
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return err
	}

	// Sanitize Markdown JSON blocks if present
	cleaned := llm.CleanJSONBlock(text)

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

	slog.Debug("Gemini: Attaching image to multimodal request",
		"path", imagePath,
		"size_bytes", len(imgData),
		"mime", mimeType)

	// Determine model based on intent/profile
	// Determine model based on intent/profile
	modelName, config, err := c.resolveModel(name)
	if err != nil {
		return "", err
	}

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
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", fmt.Errorf("generate image text error: %w", err)
	}

	text, err := getResponseText(resp)
	if err != nil {
		if c.tracker != nil {
			c.tracker.TrackAPIFailure("gemini")
		}
		return "", err
	}

	if c.tracker != nil {
		c.tracker.TrackAPISuccess("gemini")
	}

	return text, nil
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

// resolveModel determines the model name and generation config based on the intent.
// resolveModel determines the model name and generation config based on the intent.
func (c *Client) resolveModel(intent string) (string, *genai.GenerateContentConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	model, ok := c.profiles[intent]
	if !ok || model == "" {
		return "", nil, fmt.Errorf("no model configured for intent %q", intent)
	}

	cfg := &genai.GenerateContentConfig{
		Temperature: c.getTemperature(),
	}
	return model, cfg, nil
}

// ValidateModels checks if the configured models are available.
func (c *Client) ValidateModels(ctx context.Context) error {
	if os.Getenv("TEST_MODE") == "true" {
		slog.Warn("Skipping Gemini model validation (TEST_MODE=true)")
		return nil
	}
	if len(c.profiles) == 0 {
		return fmt.Errorf("no profiles configured for gemini provider")
	}

	modelsToCheck := make(map[string]bool)
	for _, m := range c.profiles {
		modelsToCheck[m] = true
	}

	var missingModels []string
	for model := range modelsToCheck {
		name := model
		if !strings.HasPrefix(name, "models/") {
			name = "models/" + name
		}
		_, err := c.genaiClient.Models.Get(ctx, name, nil)
		if err != nil {
			missingModels = append(missingModels, model)
		}
	}

	if len(missingModels) == 0 {
		return nil
	}

	// Fetch available models for the user
	iter, listErr := c.genaiClient.Models.List(ctx, nil)
	var availableInfo string
	if listErr == nil {
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
		if len(availableModels) > 0 {
			availableInfo = "\nAvailable models for this key: " + strings.Join(availableModels, ", ")
		}
	}

	return fmt.Errorf("configured models %v not found or unauthorized.%s", missingModels, availableInfo)
}

// HasProfile checks if the provider has a specific profile configured.
func (c *Client) HasProfile(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.profiles[name]
	return ok && c.profiles[name] != ""
}
