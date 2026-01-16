package narrator

import "log/slog"

// handleManualQueueAndOverride checks if a manual request needs to be queued or if it overrides staged content.
// Returns true if the request was queued and processing should stop.
func (s *AIService) handleManualQueueAndOverride(poiID, strategy string, manual, enqueueIfBusy bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if manual && enqueueIfBusy && s.active {
		s.pendingManualID = poiID
		s.pendingManualStrategy = strategy
		slog.Info("Narrator: Queued manual request (service busy)", "poi_id", poiID)
		return true
	}

	// If we are proceeding with manual play (immediate override), discard ANY staged automation.
	if manual && s.stagedNarrative != nil {
		slog.Warn("Narrator: Manual Override - Discarding prepared automation",
			"staged", s.stagedNarrative.POI.WikidataID,
			"requested", poiID)
		s.stagedNarrative = nil
	}
	return false
}
