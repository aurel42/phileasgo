package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/audio"
	"phileasgo/pkg/model"
	"phileasgo/pkg/request"
)

// GenerateNarrative creates a narrative from a standardized request.
func (s *AIService) GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error) {
	s.initAssembler()
	// 1. Sync State Check
	if err := s.handleGenerationState(req); err != nil {
		return nil, err
	}
	startTime := time.Now()
	predicted := s.AverageLatency()

	// Defer Cleanup
	defer func() {
		actual := time.Since(startTime)
		s.updateLatency(actual)
		s.mu.Lock()
		s.generating = false
		s.generatingPOI = nil
		s.mu.Unlock()
	}()

	// 2. Set Active POI (if applicable) for UI feedback
	if req.POI != nil {
		s.mu.Lock()
		s.generatingPOI = req.POI
		s.mu.Unlock()
	}

	// PHASE 2: Improved logging for Wikipedia comparison
	s.logWikipediaContext(req)

	// 3. Generate Script (LLM)
	resp, err := s.generateInitialScript(ctx, req)
	if err != nil {
		return nil, err
	}

	script := resp.Script
	extractedTitle := resp.Title

	// 4. Second Pass Refinement (if enabled)
	if req.TwoPass {
		script = s.performSecondPass(ctx, req, script)
	} else {
		// 5. Rescue Script (if too long) - Mutually exclusive with 2-pass
		script = s.performRescueIfNeeded(ctx, req, script)
	}

	// 5. TTS Synthesis (with retries)
	safeID := req.SafeID
	if safeID == "" {
		safeID = "gen_" + time.Now().Format("150405")
	}

	var audioPath, format string
	var synthErr error

	for attempt := 1; attempt <= 3; attempt++ {
		audioPath, format, synthErr = s.synthesizeAudio(ctx, script, safeID)
		if synthErr == nil {
			break
		}

		slog.Warn("Narrator: TTS synthesis attempt failed",
			"attempt", attempt,
			"error", synthErr,
			"poi", req.Title,
		)

		if attempt < 3 {
			// Fast retry for transient issues
			time.Sleep(200 * time.Millisecond)
		}
	}

	if synthErr != nil {
		s.handleTTSError(synthErr)
		return nil, fmt.Errorf("TTS synthesis failed after retries: %w", synthErr)
	}

	// 6. Get Audio Duration
	duration, _ := audio.GetDuration(audioPath)

	return s.constructNarrative(req, script, extractedTitle, audioPath, format, startTime, predicted, duration), nil
}

func (s *AIService) logWikipediaContext(req *GenerationRequest) {
	if req.POI != nil && req.POI.WPURL != "" {
		slog.Debug("Narrator: Generation context (WP)",
			"url", req.POI.WPURL,
			"approx_words", len(strings.Fields(req.Prompt)),
		)
	}
}

func (s *AIService) generateInitialScript(ctx context.Context, req *GenerationRequest) (model.GenerationResponse, error) {
	profile := string(req.Type)
	switch req.Type {
	case model.NarrativeTypePOI:
		profile = "narration"
	case model.NarrativeTypeLetsgo, model.NarrativeTypeBriefing:
		// New Announcements: check for specific profile, then fallback to shared 'announcements'
		if !s.llm.HasProfile(profile) {
			profile = "announcements"
		}
	}

	if req.ImagePath != "" {
		var res model.GenerationResponse
		err := s.llm.GenerateImageJSON(ctx, profile, req.Prompt, req.ImagePath, &res)
		if err != nil {
			slog.Warn("Narrator: LLM image generation (JSON) failed, falling back to text-only", "error", err)
			script, err := s.llm.GenerateImageText(ctx, profile, req.Prompt, req.ImagePath)
			if err != nil {
				return model.GenerationResponse{}, fmt.Errorf("LLM image generation (text) failed: %w", err)
			}
			return model.GenerationResponse{Script: strings.ReplaceAll(script, "*", "")}, nil
		}
		return res, nil
	}

	var resp model.GenerationResponse
	err := s.llm.GenerateJSON(ctx, profile, req.Prompt, &resp)
	if err != nil {
		return model.GenerationResponse{}, fmt.Errorf("LLM generation failed: %w", err)
	}
	return resp, nil
}

