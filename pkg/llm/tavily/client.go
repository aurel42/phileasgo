package tavily

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
)

const defaultBaseURL = "https://api.tavily.com/search"

// Client implements llm.Provider for the Tavily Search API.
// It is primarily used for grounding/pregrounding by using the 'include_answer' feature.
type Client struct {
	rc       *request.Client
	apiKey   string
	baseURL  string
	profiles map[string]string
	label    string

	mu sync.RWMutex
}

// searchRequest represents the Tavily Search API request payload.
type searchRequest struct {
	Query       string `json:"query"`
	SearchDepth string `json:"search_depth,omitempty"`
	// IncludeAnswer is 'any' because it accepts bool (true/false) or string ("basic"/"advanced").
	IncludeAnswer any  `json:"include_answer,omitempty"`
	IncludeImages bool `json:"include_images,omitempty"`
}

// searchResponse represents the Tavily Search API response.
type searchResponse struct {
	Answer  string `json:"answer"`
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// NewClient creates a new Tavily client.
func NewClient(cfg *config.ProviderConfig, rc *request.Client) (*Client, error) {
	if cfg.Key == "" {
		return nil, fmt.Errorf("tavily api key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		apiKey:   cfg.Key,
		baseURL:  baseURL,
		profiles: cfg.Profiles,
		rc:       rc,
	}, nil
}

// GenerateText sends a query to Tavily and returns the AI-synthesized answer.
func (c *Client) GenerateText(ctx context.Context, profile, prompt string) (string, error) {
	c.mu.RLock()
	_, ok := c.profiles[profile]
	c.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("profile %q not configured for tavily", profile)
	}

	prompt, err := llm.ResolvePrompt(ctx, c.Name(), profile, prompt)
	if err != nil {
		return "", err
	}

	req := searchRequest{
		Query:         prompt,
		SearchDepth:   "advanced",
		IncludeAnswer: "advanced", // Use "advanced" for comprehensive AI-synthesized answers
		IncludeImages: false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tavily request: %w", err)
	}

	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
		"Content-Type":  "application/json",
	}

	// Inject the provider label for accurate tracking/stats
	ctx = context.WithValue(ctx, request.CtxProviderLabel, c.getLabel())

	respBody, err := c.rc.PostWithHeaders(ctx, c.baseURL, body, headers)
	if err != nil {
		return "", err
	}

	var sresp searchResponse
	if err := json.Unmarshal(respBody, &sresp); err != nil {
		return "", fmt.Errorf("failed to unmarshal tavily response: %w", err)
	}

	return sresp.Answer, nil
}

// GenerateJSON is not natively supported by the Tavily Search API.
func (c *Client) GenerateJSON(ctx context.Context, profile, prompt string, target any) error {
	return fmt.Errorf("tavily provider does not support JSON generation")
}

// GenerateImageText is not supported by Tavily.
func (c *Client) GenerateImageText(ctx context.Context, profile, prompt, imagePath string) (string, error) {
	return "", fmt.Errorf("tavily provider does not support image input")
}

// GenerateImageJSON is not supported by Tavily.
func (c *Client) GenerateImageJSON(ctx context.Context, profile, prompt, imagePath string, target any) error {
	return fmt.Errorf("tavily provider does not support image input")
}

// ValidateModels is a no-op for Tavily as it doesn't have a standard /models endpoint.
func (c *Client) ValidateModels(ctx context.Context) error {
	slog.Debug("Skipping Tavily model validation (disabled)")
	return nil
}

// HasProfile checks if the provider has a specific profile configured.
func (c *Client) HasProfile(profile string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	model, ok := c.profiles[profile]
	return ok && model != ""
}

func (c *Client) Name() string {
	return c.getLabel()
}

// SetLabel configures the provider label used for tracking and stats.
func (c *Client) SetLabel(label string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.label = label
}

func (c *Client) getLabel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.label == "" {
		return "tavily"
	}
	return c.label
}
