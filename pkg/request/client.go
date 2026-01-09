package request

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"bytes"
	"phileasgo/pkg/cache"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/version"
	"strings"
)

var (
	defaultUserAgent = fmt.Sprintf("Phileas Tour Guide for MSFS (Phileas/%s; aurel42@gmail.com)", version.Version)
)

// Client handles HTTP requests with queuing, caching, and tracking.
type Client struct {
	httpClient *http.Client
	cache      cache.Cacher
	tracker    *tracker.Tracker

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

// New creates a new Client.
func New(c cache.Cacher, t *tracker.Tracker) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 300 * time.Second},
		cache:      c,
		tracker:    t,
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
			slog.Debug("Cache Hit", "provider", provider, "key", cacheKey)
			return val, nil
		}
		c.tracker.TrackCacheMiss(provider)
		slog.Debug("Cache Miss", "provider", provider, "key", cacheKey)
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
			slog.Debug("Cache Hit", "provider", provider, "key", cacheKey)
			return val, nil
		}
		c.tracker.TrackCacheMiss(provider)
		slog.Debug("Cache Miss", "provider", provider, "key", cacheKey)
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

// executeWithBackoff attempts the request with exponential backoff on retryable errors.
func (c *Client) executeWithBackoff(req *http.Request) ([]byte, error) {
	maxAttempts := 3
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Verify context is still alive before dialing
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}

		slog.Debug("Network Request", "host", req.URL.Host, "path", req.URL.Path, "attempt", attempt+1)
		resp, err := c.httpClient.Do(req)

		if err != nil {
			// Check if the error is a context cancellation from OUR side
			if req.Context().Err() != nil {
				return nil, req.Context().Err()
			}

			// Otherwise, it's a network error or server timeout
			slog.Warn("Request failed, retrying", "url", req.URL, "attempt", attempt+1, "error", err)

			// Simple exponential backoff
			sleepDur := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
			select {
			case <-time.After(sleepDur):
				continue
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		// Handle Status Codes
		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			resp.Body.Close()
			slog.Warn("API Backoff", "status", resp.StatusCode, "url", req.URL, "attempt", attempt+1)

			sleepDur := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
			select {
			case <-time.After(sleepDur):
				continue
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return nil, fmt.Errorf("api error: status %d", resp.StatusCode)
		}

		// Success
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		return body, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}