func (s *AIService) performRescueIfNeeded(ctx context.Context, req *GenerationRequest, script string) string {
	if req.MaxWords <= 0 {
		return script
	}

	wordCount := len(strings.Fields(script))
	limit := int(float64(req.MaxWords) * 1.30) // 30% Buffer
	if wordCount <= limit {
		return script
	}

	slog.Warn("Narrator: Script exceeded limit, attempting rescue", "requested", req.MaxWords, "actual", wordCount)

	// Attempt 1
	rescued, err := s.rescueScript(ctx, script, req.MaxWords)
	if err == nil && !s.isGarbage(req, script, rescued.Script) {
		slog.Info("Narrator: Script rescue successful", "original", wordCount, "rescued", len(strings.Fields(rescued.Script)))
		return s.finalizeRescuedScript(req, rescued)
	}

	// Retry once with excluded provider
	if provider, ok := ctx.Value(request.CtxProviderLabel).(string); ok && provider != "" {
		slog.Warn("Narrator: Rescue failed or produced garbage, retrying with next provider", "excluded", provider)
		retryCtx := context.WithValue(ctx, request.CtxExcludedProviders, []string{provider})
		rescued, err = s.rescueScript(retryCtx, script, req.MaxWords)
		if err == nil && !s.isGarbage(req, script, rescued.Script) {
			slog.Info("Narrator: Script rescue successful (retry)", "original", wordCount, "rescued", len(strings.Fields(rescued.Script)))
			return s.finalizeRescuedScript(req, rescued)
		}
	}

	slog.Error("Narrator: Script rescue failed consistently, using original", "error", err)
	return script
}

func (s *AIService) finalizeRescuedScript(req *GenerationRequest, rescued model.GenerationResponse) string {
	if rescued.Title != "" {
		req.Title = rescued.Title
	}
	return rescued.Script
}

func (s *AIService) constructNarrative(req *GenerationRequest, script, extractedTitle, audioPath, format string, startTime time.Time, predicted, duration time.Duration) *model.Narrative {
	finalTitle := req.Title
	if finalTitle == "" {
		finalTitle = extractedTitle
	}
	// For essays, prefer the LLM-generated title over the generic topic name
	if req.Type == model.NarrativeTypeEssay && extractedTitle != "" {
		finalTitle = extractedTitle
	}

	// For screenshots, we preserve the raw path in ImagePath
	imagePath := req.ImagePath

	n := &model.Narrative{
		Type:           req.Type,
		Title:          finalTitle,
		Script:         script,
		AudioPath:      audioPath,
		Format:         format,
		RequestedWords: req.MaxWords,
		Manual:         req.Manual,
		CreatedAt:      time.Now(),

		GenerationLatency: time.Since(startTime),
		PredictedLatency:  predicted,
		Duration:          duration,

		// Context passthrough
		POI:           req.POI,
		ImagePath:     imagePath,
		ThumbnailURL:  req.ThumbnailURL,
		Summary:       req.Summary,
		Lat:           req.Lat,
		Lon:           req.Lon,
		ShowInfoPanel: req.ShowInfoPanel,
	}
	if req.EssayTopic != nil {
		n.EssayTopic = req.EssayTopic.Name
		n.EssayIcon = req.EssayTopic.Icon
	}
	return n
}

