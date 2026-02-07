package request

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"bytes"
	"phileasgo/pkg/cache"
	"phileasgo/pkg/logging"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/version"
	"strings"
)

var (
	defaultUserAgent = fmt.Sprintf("Phileas Tour Guide for MSFS (Phileas/%s; aurel42@gmail.com)", version.Version)
)

// CtxKey is a type for context keys to avoid collisions.
type CtxKey string

// CtxMaxAttempts is the context key for overriding the maximum number of attempts.
// Value should be an int.
const CtxMaxAttempts CtxKey = "max_attempts"

// Client handles HTTP requests with queuing, caching, and tracking.
type Client struct {
	httpClient *http.Client
	cache      cache.Cacher
	tracker    *tracker.Tracker
	backoff    *ProviderBackoff

	// Config
	retries int

	// Queues per provider (domain)
	queues map[string]chan job
	mu     sync.Mutex // Protects queues map
}

// job represents a queued request.
type job struct {
	req      *http.Request
	headers  map[string]string
	cacheKey string
	respChan chan jobResult
}

type jobResult struct {
	body []byte
	err  error
}

// ClientConfig holds configuration for the request Client.
type ClientConfig struct {
	Retries   int
	Timeout   time.Duration
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// New creates a new Client.
func New(c cache.Cacher, t *tracker.Tracker, cfg ClientConfig) *Client {
	// Use defaults if not provided
	if cfg.Retries == 0 {
		cfg.Retries = 5
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300 * time.Second
	}
	if cfg.BaseDelay == 0 {
		cfg.BaseDelay = 1 * time.Second
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 60 * time.Second
	}

	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		cache:      c,
		tracker:    t,
		backoff:    NewProviderBackoff(cfg.BaseDelay, cfg.MaxDelay),
		retries:    cfg.Retries,
		queues:     make(map[string]chan job),
	}
}

// Get performs a GET request with queuing and caching if key is provided.
func (c *Client) Get(ctx context.Context, u, cacheKey string) ([]byte, error) {
	return c.GetWithHeaders(ctx, u, nil, cacheKey)
}

// GetWithHeaders performs a GET request with custom headers and optional caching.
func (c *Client) GetWithHeaders(ctx context.Context, u string, headers map[string]string, cacheKey string) ([]byte, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	host := parsedURL.Host
	provider := normalizeProvider(host)

	// 1. Check Cache (Only if key is provided)
	if cacheKey != "" {
		if val, hit := c.cache.GetCache(ctx, cacheKey); hit {
			c.tracker.TrackCacheHit(provider)
			logging.TraceDefault("Cache Hit", "provider", provider, "key", cacheKey)
			return val, nil
		}
		c.tracker.TrackCacheMiss(provider)
		logging.TraceDefault("Cache Miss", "provider", provider, "key", cacheKey)
	}

	// 2. Enqueue Request
	req, err := http.NewRequestWithContext(ctx, "GET", u, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	respChan := make(chan jobResult, 1)
	j := job{req: req, headers: headers, cacheKey: cacheKey, respChan: respChan}

	c.dispatch(provider, j)

	// 3. Wait for Result
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-respChan:
		return res.body, res.err
	}
}

// Post performs a POST request with queuing.
func (c *Client) Post(ctx context.Context, u string, body []byte, contentType string) ([]byte, error) {
	return c.PostWithHeaders(ctx, u, body, map[string]string{"Content-Type": contentType})
}

// PostWithHeaders performs a POST request with custom headers and queuing.
func (c *Client) PostWithHeaders(ctx context.Context, u string, body []byte, headers map[string]string) ([]byte, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	host := parsedURL.Host
	provider := normalizeProvider(host)

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	respChan := make(chan jobResult, 1)
	j := job{req: req, headers: headers, respChan: respChan}

	c.dispatch(provider, j)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-respChan:
		return res.body, res.err
	}
}

func normalizeProvider(host string) string {
	// Group all wikidata subdomains (www, query, etc.) into one "wikidata" provider for serialization
	if strings.HasSuffix(host, ".wikidata.org") || host == "wikidata.org" {
		return "wikidata"
	}
	if strings.HasSuffix(host, ".wikipedia.org") || host == "wikipedia.org" {
		return "wikipedia"
	}
	if strings.HasSuffix(host, "googleapis.com") {
		return "gemini"
	}
	if strings.HasSuffix(host, "groq.com") {
		return "groq"
	}
	if strings.HasSuffix(host, "perplexity.ai") {
		return "perplexity"
	}
	if strings.HasSuffix(host, "deepseek.com") {
		return "deepseek"
	}
	return host
}

// dispatch sends the job to the provider's queue, creating the queue/worker if needed.
func (c *Client) dispatch(provider string, j job) {
	c.mu.Lock()
	defer c.mu.Unlock()

	q, ok := c.queues[provider]
	if !ok {
		// Create new queue and start worker
		q = make(chan job, 100)
		c.queues[provider] = q
		go c.worker(provider, q)
	}

	// We block here if the queue is full, effectively throttling the caller
	select {
	case q <- j:
	case <-j.req.Context().Done():
		// Caller gave up before we could even enqueue
		j.respChan <- jobResult{err: j.req.Context().Err()}
	}
}

