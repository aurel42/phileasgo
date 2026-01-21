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
	var script string
	var err error
	if req.ImagePath != "" {
		// Multimodal: send prompt + image
		script, err = s.llm.GenerateImageText(genCtx, "narration", req.Prompt, req.ImagePath)
		if err != nil {
			return nil, fmt.Errorf("LLM image generation failed: %w", err)
		}
		// Filter markdown artifacts
		script = strings.ReplaceAll(script, "*", "")
	} else {
		// Text-only generation
		script, err = s.generateScript(genCtx, req.Prompt)
		if err != nil {
			return nil, fmt.Errorf("LLM generation failed: %w", err)
		}
	}

	// 3a. Extract Metadata (Title) - BEFORE Rescue/TTS
	// We do this early so the TITLE line is not counted in word limits or read by TTS.
	var extractedTitle string
	extractedTitle, script = s.extractTitleFromScript(script)

	// 4. Rescue Script (if too long)
	if req.MaxWords > 0 {
		wordCount := len(strings.Fields(script))
		limit := int(float64(req.MaxWords) * 1.30) // 30% Buffer
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
	finalTitle := req.Title
	if finalTitle == "" {
		finalTitle = extractedTitle
	}

	n := &model.Narrative{
		Type:           req.Type,
		Title:          finalTitle,
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

	return n, nil
}

// extractTitleFromScript parses the "TITLE:" line from the script if present.
// Returns the extracted title and the cleaned script (without the title line).
func (s *AIService) extractTitleFromScript(script string) (title, cleanScript string) {
	var extractedTitle string
	lines := strings.Split(script, "\n")
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])

		// Regex to match "TITLE:" with optional markdown asterisks and flexible spacing
		// ^[\*]*TITLE\s*:\s*(.*)
		// We process manually to avoid heavy regex compilation every time if desired,
		// but regex is cleaner for this complexity.
		// Let's use simple string manipulation for performance/safety without new imports if possible,
		// or just strip specific prefixes.

		upper := strings.ToUpper(first)
		// Remove markdown bold/italic markers from start
		cleanFirst := strings.TrimLeft(upper, "*_")

		if strings.HasPrefix(cleanFirst, "TITLE") {
			// Find the colon
			idx := strings.Index(first, ":")
			if idx != -1 && idx < 10 { // Colon must be near start
				extractedTitle = strings.TrimSpace(first[idx+1:])
				extractedTitle = strings.Trim(extractedTitle, "*_") // Remove trailing markdown
				extractedTitle = strings.TrimSpace(extractedTitle)  // Remove any spaces that were inside markers

				// Remove the title line
				if len(lines) > 1 {
					script = strings.Join(lines[1:], "\n")
				} else {
					script = ""
				}
			}
		}
	}
	return extractedTitle, strings.TrimSpace(script)
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
