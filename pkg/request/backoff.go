package request

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// ProviderBackoff manages exponential backoff per provider.
type ProviderBackoff struct {
	mu        sync.RWMutex
	providers map[string]*backoffState
	baseDelay time.Duration
	maxDelay  time.Duration
}

type backoffState struct {
	failureCount int
	nextAllowed  time.Time
}

// NewProviderBackoff creates a new backoff manager.
func NewProviderBackoff(baseDelay, maxDelay time.Duration) *ProviderBackoff {
	return &ProviderBackoff{
		providers: make(map[string]*backoffState),
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
	}
}

// Wait blocks until the provider is allowed to make a request.
func (b *ProviderBackoff) Wait(provider string) {
	b.mu.RLock()
	state, exists := b.providers[provider]
	b.mu.RUnlock()

	if !exists {
		return // No backoff state, proceed immediately
	}

	now := time.Now()
	if now.Before(state.nextAllowed) {
		time.Sleep(time.Until(state.nextAllowed))
	}
}

// RecordFailure increases the backoff delay for a provider.
func (b *ProviderBackoff) RecordFailure(provider string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	state, exists := b.providers[provider]
	if !exists {
		state = &backoffState{}
		b.providers[provider] = state
	}

	state.failureCount++
	delay := b.calculateDelay(state.failureCount)
	state.nextAllowed = time.Now().Add(delay)
}

// RecordSuccess decreases the backoff delay (gradual recovery).
func (b *ProviderBackoff) RecordSuccess(provider string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	state, exists := b.providers[provider]
	if !exists {
		return // No state to recover from
	}

	if state.failureCount > 0 {
		state.failureCount--
	}
	if state.failureCount == 0 {
		state.nextAllowed = time.Time{} // Clear backoff
	}
}

// calculateDelay returns exponential delay with jitter.
func (b *ProviderBackoff) calculateDelay(failures int) time.Duration {
	// Exponential: baseDelay * 2^(failures-1)
	multiplier := math.Pow(2, float64(failures-1))
	delay := time.Duration(float64(b.baseDelay) * multiplier)

	// Cap at maxDelay
	if delay > b.maxDelay {
		delay = b.maxDelay
	}

	// Add 10% jitter
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay))
	return delay + jitter
}

// GetState returns current backoff state for a provider (for debugging/metrics).
func (b *ProviderBackoff) GetState(provider string) (failureCount int, nextAllowed time.Time) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if state, exists := b.providers[provider]; exists {
		return state.failureCount, state.nextAllowed
	}
	return 0, time.Time{}
}
