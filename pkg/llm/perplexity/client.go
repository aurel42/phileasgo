package perplexity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
)

const baseURL = "https://api.perplexity.ai/chat/completions"

// Client implements llm.Provider for Perplexity Sonar API.
// Perplexity uses an OpenAI-compatible chat completions format.
type Client struct {
	rc       *request.Client
	apiKey   string
	profiles map[string]string

	mu sync.RWMutex
}

// sonarRequest follows the OpenAI Chat Completions format that Perplexity accepts.
type sonarRequest struct {
	Model            string            `json:"model"`
	Messages         []sonarMessage    `json:"messages"`
	WebSearchOptions *webSearchOptions `json:"web_search_options,omitempty"`
}

// webSearchOptions controls Perplexity's web search behavior.
type webSearchOptions struct {
	// SearchContextSize: "low", "medium", or "high"
	// "high" maximizes web retrieval for comprehensive answers (used for pregrounding)
	SearchContextSize string `json:"search_context_size,omitempty"`
}

type sonarMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// sonarResponse matches the Perplexity API response format.
type sonarResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	// Perplexity includes citations in the response
	Citations []string `json:"citations,omitempty"`
	Error     *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// NewClient creates a new Perplexity Sonar client.
func NewClient(cfg config.ProviderConfig, rc *request.Client) (*Client, error) {
	if cfg.Key == "" {
		return nil, fmt.Errorf("perplexity api key is required")
	}

	return &Client{
		apiKey:   cfg.Key,
		profiles: cfg.Profiles,
		rc:       rc,
	}, nil
}

func (c *Client) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	model, err := c.resolveModel(name)
	if err != nil {
		return "", err
	}

	req := sonarRequest{
		Model: model,
		Messages: []sonarMessage{
			{Role: "user", Content: prompt},
		},
		WebSearchOptions: &webSearchOptions{SearchContextSize: "high"},
	}

	return c.execute(ctx, req)
}

func (c *Client) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	model, err := c.resolveModel(name)
	if err != nil {
		return err
	}

	// Perplexity doesn't have a native JSON mode, so we instruct via prompt
	jsonPrompt := prompt + "\n\nRespond with valid JSON only, no markdown."

	req := sonarRequest{
		Model: model,
		Messages: []sonarMessage{
			{Role: "user", Content: jsonPrompt},
		},
		WebSearchOptions: nil, // JSON tasks don't need web search
	}

	respText, err := c.execute(ctx, req)
	if err != nil {
		return err
	}

	respText = llm.CleanJSONBlock(respText)

	if err := json.Unmarshal([]byte(respText), target); err != nil {
		return fmt.Errorf("failed to unmarshal perplexity json: %w (raw: %s)", err, respText)
	}

	return nil
}

// ValidateModels checks if the configured models are available.
func (c *Client) ValidateModels(ctx context.Context) error {
	// Perplexity's /models endpoint appears to be unreliable or non-standard (returning 404).
	// We disable validation for this provider to prevent startup failures.
	slog.Debug("Skipping Perplexity model validation (disabled)")
	return nil
}

// GenerateImageText is not supported by Perplexity Sonar (text-only models).
func (c *Client) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	return "", fmt.Errorf("perplexity sonar does not support image input")
}

func (c *Client) HealthCheck(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("api key not configured")
	}

	c.mu.RLock()
	var testProfile string
	for p := range c.profiles {
		testProfile = p
		break
	}
	c.mu.RUnlock()

	if testProfile == "" {
		return fmt.Errorf("no profiles configured")
	}

	// Quick ping to validate API access
	_, err := c.GenerateText(ctx, testProfile, "ping")
	return err
}

func (c *Client) HasProfile(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	model, ok := c.profiles[name]
	return ok && model != ""
}

func (c *Client) execute(ctx context.Context, sreq sonarRequest) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("api key is missing")
	}

	body, err := json.Marshal(sreq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
		"Content-Type":  "application/json",
	}

	respBody, err := c.rc.PostWithHeaders(ctx, baseURL, body, headers)
	if err != nil {
		return "", err
	}

	var sresp sonarResponse
	if err := json.Unmarshal(respBody, &sresp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if sresp.Error != nil {
		return "", fmt.Errorf("perplexity api error: %s (%s)", sresp.Error.Message, sresp.Error.Type)
	}

	if len(sresp.Choices) == 0 {
		return "", fmt.Errorf("perplexity api returned no choices")
	}

	return sresp.Choices[0].Message.Content, nil
}

func (c *Client) resolveModel(intent string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if model, ok := c.profiles[intent]; ok && model != "" {
		return model, nil
	}
	return "", fmt.Errorf("profile %q not configured for perplexity", intent)
}

// Close is a no-op for HTTP clients.
func (c *Client) Close() {}

// --- Extended API for Sonar-specific features ---

// SearchResult represents a web search grounded response with citations.
type SearchResult struct {
	Content   string
	Citations []string
}

// Search performs a grounded web search query and returns content with citations.
// This is the primary value-add of Perplexity over pure LLMs.
func (c *Client) Search(ctx context.Context, query string) (*SearchResult, error) {
	c.mu.RLock()
	// Use the first available profile/model
	var model string
	for _, m := range c.profiles {
		if m != "" {
			model = m
			break
		}
	}
	c.mu.RUnlock()

	if model == "" {
		return nil, fmt.Errorf("no model configured for search")
	}

	req := sonarRequest{
		Model: model,
		Messages: []sonarMessage{
			{Role: "user", Content: query},
		},
		WebSearchOptions: &webSearchOptions{SearchContextSize: "high"},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
		"Content-Type":  "application/json",
	}

	respBody, err := c.rc.PostWithHeaders(ctx, baseURL, body, headers)
	if err != nil {
		return nil, err
	}

	var sresp sonarResponse
	if err := json.Unmarshal(respBody, &sresp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if sresp.Error != nil {
		return nil, fmt.Errorf("perplexity api error: %s (%s)", sresp.Error.Message, sresp.Error.Type)
	}

	if len(sresp.Choices) == 0 {
		return nil, fmt.Errorf("perplexity api returned no choices")
	}

	return &SearchResult{
		Content:   strings.TrimSpace(sresp.Choices[0].Message.Content),
		Citations: sresp.Citations,
	}, nil
}
