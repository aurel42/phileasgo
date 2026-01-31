package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/announcement"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/sim"
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

// Play enqueues a narrative for playback and triggers the playback queue processing.
func (s *AIService) Play(n *model.Narrative) {
	s.enqueuePlayback(n, true)
	go s.ProcessPlaybackQueue(context.Background())
}

// popPlaybackQueue retrieves and removes the next narrative from the queue.
func (s *AIService) popPlaybackQueue() *model.Narrative {
	return s.playbackQ.Pop()
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

func (s *AIService) EnqueueAnnouncement(ctx context.Context, a announcement.Announcement, t *sim.Telemetry, onComplete func(*model.Narrative)) {
	s.enqueueGeneration(&generation.Job{
		Type:         a.Type(),
		Telemetry:    t,
		Announcement: a,
		CreatedAt:    time.Now(),
		OnComplete:   onComplete,
	})

	// Trigger processing
	go s.ProcessGenerationQueue(ctx)
}

func (s *AIService) ProcessGenerationQueue(ctx context.Context) {
	s.mu.Lock()
	// Serialization: Only one generation at a time.
	if s.generating || s.genQ.Count() == 0 {
		s.mu.Unlock()
		return
	}

	// 1. Claim state EARLY to prevent other triggers from starting pregrounding
	s.generating = true

	// 2. Pop the job
	job := s.genQ.Pop()
	if job == nil {
		s.generating = false
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// Process Job
	go func() {
		// Safety: ensure we always release the lock if anything in this goroutine fails
		// before GenerateNarrative (which has its own defer).
		done := false
		defer func() {
			if !done {
				s.mu.Lock()
				s.generating = false
				s.generatingPOI = nil
				s.mu.Unlock()
			}
		}()

		genCtx := context.Background()
		var req *GenerationRequest

		switch job.Type {
		case model.NarrativeTypePOI:
			req = s.handlePOIJob(genCtx, job)
		case model.NarrativeTypeScreenshot:
			req = s.handleScreenshotJob(genCtx, job)
		case model.NarrativeTypeBorder:
			req = s.handleBorderJob(genCtx, job)
		default:
			req = s.handleAnnouncementJob(genCtx, job)
		}

		if req == nil {
			return
		}

		done = true
		narrative, err := s.GenerateNarrative(genCtx, req)
		if err != nil {
			slog.Error("Narrator: Priority generation failed", "type", job.Type, "error", err)
			// Trigger next even on failure
			s.ProcessGenerationQueue(genCtx)
			return
		}

		// Handle Result
		if job.OnComplete != nil {
			job.OnComplete(narrative)
		} else {
			// Fallback: Default Playback
			s.enqueuePlayback(narrative, true)
			go s.ProcessPlaybackQueue(genCtx)
		}

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
	data["CurrentSummary"] = currentSummary
	data["LastTitle"] = lastTitle
	data["LastScript"] = lastScript
	data["MaxWords"] = s.cfg.Narrator.SummaryMaxWords

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

func (s *AIService) handlePOIJob(ctx context.Context, job *generation.Job) *GenerationRequest {
	p, err := s.poiMgr.GetPOI(ctx, job.POIID)
	if err != nil {
		slog.Error("Narrator: Priority job failed - POI not found", "poi_id", job.POIID)
		return nil
	}

	promptData := s.buildPromptData(ctx, p, job.Telemetry, job.Strategy)
	prompt, _ := s.prompts.Render("narrator/script.tmpl", promptData)

	req := &GenerationRequest{
		Type:          model.NarrativeTypePOI,
		Prompt:        prompt,
		Title:         p.DisplayName(),
		SafeID:        strings.ReplaceAll(p.WikidataID, "/", "_"),
		POI:           p,
		MaxWords:      promptData["MaxWords"].(int),
		Manual:        job.Manual,
		SkipBusyCheck: true,
		ThumbnailURL:  p.ThumbnailURL,
	}

	// Update state so IsPOIBusy(p.WikidataID) is true during generation
	s.mu.Lock()
	s.generatingPOI = p
	s.mu.Unlock()

	return req
}

func (s *AIService) handleScreenshotJob(ctx context.Context, job *generation.Job) *GenerationRequest {
	loc := s.geoSvc.GetLocation(job.Telemetry.Latitude, job.Telemetry.Longitude)
	data := s.getCommonPromptData()
	data["City"] = loc.CityName
	data["Region"] = loc.Admin1Name
	data["Country"] = loc.CountryCode
	data["MaxWords"] = s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthShortWords)
	data["TripSummary"] = s.getTripSummary()
	data["Lat"] = fmt.Sprintf("%.3f", job.Telemetry.Latitude)
	data["Lon"] = fmt.Sprintf("%.3f", job.Telemetry.Longitude)
	data["Alt"] = fmt.Sprintf("%.0f", job.Telemetry.AltitudeAGL)
	prompt, err := s.prompts.Render("narrator/screenshot.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render screenshot prompt", "error", err)
		return nil
	}

	return &GenerationRequest{
		Type:          model.NarrativeTypeScreenshot,
		Prompt:        prompt,
		Title:         "Screenshot Analysis",
		SafeID:        "screenshot_" + time.Now().Format("150405"),
		ImagePath:     job.ImagePath,
		Lat:           job.Telemetry.Latitude,
		Lon:           job.Telemetry.Longitude,
		MaxWords:      s.applyWordLengthMultiplier(s.cfg.Narrator.NarrationLengthShortWords),
		Manual:        true,
		SkipBusyCheck: true,
		ThumbnailURL:  "/api/images/serve?path=" + job.ImagePath,
	}
}

func (s *AIService) handleBorderJob(ctx context.Context, job *generation.Job) *GenerationRequest {
	data := s.getCommonPromptData()
	data["From"] = job.From
	data["To"] = job.To
	data["MaxWords"] = 30 // Short statement
	data["TTSInstructions"] = s.fetchTTSInstructions(data)
	prompt, err := s.prompts.Render("narrator/border.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render border prompt", "error", err)
		return nil
	}
	return &GenerationRequest{
		Type:          model.NarrativeTypeBorder,
		Prompt:        prompt,
		Title:         "Border Crossing",
		SafeID:        "border_" + time.Now().Format("150405"),
		MaxWords:      data["MaxWords"].(int),
		Manual:        true,
		SkipBusyCheck: true,
	}
}

func (s *AIService) handleAnnouncementJob(ctx context.Context, job *generation.Job) *GenerationRequest {
	if job.Announcement == nil {
		slog.Warn("Narrator: ProcessPriorityQueue received unsupported job type", "type", job.Type)
		return nil
	}

	data, err := job.Announcement.GetPromptData(job.Telemetry)
	if err != nil {
		slog.Error("Narrator: Failed to get announcement data", "error", err)
		return nil
	}

	// Find prompt template based on narrative type
	tmpl := fmt.Sprintf("announcement/%s.tmpl", strings.ToLower(string(job.Type)))
	prompt, err := s.prompts.Render(tmpl, data)
	if err != nil {
		slog.Error("Narrator: Failed to render announcement prompt", "error", err, "tmpl", tmpl)
		return nil
	}

	return &GenerationRequest{
		Type:          job.Type,
		Prompt:        prompt,
		Title:         job.Announcement.Title(),
		Summary:       job.Announcement.Summary(),
		SafeID:        job.Announcement.ID(),
		MaxWords:      300,
		Manual:        true, // Announcements are treated with same priority as manual
		SkipBusyCheck: true,
		ThumbnailURL:  job.Announcement.ImagePath(),
		POI:           job.Announcement.POI(),
	}
}
