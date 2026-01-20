package narrator

import (
	"testing"
	"time"
)

func TestAIService_StatsAndLatency(t *testing.T) {
	mockSim := &MockSim{}
	svc := &AIService{
		sim:   mockSim,
		stats: make(map[string]any),
	}

	// 1. Initial stats
	stats := svc.Stats()
	if len(stats) != 0 {
		t.Error("expected empty stats")
	}

	// 2. Update latency
	svc.updateLatency(100 * time.Millisecond)
	svc.updateLatency(200 * time.Millisecond)

	stats = svc.Stats()
	avg, ok := stats["latency_avg_ms"]
	if !ok || avg.(int64) != 150 {
		t.Errorf("expected avg 150ms, got %v", avg)
	}

	if svc.AverageLatency() != 150*time.Millisecond {
		t.Errorf("expected average latency 150ms, got %v", svc.AverageLatency())
	}

	// 3. Verify sim prediction window update
	// avg=150ms, avg*2=300ms. Min 60s.
	if mockSim.PredWindow != 60*time.Second {
		t.Errorf("expected 60s pred window, got %v", mockSim.PredWindow)
	}

	// Higher latency
	for i := 0; i < 11; i++ {
		svc.updateLatency(1 * time.Minute)
	}
	if mockSim.PredWindow != 2*time.Minute {
		t.Errorf("expected 120s pred window, got %v", mockSim.PredWindow)
	}
}

func TestAIService_NarratedCount(t *testing.T) {
	svc := &AIService{narratedCount: 5}
	if svc.NarratedCount() != 5 {
		t.Error("mismatch narrated count")
	}
}
