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
	// 1. Sync State Check
	if err := s.handleGenerationState(req.Manual); err != nil {
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
	script, err := s.generateInitialScript(ctx, req)
	if err != nil {
		return nil, err
	}

	// 3a. Extract Metadata (Title) - BEFORE Rescue/TTS
	extractedTitle, script := s.extractTitleFromScript(script)

	// 4. Rescue Script (if too long)
	script = s.performRescueIfNeeded(ctx, req, script)

	// 5. TTS Synthesis
	safeID := req.SafeID
	if safeID == "" {
		safeID = "gen_" + time.Now().Format("150405")
	}

	audioPath, format, err := s.synthesizeAudio(ctx, script, safeID)
	if err != nil {
		s.handleTTSError(err)
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	return s.constructNarrative(req, script, extractedTitle, audioPath, format, startTime, predicted), nil
}

func (s *AIService) logWikipediaContext(req *GenerationRequest) {
	if req.POI != nil && req.POI.WPURL != "" {
		slog.Debug("Narrator: Generation context (WP)",
			"url", req.POI.WPURL,
			"approx_words", len(strings.Fields(req.Prompt)),
		)
		// Detailed prose logging for comparison (wrapped in logs)
		fmt.Printf("\n--- [WP PROSE START] ---\n%s\n--- [WP PROSE END] ---\n\n", req.Prompt)
	}
}

func (s *AIService) generateInitialScript(ctx context.Context, req *GenerationRequest) (string, error) {
	profile := string(req.Type)
	if req.Type == model.NarrativeTypePOI {
		profile = "narration"
	}

	if req.ImagePath != "" {
		script, err := s.llm.GenerateImageText(ctx, profile, req.Prompt, req.ImagePath)
		if err != nil {
			return "", fmt.Errorf("LLM image generation failed: %w", err)
		}
		return strings.ReplaceAll(script, "*", ""), nil
	}

	script, err := s.generateScript(ctx, profile, req.Prompt)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}
	return script, nil
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
	rescued, err := s.rescueScript(ctx, script, req.MaxWords)
	if err != nil {
		slog.Error("Narrator: Script rescue failed, using original", "error", err)
		return script
	}

	// Re-extract TITLE from rescued script if needed
	resTitle, resScript := s.extractTitleFromScript(rescued)
	if resTitle != "" {
		req.Title = resTitle
		script = resScript
	} else {
		script = rescued
	}
	slog.Info("Narrator: Script rescue successful", "original", wordCount, "rescued", len(strings.Fields(script)))
	return script
}

func (s *AIService) constructNarrative(req *GenerationRequest, script, extractedTitle, audioPath, format string, startTime time.Time, predicted time.Duration) *model.Narrative {
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
		CreatedAt:      time.Now(),

		GenerationLatency: time.Since(startTime),
		PredictedLatency:  predicted,

		// Context passthrough
		POI:       req.POI,
		ImagePath: req.ImagePath,
	}
	if req.EssayTopic != nil {
		n.EssayTopic = req.EssayTopic.Name
	}
	return n
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
		"MaxWords": int(float64(maxWords) * 1.5), // Give rescue 50% more headroom
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