// rescueScript attempts to extract a clean script from contaminated LLM output.
func (s *AIService) rescueScript(ctx context.Context, script string, maxWords int) (model.GenerationResponse, error) {
	s.initAssembler()
	// Fetch TTS instructions for consistent formatting during rescue
	pd := s.promptAssembler.NewPromptData(s.getSessionState())
	ttsInstr, _ := pd["TTSInstructions"].(string)

	prompt, err := s.prompts.Render("context/rescue_script.tmpl", map[string]any{
		"Script":          script,
		"MaxWords":        int(float64(maxWords) * 1.5), // Give rescue 50% more headroom
		"TTSInstructions": ttsInstr,
	})
	if err != nil {
		return model.GenerationResponse{}, fmt.Errorf("failed to render rescue prompt: %w", err)
	}

	// Use script_rescue profile (cheap model)
	var resp model.GenerationResponse
	err = s.llm.GenerateJSON(ctx, "script_rescue", prompt, &resp)
	if err != nil {
		return model.GenerationResponse{}, fmt.Errorf("rescue LLM call failed: %w", err)
	}

	return resp, nil
}

func (s *AIService) performSecondPass(ctx context.Context, req *GenerationRequest, script string) string {
	s.initAssembler()

	pd := req.PromptData
	if pd == nil {
		// Fallback to basic data if missing
		pd = s.promptAssembler.NewPromptData(s.getSessionState())
	}

	// Update with second-pass context
	pd["Script"] = script
	pd["MaxWords"] = int(float64(req.MaxWords) * 1.2)
	pd["NarrativeType"] = string(req.Type)

	promptStr, err := s.prompts.Render("context/second_pass.tmpl", pd)
	if err != nil {
		slog.Error("Narrator: Failed to render second-pass prompt", "error", err)
		return script
	}

	// Attempt 1
	var resp model.GenerationResponse
	err = s.llm.GenerateJSON(ctx, "script_rescue", promptStr, &resp)
	if err == nil && !s.isGarbage(req, script, resp.Script) {
		slog.Info("Narrator: Second-pass refinement successful")
		return strings.TrimSpace(resp.Script)
	}

	// Retry once with excluded provider
	if provider, ok := ctx.Value(request.CtxProviderLabel).(string); ok && provider != "" {
		slog.Warn("Narrator: Second-pass failed or produced garbage, retrying with next provider", "excluded", provider)
		retryCtx := context.WithValue(ctx, request.CtxExcludedProviders, []string{provider})
		err = s.llm.GenerateJSON(retryCtx, "script_rescue", promptStr, &resp)
		if err == nil && !s.isGarbage(req, script, resp.Script) {
			slog.Info("Narrator: Second-pass refinement successful (retry)")
			return strings.TrimSpace(resp.Script)
		}
	}

	slog.Warn("Narrator: Second-pass refinement failed consistently, using original script")
	return script
}

func (s *AIService) isGarbage(req *GenerationRequest, input, output string) bool {
	if output == "" {
		return true
	}
	if strings.TrimSpace(output) == "RESCUE_FAILED" {
		return true
	}

	outWords := len(strings.Fields(output))
	inWords := len(strings.Fields(input))
	targetWords := req.MaxWords

	// Garbage detection threshold:
	// We use two signals: the requested target and the actual input size.
	// Output should not exceed target * 2 AND not exceed input * 1.5.
	// We take the MINIMUM/STRICTER of these two if target is specified.
	threshold := float64(inWords) * 1.5

	if targetWords > 0 {
		targetThreshold := float64(targetWords) * 2.0
		if targetThreshold < threshold {
			threshold = targetThreshold
		}
	}

	if float64(outWords) > threshold {
		// Resilience: If refinement significantly reduced word count (e.g. by >50%),
		// and the input was already way over target, we accept it as a successful step
		// towards the goal, even if it's still over the absolute threshold.
		// This prevents catastrophic fallbacks to contaminated originals.
		if targetWords > 0 && float64(inWords) > float64(targetWords)*2.0 && outWords < inWords/2 {
			slog.Debug("Narrator: Accepting verbose output due to significant reduction",
				"in", inWords,
				"out", outWords,
				"target", targetWords,
			)
			return false
		}

		slog.Warn("Narrator: LLM produced probable garbage output",
			"words", outWords,
			"threshold", int(threshold),
			"target", targetWords,
			"input_words", inWords,
		)
		return true
	}

	return false
}