// worker processes requests for a specific provider sequentially.
func (c *Client) worker(provider string, q <-chan job) {
	for j := range q {
		// Check context before processing
		if j.req.Context().Err() != nil {
			slog.Warn("Job dropped from queue (context expired)", "provider", provider, "error", j.req.Context().Err())
			j.respChan <- jobResult{err: j.req.Context().Err()}
			continue
		}

		// Apply User-Agent (Default if not provided)
		uaMatch := false
		for k, v := range j.headers {
			j.req.Header.Set(k, v)
			if http.CanonicalHeaderKey(k) == "User-Agent" {
				uaMatch = true
			}
		}
		if !uaMatch {
			j.req.Header.Set("User-Agent", defaultUserAgent)
		}

		body, err := c.executeWithBackoff(j.req)

		if err == nil {
			c.tracker.TrackAPISuccess(provider)
			// Cache result (Only if key is provided)
			if j.cacheKey != "" {
				if err := c.cache.SetCache(context.Background(), j.cacheKey, body); err != nil {
					slog.Error("Failed to cache response", "url", j.req.URL, "error", err)
				}
			}
		} else {
			c.tracker.TrackAPIFailure(provider)
		}

		j.respChan <- jobResult{body: body, err: err}

		// Hardcoded safety gap to prevent hitting rate limits
		time.Sleep(100 * time.Millisecond)
	}
}

// PostWithCache performs a POST request with queuing and caching.
func (c *Client) PostWithCache(ctx context.Context, u string, body []byte, headers map[string]string, cacheKey string) ([]byte, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	host := parsedURL.Host
	provider := normalizeProvider(host)

	// 1. Check Cache
	if cacheKey != "" {
		if val, hit := c.cache.GetCache(ctx, cacheKey); hit {
			c.tracker.TrackCacheHit(provider)
			logging.TraceDefault("Cache Hit", "provider", provider, "key", cacheKey)
			return val, nil
		}
		c.tracker.TrackCacheMiss(provider)
		logging.TraceDefault("Cache Miss", "provider", provider, "key", cacheKey)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	respChan := make(chan jobResult, 1)
	j := job{req: req, headers: headers, cacheKey: cacheKey, respChan: respChan}

	c.dispatch(provider, j)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-respChan:
		return res.body, res.err
	}
}

// PostWithGeodataCache performs a POST request with caching to the geodata table (with radius metadata).
func (c *Client) PostWithGeodataCache(ctx context.Context, u string, body []byte, headers map[string]string, cacheKey string, radiusM int, lat, lon float64) ([]byte, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	host := parsedURL.Host
	provider := normalizeProvider(host)

	// 1. Check Geodata Cache
	if cacheKey != "" {
		if val, _, hit := c.cache.GetGeodataCache(ctx, cacheKey); hit {
			c.tracker.TrackCacheHit(provider)
			logging.TraceDefault("Geodata Cache Hit", "provider", provider, "key", cacheKey)
			return val, nil
		}
		c.tracker.TrackCacheMiss(provider)
		logging.TraceDefault("Geodata Cache Miss", "provider", provider, "key", cacheKey)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Apply headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}

	// Execute with backoff (synchronously, not queued - geodata is important)
	respBody, err := c.executeWithBackoff(req)
	if err != nil {
		c.tracker.TrackAPIFailure(provider)
		return nil, err
	}

	c.tracker.TrackAPISuccess(provider)

	// Cache result to geodata table with radius
	if cacheKey != "" {
		if err := c.cache.SetGeodataCache(ctx, cacheKey, respBody, radiusM, lat, lon); err != nil {
			slog.Error("Failed to cache geodata response", "key", cacheKey, "error", err)
		}
	}

	return respBody, nil
}

// executeWithBackoff attempts the request with exponential backoff on retryable errors.
func (c *Client) executeWithBackoff(req *http.Request) ([]byte, error) {
	provider := normalizeProvider(req.URL.Host)

	maxAttempts := c.retries
	if v := req.Context().Value(CtxMaxAttempts); v != nil {
		if val, ok := v.(int); ok {
			maxAttempts = val
		}
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Wait for any provider-level backoff (unless we are in single-attempt mode)
		if maxAttempts > 1 {
			c.backoff.Wait(provider)
		}

		body, retryable, err := c.executeAttempt(req, provider, attempt)
		if err == nil {
			return body, nil
		}
		if !retryable {
			return nil, err
		}
		// Continue to next attempt
	}

	return nil, fmt.Errorf("max attempts (%d) exceeded for %s", maxAttempts, provider)
}

func (c *Client) executeAttempt(req *http.Request, provider string, attempt int) (body []byte, retryable bool, err error) {
	// Verify context is still alive before dialing
	if req.Context().Err() != nil {
		return nil, false, req.Context().Err()
	}

	logging.TraceDefault("Network Request", "host", req.URL.Host, "path", req.URL.Path, "attempt", attempt+1, "max", c.retries)

	// RESET BODY for retries
	if attempt > 0 && req.GetBody != nil {
		var err error
		req.Body, err = req.GetBody()
		if err != nil {
			return nil, false, fmt.Errorf("failed to reset request body: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check if the error is a context cancellation from OUR side
		if req.Context().Err() != nil {
			return nil, false, req.Context().Err()
		}

		// Network error - record failure and retry
		slog.Debug("Request failed", "provider", provider, "error", err)
		slog.Warn("Request failed, retrying", "provider", provider, "attempt", attempt+1, "error", err)
		c.backoff.RecordFailure(provider)
		return nil, true, err
	}
	defer resp.Body.Close()

	// Handle Status Codes
	if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
		slog.Debug("Request failed (retryable)", "status", resp.StatusCode, "provider", provider)
		slog.Warn("API Backoff", "status", resp.StatusCode, "provider", provider, "attempt", attempt+1)
		c.backoff.RecordFailure(provider)
		return nil, true, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		slog.Debug("Request failed (terminal)", "status", resp.StatusCode, "provider", provider, "url", req.URL.String())
		return nil, false, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	// Success - record it for gradual recovery
	c.backoff.RecordSuccess(provider)

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("read error: %w", err)
	}
	return body, false, nil
}
