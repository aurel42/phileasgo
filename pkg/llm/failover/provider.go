package failover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"os"
	"path/filepath"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

// Provider wraps multiple LLM providers and handles fallbacks.
type Provider struct {
	providers []llm.Provider
	names     []string
	timeouts  []time.Duration
	disabled  map[int]bool
	backoffs  map[string]*backoffState // key: providerName:profileName
	logPath   string
	enabled   bool
	tracker   *tracker.Tracker
	mu        sync.RWMutex
}

type backoffState struct {
	subsequentFailures int
	skippedRequests    int
	targetSkips        int
}

// New creates a new Provider with failover and unified logging.
// providers: ordered list of all initialized providers (global fallback chain).
// names: names corresponding to the provider list.
func New(providers []llm.Provider, names []string, timeouts []time.Duration, logPath string, enabled bool, t *tracker.Tracker) (*Provider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one provider required for failover")
	}
	if len(providers) != len(names) {
		return nil, fmt.Errorf("provider count (%d) does not match name count (%d)", len(providers), len(names))
	}
	if len(providers) != len(timeouts) {
		return nil, fmt.Errorf("provider count (%d) does not match timeout count (%d)", len(providers), len(timeouts))
	}

	return &Provider{
		providers: providers,
		names:     names,
		timeouts:  timeouts,
		disabled:  make(map[int]bool),
		backoffs:  make(map[string]*backoffState),
		logPath:   logPath,
		enabled:   enabled,
		tracker:   t,
	}, nil
}

// GenerateText implements llm.Provider.
func (f *Provider) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	res, err := f.execute(ctx, name, prompt, func(pCtx context.Context, p llm.Provider) (any, error) {
		return p.GenerateText(pCtx, name, prompt)
	})
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// GenerateJSON implements llm.Provider.
func (f *Provider) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	_, err := f.execute(ctx, name, prompt, func(pCtx context.Context, p llm.Provider) (any, error) {
		err := p.GenerateJSON(pCtx, name, prompt, target)
		if err != nil {
			return nil, err
		}
		return target, nil
	})
	return err
}

// GenerateImageText implements llm.Provider.
func (f *Provider) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	res, err := f.execute(ctx, name, prompt, func(pCtx context.Context, p llm.Provider) (any, error) {
		return p.GenerateImageText(pCtx, name, prompt, imagePath)
	})
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// HasProfile implements llm.Provider.
func (f *Provider) HasProfile(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, p := range f.providers {
		if p.HasProfile(name) {
			return true
		}
	}
	return false
}

