package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/model"
)

// HasPendingManualOverride returns true if a user-requested POI is queued.
func (s *AIService) HasPendingManualOverride() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.generationQueue) > 0 || s.pendingManualID != ""
}

// GetPendingManualOverride returns and clears the pending manual override.
func (s *AIService) GetPendingManualOverride() (poiID, strategy string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingManualID != "" {
		id := s.pendingManualID
		strat := s.pendingManualStrategy
		s.pendingManualID = ""
		s.pendingManualStrategy = ""
		return id, strat, true
	}
	return "", "", false
}

// enqueuePlayback adds a narrative to the playback queue.
func (s *AIService) enqueuePlayback(n *model.Narrative, priority bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Max queue size check (e.g. 5)
	if len(s.playbackQueue) >= 5 && !priority {
		slog.Info("Narrator: Queue full, dropping low priority item", "title", n.Title)
		return
	}

	if priority {
		// Prepend
		s.playbackQueue = append([]*model.Narrative{n}, s.playbackQueue...)
	} else {
		// Append
		s.playbackQueue = append(s.playbackQueue, n)
	}
	slog.Info("Narrator: Enqueued narrative", "title", n.Title, "priority", priority, "queue_len", len(s.playbackQueue))
}

// popPlaybackQueue retrieves and removes the next narrative from the queue.
func (s *AIService) popPlaybackQueue() *model.Narrative {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.playbackQueue) == 0 {
		return nil
	}
	n := s.playbackQueue[0]
	s.playbackQueue = s.playbackQueue[1:]
	slog.Debug("Narrator: Popped from queue", "title", n.Title, "remaining", len(s.playbackQueue))
	return n
}

// peekPlaybackQueue returns the head of the queue without removing it.
func (s *AIService) peekPlaybackQueue() *model.Narrative {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.playbackQueue) == 0 {
		return nil
	}
	return s.playbackQueue[0]
}

func (s *AIService) canEnqueuePlayback(nType string, manual bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Auto POI/Essay: Only allowed if queue is empty
	if !manual && (nType == "poi" || nType == "essay") && len(s.playbackQueue) > 0 {
		return false
	}

	return checkQueueLimits(s.playbackQueue, nType, manual)
}

//nolint:gocyclo // Complexity due to limit checking switch
func checkQueueLimits(queue []*model.Narrative, nType string, manual bool) bool {
	// 1. Auto Logic: Only one generation at a time, and only if queue is empty
	// We only allow one auto POI or auto Essay in the playback queue at once.
	if !manual && (nType == "poi" || nType == "essay") && len(queue) > 0 {
		return false
	}

	var manualPOIs, screenshots, debriefs, essays int
	for _, n := range queue {
		switch n.Type {
		case "poi":
			if n.Manual {
				manualPOIs++
			}
		case "screenshot":
			screenshots++
		case "debrief":
			debriefs++
		case "essay":
			essays++
		}
	}

	if nType == "poi" && manual && manualPOIs >= 1 {
		return false
	}
	if nType == "screenshot" && screenshots >= 1 {
		return false
	}
	if nType == "debrief" && debriefs >= 1 {
		return false
	}
	if nType == "essay" && essays >= 1 {
		return false
	}

	return true
}

// enqueueGeneration adds a generation job to the priority queue.
func (s *AIService) enqueueGeneration(job *GenerationJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generationQueue = append(s.generationQueue, job)
	slog.Info("Narrator: Enqueued priority generation job", "type", job.Type, "poi_id", job.POIID, "queue_len", len(s.generationQueue))
}

// HasPendingGeneration returns true if there are items in the priority generation queue.
func (s *AIService) HasPendingGeneration() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.generationQueue) > 0
}

