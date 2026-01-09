package request

import (
	"testing"
	"time"
)

func TestProviderBackoff_ExponentialDelay(t *testing.T) {
	tests := []struct {
		name      string
		failures  int
		baseDelay time.Duration
		maxDelay  time.Duration
		wantMinMs int64
		wantMaxMs int64
	}{
		{"First failure", 1, 1 * time.Second, 60 * time.Second, 1000, 1200},
		{"Second failure", 2, 1 * time.Second, 60 * time.Second, 2000, 2400},
		{"Third failure", 3, 1 * time.Second, 60 * time.Second, 4000, 4800},
		{"Max cap hit", 10, 1 * time.Second, 60 * time.Second, 60000, 66000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewProviderBackoff(tt.baseDelay, tt.maxDelay)

			// Simulate failures
			for i := 0; i < tt.failures; i++ {
				b.RecordFailure("test-provider")
			}

			fc, nextAllowed := b.GetState("test-provider")
			if fc != tt.failures {
				t.Errorf("failureCount = %d, want %d", fc, tt.failures)
			}

			delay := time.Until(nextAllowed)
			delayMs := delay.Milliseconds()

			// Allow some tolerance for jitter and timing
			if delayMs < tt.wantMinMs || delayMs > tt.wantMaxMs {
				t.Errorf("delay = %dms, want between %dms and %dms", delayMs, tt.wantMinMs, tt.wantMaxMs)
			}
		})
	}
}

func TestProviderBackoff_GradualRecovery(t *testing.T) {
	b := NewProviderBackoff(1*time.Second, 60*time.Second)

	// Build up failures
	b.RecordFailure("provider")
	b.RecordFailure("provider")
	b.RecordFailure("provider")

	fc, _ := b.GetState("provider")
	if fc != 3 {
		t.Errorf("after 3 failures, count = %d, want 3", fc)
	}

	// Gradual recovery
	b.RecordSuccess("provider")
	fc, _ = b.GetState("provider")
	if fc != 2 {
		t.Errorf("after 1 success, count = %d, want 2", fc)
	}

	b.RecordSuccess("provider")
	b.RecordSuccess("provider")
	fc, _ = b.GetState("provider")
	if fc != 0 {
		t.Errorf("after full recovery, count = %d, want 0", fc)
	}
}

func TestProviderBackoff_IsolatedProviders(t *testing.T) {
	b := NewProviderBackoff(1*time.Second, 60*time.Second)

	b.RecordFailure("wikidata")
	b.RecordFailure("wikidata")

	fc1, _ := b.GetState("wikidata")
	fc2, _ := b.GetState("wikipedia")

	if fc1 != 2 {
		t.Errorf("wikidata failures = %d, want 2", fc1)
	}
	if fc2 != 0 {
		t.Errorf("wikipedia failures = %d, want 0 (isolated)", fc2)
	}
}
