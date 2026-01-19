package narrator

// handleManualQueueAndOverride checks if a manual request needs to be queued or if it overrides staged content.
// Returns true if the request was queued and processing should stop.
func (s *AIService) handleManualQueueAndOverride(poiID, strategy string, manual, enqueueIfBusy bool) bool {
	// With the queue system, we let PlayPOI handle logic.
	// Manual requests will force generation and enqueue at high priority.
	// We no longer need to check stagedNarrative or set pendingManualID here for blocking.
	return false
}
