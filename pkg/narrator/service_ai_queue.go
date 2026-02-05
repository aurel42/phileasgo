package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/announcement"
	"phileasgo/pkg/generation"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
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

// enqueuePlayback adds a narrative to the playback queue via the registered callback.
func (s *AIService) enqueuePlayback(n *model.Narrative, priority bool) {
	if s.onPlayback != nil {
		s.onPlayback(n, priority)
	}
}

// Play enqueues a narrative for playback.
func (s *AIService) Play(n *model.Narrative) {
	s.enqueuePlayback(n, true)
}

// enqueueGeneration adds a generation job to the priority queue.
func (s *AIService) enqueueGeneration(job *generation.Job) {
	s.genQ.Enqueue(job)
}

func (s *AIService) EnqueueAnnouncement(ctx context.Context, a announcement.Item, t *sim.Telemetry, onComplete func(*model.Narrative)) {
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
	s.initAssembler()
	s.mu.Lock()
	if s.generating || s.genQ.Count() == 0 {
		s.mu.Unlock()
		return
	}

	s.generating = true
	job := s.genQ.Pop()
	if job == nil {
		s.generating = false
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	go func() {
		done := false
		defer func() {
			if !done {
				s.releaseGeneration()
			}
		}()

		genCtx := context.Background()
		var req *GenerationRequest

		switch job.Type {
		case model.NarrativeTypePOI:
			req = s.handlePOIJob(genCtx, job)
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
			if job.OnComplete != nil {
				job.OnComplete(nil)
			}
			s.ProcessGenerationQueue(genCtx)
			return
		}

		if job.OnComplete != nil {
			job.OnComplete(narrative)
		} else {
			s.enqueuePlayback(narrative, true)
		}

		s.ProcessGenerationQueue(genCtx)
	}()
}

func (s *AIService) summarizeAndLogEvent(ctx context.Context, n *model.Narrative) {
	s.initAssembler()

	if n.Type == model.NarrativeTypeBorder || n.Type == model.NarrativeTypeLetsgo || n.Type == model.NarrativeTypeDebriefing {
		return
	}

	data := s.promptAssembler.NewPromptData(s.getSessionState())
	data["LastTitle"] = n.Title
	data["LastScript"] = n.Script

	promptBody, err := s.prompts.Render("narrator/event_summary.tmpl", data)
	if err != nil {
		slog.Error("Narrator: Failed to render event summary template", "error", err)
		return
	}

	summary, err := s.llm.GenerateText(ctx, "summary", promptBody)
	if err != nil {
		slog.Error("Narrator: Failed to summarize event", "error", err)
		summary = n.Title
	}

	summary = strings.TrimSpace(summary)
	summary = strings.ReplaceAll(summary, "\n", " ")
	summary = strings.ReplaceAll(summary, "**", "")
	summary = strings.ReplaceAll(summary, "* ", "")

	event := model.TripEvent{
		Timestamp: time.Now(),
		Type:      "narration",
		Category:  n.Type,
		Title:     n.Title,
		Summary:   summary,
		Metadata:  make(map[string]string),
	}

	if n.POI != nil {
		event.Metadata["qid"] = n.POI.WikidataID
		event.Metadata["icon"] = n.POI.Icon
		event.Metadata["poi_lat"] = fmt.Sprintf("%.6f", n.POI.Lat)
		event.Metadata["poi_lon"] = fmt.Sprintf("%.6f", n.POI.Lon)
		event.Metadata["poi_category"] = n.POI.Category
	}

	s.session().AddEvent(&event)
	slog.Debug("Narrator: Trip event logged", "type", n.Type, "title", n.Title)
}

func (s *AIService) RecordNarration(ctx context.Context, n *model.Narrative) {
	s.initAssembler()
	id := n.Title
	if n.POI != nil {
		id = n.POI.WikidataID
	} else if n.EssayTopic != "" {
		id = n.EssayTopic
	}

	s.session().AddNarration(id, n.Title, n.Script)
	go s.summarizeAndLogEvent(ctx, n)
}

func (s *AIService) handlePOIJob(ctx context.Context, job *generation.Job) *GenerationRequest {
	s.initAssembler()
	p, err := s.poiMgr.GetPOI(ctx, job.POIID)
	if err != nil {
		slog.Error("Narrator: Priority job failed - POI not found", "poi_id", job.POIID)
		return nil
	}

	promptData := s.promptAssembler.ForPOI(ctx, p, job.Telemetry, job.Strategy, s.getSessionState())
	promptStr, _ := s.prompts.Render("narrator/script.tmpl", promptData)

	req := &GenerationRequest{
		Type:          model.NarrativeTypePOI,
		Prompt:        promptStr,
		Title:         p.DisplayName(),
		SafeID:        strings.ReplaceAll(p.WikidataID, "/", "_"),
		POI:           p,
		MaxWords:      promptData["MaxWords"].(int),
		Manual:        job.Manual,
		SkipBusyCheck: true,
		ThumbnailURL:  p.ThumbnailURL,
		ShowInfoPanel: true,
		TwoPass:       s.cfg.TwoPassScriptGeneration(ctx),
	}

	s.mu.Lock()
	s.generatingPOI = p
	s.mu.Unlock()

	return req
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

	tmpl := fmt.Sprintf("announcement/%s.tmpl", strings.ToLower(string(job.Type)))
	promptBody, err := s.prompts.Render(tmpl, data)
	if err != nil {
		slog.Error("Narrator: Failed to render announcement prompt", "error", err, "tmpl", tmpl)
		return nil
	}

	maxWords := 300
	switch v := data.(type) {
	case prompt.Data:
		if mw, ok := v["MaxWords"].(int); ok {
			maxWords = mw
		}
	case map[string]any:
		if mw, ok := v["MaxWords"].(int); ok {
			maxWords = mw
		}
	}

	req := &GenerationRequest{
		Type:          job.Type,
		Prompt:        promptBody,
		Title:         job.Announcement.Title(),
		Summary:       job.Announcement.Summary(),
		SafeID:        job.Announcement.ID(),
		MaxWords:      maxWords,
		Manual:        true,
		SkipBusyCheck: true,
		ThumbnailURL:  job.Announcement.ImagePath(),
		POI:           job.Announcement.POI(),
	}

	// Attach full prompt data for two-pass refinement
	switch d := data.(type) {
	case prompt.Data:
		req.PromptData = d
	case map[string]any:
		req.PromptData = prompt.Data(d)
	}

	// For screenshots, set raw path for LLM image analysis and API URL for UI
	if ss, ok := job.Announcement.(*announcement.Screenshot); ok {
		req.ImagePath = ss.RawPath() // Raw path for LLM GenerateImageText
	}

	if job.Type == model.NarrativeTypeScreenshot || job.Type == model.NarrativeTypeDebriefing {
		req.ShowInfoPanel = true
	}

	req.TwoPass = s.cfg.TwoPassScriptGeneration(ctx) && job.Announcement.TwoPass()

	return req
}
