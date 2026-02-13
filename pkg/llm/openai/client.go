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
	"phileasgo/pkg/llm/imageutil"
	"phileasgo/pkg/request"
)

// Client implements llm.Provider for any OpenAI-compatible API.
type Client struct {
	rc       *request.Client
	apiKey   string
	baseURL  string
	profiles map[string]string

	// Temperature settings
	temperatureBase   float32
	temperatureJitter float32

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

// NewClient creates a new OpenAI client.
func NewClient(cfg config.ProviderConfig, defaultBaseURL string, rc *request.Client) (*Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	return &Client{
		baseURL:           strings.TrimSuffix(baseURL, "/"),
		apiKey:            cfg.Key,
		profiles:          cfg.Profiles,
		rc:                rc,
		temperatureBase:   1.0,
		temperatureJitter: 0.3,
	}, nil
}

// ValidateModels checks if the configured models are available.
func (c *Client) ValidateModels(ctx context.Context) error {
	if os.Getenv("TEST_MODE") == "true" {
		slog.Warn("Skipping OpenAI model validation (TEST_MODE=true)")
		return nil
	}
	if len(c.profiles) == 0 {
		return nil
	}

	// OpenAI-compatible /models endpoint
	// We assume baseURL is the root (e.g. https://api.openai.com/v1)
	// If it's the full chat/completions URL, this will fail, which is intended
	// as we want to encourage using the root URL.
	u := c.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}

	respBody, err := c.rc.GetWithHeaders(ctx, u, headers, "")
	if err != nil {
		return fmt.Errorf("failed to fetch models from %s: %w", u, err)
	}

	var mresp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &mresp); err != nil {
		return fmt.Errorf("failed to parse models response: %w", err)
	}

	available := make(map[string]bool)
	var availableList []string
	for _, m := range mresp.Data {
		available[m.ID] = true
		availableList = append(availableList, m.ID)
	}

	var missing []string
	for _, model := range c.profiles {
		if !available[model] {
			missing = append(missing, model)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("configured models %v not found at %s. Available models: %v", missing, u, availableList)
	}

	return nil
}

func (c *Client) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	model, err := c.resolveModel(name)
	if err != nil {
		return "", err
	}

	var temp float32 = 0.7
	if isReasoner(model) {
		temp = 1.0
	}

	req := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: temp,
	}

	return c.execute(ctx, req)
}

func (c *Client) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	model, err := c.resolveModel(name)
	if err != nil {
		return err
	}

	// DeepSeek (and others) require "json" in the prompt for json_object mode
	if !strings.Contains(strings.ToLower(prompt), "json") {
		prompt += " Respond in JSON."
	}

	var temp float32 = 0.1
	var respFmt *responseFormat = &responseFormat{Type: "json_object"}

	if isReasoner(model) {
		temp = 1.0
		respFmt = nil // Reasoner/R1 doesn't support json_object mode
	}

	req := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
		ResponseFormat: respFmt,
		Temperature:    temp,
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
	model, err := c.resolveModel(name)
	if err != nil {
		return "", err
	}

	data, mimeType, err := imageutil.PrepareForLLM(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to prepare image: %w", err)
	}

	b64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64Data)

	var temp float32 = 0.7
	if isReasoner(model) {
		temp = 1.0
	}

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
		Temperature: temp,
	}

	return c.execute(ctx, req)
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

	u := c.baseURL + "/chat/completions"
	respBody, err := c.rc.PostWithHeaders(ctx, u, body, headers)
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

func (c *Client) HasProfile(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.profiles[name]
	return ok && c.profiles[name] != ""
}

func (c *Client) resolveModel(intent string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if model, ok := c.profiles[intent]; ok && model != "" {
		return model, nil
	}
	return "", fmt.Errorf("profile %q not configured", intent)
}

func isReasoner(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "reasoner") || strings.Contains(m, "r1")
}
