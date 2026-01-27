package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
)

// HasPendingManualOverride returns true if a user-requested POI is queued.
func (s *AIService) HasPendingManualOverride() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.genQ.Count() > 0 || s.pendingManualID != ""
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
	s.playbackQ.Enqueue(n, priority)
}

// popPlaybackQueue retrieves and removes the next narrative from the queue.
func (s *AIService) popPlaybackQueue() *model.Narrative {
	return s.playbackQ.Pop()
}

// peekPlaybackQueue returns the head of the queue without removing it.
func (s *AIService) peekPlaybackQueue() *model.Narrative {
	return s.playbackQ.Peek()
}

func (s *AIService) canEnqueuePlayback(nType model.NarrativeType, manual bool) bool {
	return s.playbackQ.CanEnqueue(nType, manual)
}

// enqueueGeneration adds a generation job to the priority queue.
func (s *AIService) enqueueGeneration(job *generation.Job) {
	s.genQ.Enqueue(job)
}

// HasPendingGeneration returns true if there are items in the priority generation queue.
func (s *AIService) HasPendingGeneration() bool {
	return s.genQ.HasPending()
}

func (s *AIService) ProcessGenerationQueue(ctx context.Context) {
	s.mu.Lock()
	// Serialization: Only one generation at a time.
	// If already generating, the current GenerateNarrative will kick this again on finish.
	if s.generating || s.genQ.Count() == 0 {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// Peek first to get job info, then Pop inside the goroutine?
	// Or Pop now? If we Pop now, we must ensure processing happens.
	// But ProcessGenerationQueue is async logic wrapper.
	// Let's rely on Manager.
	job := s.genQ.Pop()
	if job == nil {
		return
	}

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
				Lat:       job.Telemetry.Latitude,
				Lon:       job.Telemetry.Longitude,
				MaxWords:  s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthShortWords),
				Manual:    true,
			}

		case model.NarrativeTypeBorder:
			data := struct {
				TourGuideName   string
				Persona         string
				Accent          string
				From            string
				To              string
				MaxWords        int
				Language_name   string
				Language_code   string
				TripSummary     string
				TTSInstructions string
			}{
				TourGuideName: "Ava", // TODO: Config
				Persona:       "Intelligent, fascinating",
				Accent:        "Neutral",
				From:          job.From,
				To:            job.To,
				MaxWords:      30, // Short statement
				Language_name: "English",
				Language_code: "en-US",
				TripSummary:   s.getTripSummary(),
			}
			data.TTSInstructions = s.fetchTTSInstructions(data)
			prompt, err := s.prompts.Render("narrator/border.tmpl", data)
			if err != nil {
				slog.Error("Narrator: Failed to render border prompt", "error", err)
				return
			}
			req = &GenerationRequest{
				Type:     model.NarrativeTypeBorder,
				Prompt:   prompt,
				Title:    "Border Crossing",
				SafeID:   "border_" + time.Now().Format("150405"),
				MaxWords: data.MaxWords,
				Manual:   true,
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
			s.enqueueGeneration(&generation.Job{
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
	data := s.getCommonPromptData()
	data.CurrentSummary = currentSummary
	data.LastTitle = lastTitle
	data.LastScript = lastScript
	data.MaxWords = s.cfg.Narrator.SummaryMaxWords

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

	slog.Debug("Narrator: Trip summary updated", "length", len(s.tripSummary))
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
	s.playbackQ.Clear()

	slog.Info("Narrator: Session reset (teleport/new flight detected)")
}
