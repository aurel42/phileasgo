package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
)

// Client implements llm.Provider for any OpenAI-compatible API.
type Client struct {
	rc        *request.Client
	apiKey    string
	modelName string
	baseURL   string
	profiles  map[string]string

	mu sync.RWMutex
}

// openaiRequest follows the standard Chat Completions format.
type openaiRequest struct {
	Model          string          `json:"model"`
	Messages       []openaiMessage `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    float32         `json:"temperature,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Can be string or []contentPart
}

type contentPart struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	ImageURL *imageURLContent `json:"image_url,omitempty"`
}

type imageURLContent struct {
	URL string `json:"url"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// openaiResponse follows the standard Chat Completions response format.
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// NewClient creates a new OpenAI-compatible client.
func NewClient(cfg config.ProviderConfig, baseURL string, rc *request.Client) (*Client, error) {
	c := &Client{
		rc:        rc,
		apiKey:    cfg.Key,
		modelName: cfg.Model,
		baseURL:   baseURL,
		profiles:  cfg.Profiles,
	}

	if c.baseURL == "" {
		return nil, fmt.Errorf("openai-compatible client requires a baseURL")
	}

	// Validate Model Availability
	if c.apiKey != "" {
		if err := c.validateModel(context.Background()); err != nil {
			if os.Getenv("TEST_MODE") == "true" {
				slog.Warn("OpenAI model validation failed (proceeding due to TEST_MODE)", "error", err)
			} else {
				return nil, fmt.Errorf("model validation failed: %w", err)
			}
		}
	}

	return c, nil
}

func (c *Client) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	model, _ := c.resolveModel(name)

	req := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.7,
	}

	return c.execute(ctx, req)
}

func (c *Client) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	model, _ := c.resolveModel(name)

	req := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
		Temperature:    0.1,
	}

	respText, err := c.execute(ctx, req)
	if err != nil {
		return err
	}

	respText = llm.CleanJSONBlock(respText)

	if err := json.Unmarshal([]byte(respText), target); err != nil {
		return fmt.Errorf("failed to unmarshal openai json: %w (raw: %s)", err, respText)
	}

	return nil
}

func (c *Client) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	model, _ := c.resolveModel(name)

	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
		mimeType = "image/png"
	}

	b64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64Data)

	req := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "text", Text: prompt},
					{Type: "image_url", ImageURL: &imageURLContent{URL: dataURL}},
				},
			},
		},
		Temperature: 0.7,
	}

	return c.execute(ctx, req)
}

func (c *Client) HealthCheck(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("api key not configured")
	}
	// Simple text generation call as health check
	_, err := c.GenerateText(ctx, "health", "ping")
	return err
}

func (c *Client) Close() {}

func (c *Client) execute(ctx context.Context, oreq openaiRequest) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("api key is missing")
	}

	body, err := json.Marshal(oreq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
		"Content-Type":  "application/json",
	}

	respBody, err := c.rc.PostWithHeaders(ctx, c.baseURL, body, headers)
	if err != nil {
		return "", err
	}

	var oresp openaiResponse
	if err := json.Unmarshal(respBody, &oresp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if oresp.Error != nil {
		return "", fmt.Errorf("openai api error: %s (%s)", oresp.Error.Message, oresp.Error.Type)
	}

	if len(oresp.Choices) == 0 {
		return "", fmt.Errorf("api returned no choices")
	}

	return oresp.Choices[0].Message.Content, nil
}

func (c *Client) resolveModel(intent string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if model, ok := c.profiles[intent]; ok && model != "" {
		return model, true
	}
	return c.modelName, false
}

func (c *Client) validateModel(ctx context.Context) error {
	if c.modelName == "" {
		return fmt.Errorf("primary model name is required")
	}
	return nil
}