// ValidateModels checks if the configured models are available for all providers.
func (f *Provider) ValidateModels(ctx context.Context) error {
	f.mu.RLock()
	providers := f.providers
	names := f.names
	disabled := make(map[int]bool)
	for k, v := range f.disabled {
		disabled[k] = v
	}
	f.mu.RUnlock()

	var errors []string
	for i, p := range providers {
		if disabled[i] {
			continue
		}
		if err := p.ValidateModels(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", names[i], err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("llm model validation failed: %s", strings.Join(errors, "; "))
	}
	return nil
}

// execute runs the given function against the provider chain.
func (f *Provider) execute(ctx context.Context, callName, prompt string, fn func(context.Context, llm.Provider) (any, error)) (any, error) {
	f.mu.RLock()
	providers := f.providers
	names := f.names
	f.mu.RUnlock()

	// Filter providers that actually support the requested profile
	// This implicitly handles the "Sparse Profile" requirement.
	type candidate struct {
		index int
		p     llm.Provider
		name  string
	}
	var candidates []candidate

	for i, p := range providers {
		// 1. Check Circuit Breaker
		f.mu.RLock()
		isDisabled := f.disabled[i]
		f.mu.RUnlock()
		if isDisabled {
			continue
		}

		// 2. Check Profile Support (Dynamic Routing)
		if !p.HasProfile(callName) {
			continue
		}

		candidates = append(candidates, candidate{i, p, names[i]})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active provider supports profile %q", callName)
	}

	for idx, c := range candidates {
		// 3. Check Smart Backoff
		backoffKey := c.name + ":" + callName
		f.mu.Lock()
		bs, exists := f.backoffs[backoffKey]
		if exists && bs.skippedRequests < bs.targetSkips {
			bs.skippedRequests++
			slog.Info("LLM Provider in backoff, skipping",
				"provider", c.name,
				"profile", callName,
				"skipped", bs.skippedRequests,
				"target", bs.targetSkips,
				"failures", bs.subsequentFailures,
			)
			f.mu.Unlock()
			continue
		}
		f.mu.Unlock()

		// 4. Execute with Timeout
		timeout := f.timeouts[c.index]
		callCtx, cancel := context.WithTimeout(ctx, timeout)

		// Inject MaxAttempts=1 context for all but the last candidate
		// This forces the request client to fail immediately on error, relying on our loop for fallback.
		// The last candidate gets standard behavior (retries allowed as configured in client).
		if idx < len(candidates)-1 {
			callCtx = context.WithValue(callCtx, request.CtxMaxAttempts, 1)
		}

		res, err := fn(callCtx, c.p)
		cancel()

		if err == nil {
			// SUCCESS - Reset Backoff
			f.mu.Lock()
			delete(f.backoffs, backoffKey)
			f.mu.Unlock()

			f.trackStats(c.name, true)
			f.logRequest(c.name, callName, prompt, fmt.Sprintf("%v", res), nil)
			return res, nil
		}

		// Handle error
		f.trackStats(c.name, false)
		f.logRequest(c.name, callName, prompt, "", err)

		isFatal := isUnrecoverable(err)
		isLast := idx == len(candidates)-1

		if isFatal {
			if !isLast {
				slog.Warn("LLM Provider fatal error, disabling for the session", "provider", c.name, "error", err)
				f.mu.Lock()
				f.disabled[c.index] = true
				f.mu.Unlock()
				continue // Try next candidate
			}
			// Last candidate failed fatally
			return nil, err
		}

		// Retryable error: apply backoff increment
		f.mu.Lock()
		bs, exists = f.backoffs[backoffKey]
		if !exists {
			bs = &backoffState{}
			f.backoffs[backoffKey] = bs
		}
		bs.subsequentFailures++
		bs.skippedRequests = 0
		// Exponential skip: 2^(N-1)
		bs.targetSkips = int(1 << (uint(bs.subsequentFailures) - 1))
		f.mu.Unlock()

		if !isLast {
			slog.Info("LLM Provider failed (retryable), falling back",
				"provider", c.name,
				"next", candidates[idx+1].name,
				"error", err,
				"failures", bs.subsequentFailures,
				"next_skips", bs.targetSkips,
			)
			continue // Try next immediately
		}

		// Last candidate, retry with backoff
		res, err = f.retryLast(ctx, c.p, c.name, timeout, fn)
		if err != nil {
			f.logRequest(c.name, callName, prompt, "", err)
		} else {
			// Success on retry: reset backoff
			f.mu.Lock()
			delete(f.backoffs, backoffKey)
			f.mu.Unlock()
			f.logRequest(c.name, callName, prompt, fmt.Sprintf("%v", res), nil)
		}
		return res, err
	}

	return nil, fmt.Errorf("all LLM providers exhausted for profile %q", callName)
}

func (f *Provider) retryLast(ctx context.Context, p llm.Provider, name string, timeout time.Duration, fn func(context.Context, llm.Provider) (any, error)) (any, error) {
	var lastErr error
	delay := 1 * time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		res, err := fn(callCtx, p)
		cancel()
		if err == nil {
			f.trackStats(name, true)
			return res, nil
		}

		f.trackStats(name, false)
		lastErr = err
		if isUnrecoverable(err) {
			return nil, fmt.Errorf("last provider failed with fatal error: %w", err)
		}

		slog.Warn("Last LLM provider failed, retrying with backoff", "provider", name, "attempt", attempt, "next_delay", delay, "error", err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
	return nil, fmt.Errorf("last provider exhausted after 3 retries: %w", lastErr)
}

func (f *Provider) trackStats(providerName string, success bool) {
	if f.tracker == nil {
		return
	}
	// Tracking is now handled by individual providers or the request client.
	// We no longer track global "llm" stats to prevent double counting and clutter.

}

func (f *Provider) logRequest(providerName, callName, prompt, response string, err error) {
	if f.logPath == "" || !f.enabled {
		return
	}

	if err := os.MkdirAll(filepath.Dir(f.logPath), 0o755); err != nil {
		return
	}

	file, fErr := os.OpenFile(f.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if fErr != nil {
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var entry string

	if err != nil {
		// 1) for unsuccessful requests, we log in llm.log only the fact that they happened and the reason why they failed.
		entry = fmt.Sprintf("[%s][%s] ERROR: %s - %v\n%s\n",
			timestamp, strings.ToUpper(providerName), callName, err, strings.Repeat("-", 80))
	} else {
		// 2) for successful requests, we log in llm.log the full prompt, but we truncate wikipedia article lines as before
		// 3) for successful requests, we log in llm.log the full response, but we wrap it to 80 chars.
		wrappedResponse := llm.WordWrap(response, 80)

		entry = fmt.Sprintf("[%s][%s] PROMPT: %s\nPROMPT_TEXT:\n%s\n\nRESPONSE:\n%s\n%s\n",
			timestamp, strings.ToUpper(providerName), callName, prompt, wrappedResponse, strings.Repeat("-", 80))
	}

	_, _ = file.WriteString(entry)
}

// isUnrecoverable identifies errors that should trigger a circuit break (unless it's the last provider).
func isUnrecoverable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// 401: Unauthorized (Invalid Key)
	// 403: Forbidden (Disabled Key / Restricted Access)
	// Note: 429 (Too Many Requests) and 400 (Bad Request) are NOT fatal.
	// 400 might be a model-specific issue or a transient prompt error that doesn't warrant disabling the provider.
	return strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "invalid_api_key")
}
