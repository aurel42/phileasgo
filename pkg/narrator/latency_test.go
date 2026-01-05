package narrator

import (
	"testing"
	"time"

	"phileasgo/pkg/config"
)

// TestLatencyTracking verifies the rolling window logic for latency stats.
func TestLatencyTracking(t *testing.T) {
	s := &AIService{
		cfg:       config.DefaultConfig(),
		latencies: make([]time.Duration, 0, 10),
		stats:     make(map[string]any),
	}

	// 1. Initial state (empty)
	if avg := s.getPredictedLatency(); avg != 60*time.Second {
		t.Errorf("Expected default 60s, got %v", avg)
	}

	// 2. Add one value (10s)
	s.updateLatency(10 * time.Second)
	if avg := s.getPredictedLatency(); avg != 10*time.Second {
		t.Errorf("Expected 10s, got %v", avg)
	}

	// 3. Add second value (20s) -> Avg 15s
	s.updateLatency(20 * time.Second)
	if avg := s.getPredictedLatency(); avg != 15*time.Second {
		t.Errorf("Expected 15s, got %v", avg)
	}

	// 4. Fill window (add 8 more 20s) -> [10, 20, 20...]
	for i := 0; i < 8; i++ {
		s.updateLatency(20 * time.Second)
	}
	// Sum: 10 + 9*20 = 190. Count: 10. Avg: 19s.
	if avg := s.getPredictedLatency(); avg != 19*time.Second {
		t.Errorf("Expected 19s, got %v", avg)
	}
	if len(s.latencies) != 10 {
		t.Errorf("Expected length 10, got %d", len(s.latencies))
	}

	// 5. Overflow (add 100s) -> [20, 20... 100]. Oldest (10) removed.
	// Current: [20, 20, 20, 20, 20, 20, 20, 20, 20, 100]
	// Sum: 9*20 + 100 = 280. Avg: 28s.
	s.updateLatency(100 * time.Second)
	if avg := s.getPredictedLatency(); avg != 28*time.Second {
		t.Errorf("Expected 28s, got %v", avg)
	}
	if len(s.latencies) != 10 {
		t.Errorf("Expected length 10, got %d", len(s.latencies))
	}
}
