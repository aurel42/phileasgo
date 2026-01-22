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
	"phileasgo/pkg/tracker"
)

// Provider wraps multiple LLM providers and handles fallbacks.
type Provider struct {
	providers []llm.Provider
	names     []string
	disabled  map[int]bool
	logPath   string
	tracker   *tracker.Tracker
	mu        sync.RWMutex
}

// New creates a new Provider with failover and unified logging.
func New(providers []llm.Provider, names []string, logPath string, t *tracker.Tracker) (*Provider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one provider required for failover")
	}
	if len(providers) != len(names) {
		return nil, fmt.Errorf("provider count (%d) does not match name count (%d)", len(providers), len(names))
	}

	return &Provider{
		providers: providers,
		names:     names,
		disabled:  make(map[int]bool),
		logPath:   logPath,
		tracker:   t,
	}, nil
}

// GenerateText implements llm.Provider.
func (f *Provider) GenerateText(ctx context.Context, name, prompt string) (string, error) {
	res, err := f.execute(ctx, name, prompt, func(p llm.Provider) (any, error) {
		return p.GenerateText(ctx, name, prompt)
	})
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// GenerateJSON implements llm.Provider.
func (f *Provider) GenerateJSON(ctx context.Context, name, prompt string, target any) error {
	_, err := f.execute(ctx, name, prompt, func(p llm.Provider) (any, error) {
		return nil, p.GenerateJSON(ctx, name, prompt, target)
	})
	return err
}

// GenerateImageText implements llm.Provider.
func (f *Provider) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	res, err := f.execute(ctx, name, prompt, func(p llm.Provider) (any, error) {
		return p.GenerateImageText(ctx, name, prompt, imagePath)
	})
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// HealthCheck verifies that at least one provider is healthy.
func (f *Provider) HealthCheck(ctx context.Context) error {
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
		if err := p.HealthCheck(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", names[i], err))
			continue
		}
		return nil // At least one is healthy
	}

	if len(errors) == 0 {
		return fmt.Errorf("no providers available in failover chain")
	}
	return fmt.Errorf("all LLM providers failed health check: %s", strings.Join(errors, "; "))
}

// execute runs the given function against the provider chain.
func (f *Provider) execute(ctx context.Context, callName, prompt string, fn func(llm.Provider) (any, error)) (any, error) {
	f.mu.RLock()
	providers := f.providers
	names := f.names
	f.mu.RUnlock()

	lastIndex := len(providers) - 1

	for i, p := range providers {
		// Skip if disabled (Circuit Breaker)
		f.mu.RLock()
		isDisabled := f.disabled[i]
		f.mu.RUnlock()

		if isDisabled {
			continue
		}

		res, err := fn(p)
		if err == nil {
			// SUCCESS
			f.trackStats(names[i], true)
			f.logRequest(names[i], callName, prompt, fmt.Sprintf("%v", res), nil)
			return res, nil
		}

		// Handle error
		f.trackStats(names[i], false)
		f.logRequest(names[i], callName, prompt, "", err)

		isFatal := isUnrecoverable(err)
		isLast := i == lastIndex

		if isFatal {
			if !isLast {
				slog.Warn("LLM Provider fatal error, disabling for the session", "provider", names[i], "error", err)
				f.mu.Lock()
				f.disabled[i] = true
				f.mu.Unlock()
				continue // Try next
			}
			// It's the last one, don't disable, just return the error
			return nil, err
		}

		// Retryable error
		if !isLast {
			slog.Info("LLM Provider failed (retryable), falling back", "provider", names[i], "next", names[i+1], "error", err)
			continue // Try next immediately
		}

		// It's the last provider, retry with backoff
		res, err = f.retryLast(p, names[i], fn)
		if err != nil {
			f.logRequest(names[i], callName, prompt, "", err)
		} else {
			f.logRequest(names[i], callName, prompt, fmt.Sprintf("%v", res), nil)
		}
		return res, err
	}

	return nil, fmt.Errorf("all LLM providers exhausted or disabled")
}

func (f *Provider) retryLast(p llm.Provider, name string, fn func(llm.Provider) (any, error)) (any, error) {
	var lastErr error
	delay := 1 * time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		res, err := fn(p)
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
		time.Sleep(delay)
		delay *= 2
	}
	return nil, fmt.Errorf("last provider exhausted after 3 retries: %w", lastErr)
}

func (f *Provider) trackStats(providerName string, success bool) {
	if f.tracker == nil {
		return
	}
	if success {
		f.tracker.TrackAPISuccess(providerName)
		f.tracker.TrackAPISuccess("llm") // Global stat
	} else {
		f.tracker.TrackAPIFailure(providerName)
		f.tracker.TrackAPIFailure("llm") // Global stat
	}
}

func (f *Provider) logRequest(providerName, callName, prompt, response string, err error) {
	if f.logPath == "" {
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
		truncatedPrompt := llm.TruncateParagraphs(prompt, 80)

		entry = fmt.Sprintf("[%s][%s] PROMPT: %s\nPROMPT_TEXT:\n%s\n\nRESPONSE:\n%s\n%s\n",
			timestamp, strings.ToUpper(providerName), callName, truncatedPrompt, wrappedResponse, strings.Repeat("-", 80))
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
	// 400: Bad Request (Malformed structure - no point in retrying)
	return strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "400") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "invalid_api_key")
}
