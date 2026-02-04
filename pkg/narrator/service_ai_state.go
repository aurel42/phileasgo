package narrator

import (
	"context"
	"fmt"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/model"
	"time"
)

func (s *AIService) handleGenerationState(req *GenerationRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.SkipBusyCheck {
		s.generating = true
		s.generatingPOI = req.POI
		s.generatingTitle = req.Title
		s.generatingThumbnail = req.ThumbnailURL
		return nil
	}

	if s.generating {
		return fmt.Errorf("narrator already generating")
	}
	s.generating = true
	s.generatingPOI = req.POI
	s.generatingTitle = req.Title
	s.generatingThumbnail = req.ThumbnailURL
	return nil
}

func (s *AIService) claimGeneration(p *model.POI) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generating {
		return false
	}
	s.generating = true
	s.generatingPOI = p
	return true
}

func (s *AIService) releaseGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generating = false
	s.generatingPOI = nil
	s.generatingTitle = ""
	s.generatingThumbnail = ""
}

func (s *AIService) IsGenerating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generating || s.genQ.Count() > 0
}

func (s *AIService) IsActive() bool {
	return s.IsGenerating()
}

func (s *AIService) HasPendingGeneration() bool {
	return s.genQ.HasPending()
}

func (s *AIService) IsPOIBusy(poiID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.generatingPOI != nil && s.generatingPOI.WikidataID == poiID {
		return true
	}

	if s.genQ.HasPOI(poiID) {
		return true
	}

	return false
}

func (s *AIService) GetPreparedPOI() *model.POI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generatingPOI
}

func (s *AIService) Stats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]any, len(s.stats))
	for k, v := range s.stats {
		res[k] = v
	}
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

func (s *AIService) AverageLatency() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.latencies) == 0 {
		return 60 * time.Second
	}
	var sum time.Duration
	for _, d := range s.latencies {
		sum += d
	}
	return sum / time.Duration(len(s.latencies))
}

func (s *AIService) updateLatency(d time.Duration) {
	s.mu.Lock()
	s.latencies = append(s.latencies, d)
	if len(s.latencies) > 10 {
		s.latencies = s.latencies[1:]
	}

	var sum time.Duration
	for _, lat := range s.latencies {
		sum += lat
	}
	avg := sum / time.Duration(len(s.latencies))
	s.mu.Unlock()

	predWindow := max(avg*2, 60*time.Second)
	s.sim.SetPredictionWindow(predWindow)
}

func (s *AIService) POIManager() POIProvider {
	return s.poiMgr
}

func (s *AIService) LLMProvider() llm.Provider {
	return s.llm
}

func (s *AIService) Reset(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generatingPOI = nil
	s.generating = false
	s.genQ.Clear()
}

func (s *AIService) ResetSession(ctx context.Context) {
	s.Reset(ctx)
}

func (s *AIService) IsPlaying() bool                                             { return false }
func (s *AIService) ProcessPlaybackQueue(ctx context.Context)                    {}
func (s *AIService) PlayNarrative(ctx context.Context, n *model.Narrative) error { return nil }
func (s *AIService) SkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = true
}
func (s *AIService) ShouldSkipCooldown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.skipCooldown
}
func (s *AIService) ResetSkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = false
}
func (s *AIService) IsPaused() bool                      { return false }
func (s *AIService) CurrentPOI() *model.POI              { return nil }
func (s *AIService) CurrentTitle() string                { return "" }
func (s *AIService) CurrentType() model.NarrativeType    { return "" }
func (s *AIService) CurrentShowInfoPanel() bool          { return false }
func (s *AIService) Remaining() time.Duration            { return 0 }
func (s *AIService) ReplayLast(ctx context.Context) bool { return false }
func (s *AIService) CurrentImagePath() string            { return "" }
func (s *AIService) CurrentThumbnailURL() string         { return "" }
func (s *AIService) ClearCurrentImage()                  {}
func (s *AIService) Pause()                              {}
func (s *AIService) Resume()                             {}
func (s *AIService) Skip()                               {}
func (s *AIService) TriggerIdentAction()                 {}
func (s *AIService) HasStagedAuto() bool                 { return false }
