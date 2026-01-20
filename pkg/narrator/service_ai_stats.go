package narrator

import (
	"log/slog"
	"time"
)

// NarratedCount returns the number of narrated POIs.
func (s *AIService) NarratedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.narratedCount
}

// Stats returns narrator statistics.
func (s *AIService) Stats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	res := make(map[string]any, len(s.stats))
	for k, v := range s.stats {
		res[k] = v
	}
	// Add prediction window stats
	if len(s.latencies) > 0 {
		var sum time.Duration
		for _, d := range s.latencies {
			sum += d
		}
		avg := sum / time.Duration(len(s.latencies))
		res["latency_avg_ms"] = avg.Milliseconds()
	}
	return res
}

func (s *AIService) updateLatency(d time.Duration) {
	s.mu.Lock()
	s.latencies = append(s.latencies, d)
	if len(s.latencies) > 10 {
		s.latencies = s.latencies[1:]
	}

	// Calculate rolling average and update prediction window
	var sum time.Duration
	for _, lat := range s.latencies {
		sum += lat
	}
	avg := sum / time.Duration(len(s.latencies))
	s.mu.Unlock()

	// Update the sim's prediction window with the observed latency (minimum 60s)
	predWindow := max(avg*2, 60*time.Second)
	s.sim.SetPredictionWindow(predWindow)
	slog.Debug("Narrator: Updated latency stats", "new_latency", d, "rolling_window_size", len(s.latencies), "new_prediction_window", predWindow)
}

func (s *AIService) AverageLatency() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.latencies) == 0 {
		return 60 * time.Second // Default initial window
	}
	var sum time.Duration
	for _, lat := range s.latencies {
		sum += lat
	}
	return sum / time.Duration(len(s.latencies))
}