func (s *AIService) ProcessGenerationQueue(ctx context.Context) {
	s.mu.Lock()
	// Serialization: Only one generation at a time.
	// If already generating, the current GenerateNarrative will kick this again on finish.
	if s.generating || len(s.generationQueue) == 0 {
		s.mu.Unlock()
		return
	}
	job := s.generationQueue[0]
	s.generationQueue = s.generationQueue[1:]
	s.mu.Unlock()

	// Process Job
	go func() {
		genCtx := context.Background()
		var req *GenerationRequest

		switch job.Type {
		case model.NarrativeTypePOI:
			p, err := s.poiMgr.GetPOI(genCtx, job.POIID)
			if err != nil {
				slog.Error("Narrator: Priority job failed - POI not found", "poi_id", job.POIID)
				return
			}

			promptData := s.buildPromptData(genCtx, p, job.Telemetry, job.Strategy)
			prompt, _ := s.prompts.Render("narrator/script.tmpl", promptData)

			req = &GenerationRequest{
				Type:     model.NarrativeTypePOI,
				Prompt:   prompt,
				Title:    p.DisplayName(),
				SafeID:   strings.ReplaceAll(p.WikidataID, "/", "_"),
				POI:      p,
				MaxWords: promptData.MaxWords,
				Manual:   job.Manual,
			}

		case model.NarrativeTypeScreenshot:
			loc := s.geoSvc.GetLocation(job.Telemetry.Latitude, job.Telemetry.Longitude)
			data := map[string]any{
				"City":        loc.CityName,
				"Region":      loc.Admin1Name,
				"Country":     loc.CountryCode,
				"MaxWords":    s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthShortWords),
				"TripSummary": s.getTripSummary(),
				"Lat":         fmt.Sprintf("%.3f", job.Telemetry.Latitude),
				"Lon":         fmt.Sprintf("%.3f", job.Telemetry.Longitude),
				"Alt":         fmt.Sprintf("%.0f", job.Telemetry.AltitudeAGL),
			}
			prompt, err := s.prompts.Render("narrator/screenshot.tmpl", data)
			if err != nil {
				slog.Error("Narrator: Failed to render screenshot prompt", "error", err)
				return
			}

			req = &GenerationRequest{
				Type:      model.NarrativeTypeScreenshot,
				Prompt:    prompt,
				Title:     "Screenshot Analysis",
				SafeID:    "screenshot_" + time.Now().Format("150405"),
				ImagePath: job.ImagePath,
				MaxWords:  s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthShortWords),
				Manual:    true,
			}

		default:
			slog.Warn("Narrator: ProcessPriorityQueue received unsupported job type", "type", job.Type)
			return
		}

		n, err := s.GenerateNarrative(genCtx, req)
		if err != nil {
			slog.Error("Narrator: Priority generation failed", "type", job.Type, "error", err)
			// Trigger next even on failure
			s.ProcessGenerationQueue(genCtx)
			return
		}

		// Enqueue & Trigger
		s.enqueuePlayback(n, true)
		go s.ProcessPlaybackQueue(genCtx)

		// Self-perpetuation: Trigger next queued item
		s.ProcessGenerationQueue(genCtx)
	}()
}

// handleManualQueueAndOverride handles busy states for manual requests.
func (s *AIService) handleManualQueueAndOverride(poiID, strategy string, manual, enqueueIfBusy bool) bool {
	if !manual {
		return false
	}

	s.mu.RLock()
	isGenerating := s.generating
	s.mu.RUnlock()

	if isGenerating {
		if enqueueIfBusy {
			s.enqueueGeneration(&GenerationJob{
				Type:      model.NarrativeTypePOI,
				POIID:     poiID,
				Manual:    true,
				Strategy:  strategy,
				CreatedAt: time.Now(),
			})
			return true
		}
		// If NOT enqueueing, we fall through and handleGenerationState will cancel the old job
	}

	return false
}

// updateTripSummary updates the running summary of the flight based on the last narration.
func (s *AIService) updateTripSummary(ctx context.Context, lastTitle, lastScript string) {
	s.mu.Lock()
	currentSummary := s.tripSummary
	s.mu.Unlock()

	// Build update prompt data
	data := struct {
		CurrentSummary string
		LastTitle      string
		LastScript     string
		MaxWords       int
	}{
		CurrentSummary: currentSummary,
		LastTitle:      lastTitle,
		LastScript:     lastScript,
		MaxWords:       s.cfg.Narrator.SummaryMaxWords,
	}

	prompt, err := s.prompts.Render("narrator/summary_update.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render summary update template", "error", err)
		return
	}

	newSummary, err := s.llm.GenerateText(ctx, "summary", prompt)
	if err != nil {
		slog.Error("Narrator: Failed to update trip summary", "error", err)
		return
	}

	s.mu.Lock()
	s.tripSummary = strings.TrimSpace(newSummary)
	s.mu.Unlock()

	slog.Info("Narrator: Trip summary updated", "length", len(s.tripSummary))
}

func (s *AIService) getTripSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tripSummary
}

func (s *AIService) addScriptToHistory(qid, title, script string) {
	// Extract the last sentence for flow continuity
	sentences := strings.Split(script, ".")
	var lastSentence string
	for i := len(sentences) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(sentences[i])
		if len(trimmed) > 10 { // Ignore tiny fragments
			lastSentence = trimmed
			break
		}
	}

	s.mu.Lock()
	if lastSentence != "" {
		s.lastScriptEnd = lastSentence
	}
	s.mu.Unlock()

	go s.updateTripSummary(context.Background(), title, script)
}

// ResetSession clears the session history (trip summary, counters, replay memory).
func (s *AIService) ResetSession(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tripSummary = ""
	s.narratedCount = 0
	s.currentPOI = nil
	s.currentTopic = nil
	s.currentEssayTitle = ""
	s.lastPOI = nil
	s.lastEssayTopic = nil
	s.lastEssayTitle = ""
	s.lastScriptEnd = ""

	// Clear active beacons
	if s.beaconSvc != nil {
		s.beaconSvc.Clear()
	}

	// Clear Queue
	s.playbackQueue = make([]*model.Narrative, 0)

	slog.Info("Narrator: Session reset (teleport/new flight detected)")
}
