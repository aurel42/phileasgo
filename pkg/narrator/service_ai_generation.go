package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"phileasgo/pkg/model"
)

// GenerateNarrative creates a narrative from a standardized request.
func (s *AIService) GenerateNarrative(ctx context.Context, req *GenerationRequest) (*model.Narrative, error) {
	// 1. Sync State Check & Cancellation
	if err := s.handleGenerationState(req.Manual); err != nil {
		return nil, err
	}

	// Create cancellable context
	genCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.genCancelFunc = cancel
	s.mu.Unlock()

	startTime := time.Now()

	// Defer Cleanup
	defer func() {
		s.updateLatency(time.Since(startTime))
		s.mu.Lock()
		s.generating = false
		s.genCancelFunc = nil
		s.generatingPOI = nil
		s.mu.Unlock()
		cancel()
	}()

	// 2. Set Active POI (if applicable) for UI feedback
	if req.POI != nil {
		s.mu.Lock()
		s.generatingPOI = req.POI
		s.mu.Unlock()
	}

	// 3. Generate Script (LLM)
	script, err := s.generateScript(genCtx, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// 4. Rescue Script (if too long)
	if req.MaxWords > 0 {
		wordCount := len(strings.Fields(script))
		limit := req.MaxWords + 100 // Buffer
		if wordCount > limit {
			slog.Warn("Narrator: Script exceeded limit, attempting rescue", "requested", req.MaxWords, "actual", wordCount)
			rescued, err := s.rescueScript(genCtx, script, req.MaxWords)
			if err == nil {
				script = rescued
				slog.Info("Narrator: Script rescue successful")
			} else {
				slog.Error("Narrator: Script rescue failed, using original", "error", err)
			}
		}
	}

	// 5. TTS Synthesis
	safeID := req.SafeID
	if safeID == "" {
		safeID = "gen_" + time.Now().Format("150405")
	}

	audioPath, format, err := s.synthesizeAudio(genCtx, script, safeID)
	if err != nil {
		s.handleTTSError(err)
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	// 6. Construct Narrative
	n := &model.Narrative{
		Type:           req.Type,
		Title:          req.Title,
		Script:         script,
		AudioPath:      audioPath,
		Format:         format,
		RequestedWords: req.MaxWords,
		Manual:         req.Manual,

		// Context passthrough
		POI:       req.POI,
		ImagePath: req.ImagePath,
	}
	if req.EssayTopic != nil {
		n.EssayTopic = req.EssayTopic.Name
	}

	// If Title was missing, try to extract from script (common for Essays)
	if n.Title == "" {
		lines := strings.Split(script, "\n")
		if len(lines) > 0 {
			first := strings.TrimSpace(lines[0])
			if strings.HasPrefix(first, "TITLE:") {
				n.Title = strings.TrimSpace(strings.TrimPrefix(first, "TITLE:"))
				n.Script = strings.Join(lines[1:], "\n")
			}
		}
	}

	return n, nil
}

// rescueScript attempts to extract a clean script from contaminated LLM output.
func (s *AIService) rescueScript(ctx context.Context, script string, maxWords int) (string, error) {
	prompt, err := s.prompts.Render("context/rescue_script.tmpl", map[string]any{
		"Script":   script,
		"MaxWords": maxWords,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render rescue prompt: %w", err)
	}

	// Use script_rescue profile (cheap model)
	rescued, err := s.llm.GenerateText(ctx, "script_rescue", prompt)
	if err != nil {
		return "", fmt.Errorf("rescue LLM call failed: %w", err)
	}

	// Check for explicit failure signal
	if strings.TrimSpace(rescued) == "RESCUE_FAILED" {
		return "", fmt.Errorf("LLM could not extract valid script")
	}

	return strings.TrimSpace(rescued), nil
}
