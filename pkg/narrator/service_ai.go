package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"phileasgo/pkg/audio"
	"phileasgo/pkg/config"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
)

// GenerationRequest represents a standardized request to generate a narrative.
type GenerationRequest struct {
	Type   model.NarrativeType
	Prompt string

	// Metadata
	Title       string
	SafeID      string // Used for temporal file naming (not caching)
	AudioFormat string // Default "mp3"

	// Context Objects (passed through to result)
	POI        *model.POI
	ImagePath  string
	EssayTopic *EssayTopic

	// Constraints
	MaxWords int
	Manual   bool
}

// GenerationJob represents a queued request (priority queue).
type GenerationJob struct {
	Type      model.NarrativeType
	POIID     string
	ImagePath string
	Manual    bool
	Strategy  string // e.g., "funny", "historic"
	CreatedAt time.Time
	Telemetry *sim.Telemetry
}

// ScriptEntry represents a previously generated narration script.
type ScriptEntry struct {
	QID    string
	Title  string
	Script string
}

// AIService is the real implementation of the narrator service using LLM and TTS.
type AIService struct {
	cfg           *config.Config
	llm           llm.Provider
	tts           tts.Provider
	prompts       *prompts.Manager
	audio         audio.Service
	poiMgr        POIProvider
	beaconSvc     BeaconProvider
	geoSvc        GeoProvider
	sim           sim.Client
	st            store.Store
	wikipedia     WikipediaProvider
	langRes       LanguageResolver
	tripSummary   string // Added tripSummary field
	lastScriptEnd string // The last sentence of the previous narration for flow continuity

	mu            sync.RWMutex
	running       bool
	active        bool
	generating    bool
	skipCooldown  bool
	narratedCount int
	stats         map[string]any
	latencies     []time.Duration

	// Playback State
	currentPOI        *model.POI
	currentTopic      *EssayTopic
	currentEssayTitle string
	currentImagePath  string // Added field

	// Generation State
	genCancelFunc context.CancelFunc

	// Pending Manual Override (Queued)
	pendingManualID       string
	pendingManualStrategy string

	// Replay State
	lastPOI        *model.POI
	lastEssayTopic *EssayTopic
	lastEssayTitle string
	lastImagePath  string // Added field

	// Staging State (Pipeline)
	priorityGenQueue []*GenerationJob   // Priority queue for generation requests (Manual/Screenshot)
	queue            []*model.Narrative // Playback queue (ready items)
	generatingPOI    *model.POI         // The POI currently being generated (for UI feedback)

	essayH    *EssayHandler
	interests []string
	avoid     []string

	// scriptHistory []ScriptEntry // Removed scriptHistory

	// TTS Fallback State (session-level)
	fallbackTTS     tts.Provider
	useFallbackTTS  bool
	fallbackTracker *tracker.Tracker
}

// NewAIService creates a new AI-powered narrator service.
func NewAIService(
	cfg *config.Config,
	llm llm.Provider,
	tts tts.Provider,
	prompts *prompts.Manager,
	audioMgr audio.Service,
	poiMgr POIProvider,
	beaconSvc BeaconProvider,
	geoSvc GeoProvider,
	simClient sim.Client,
	st store.Store,
	wikipediaClient WikipediaProvider,
	langRes LanguageResolver,
	essayH *EssayHandler,
	interests []string,
	avoid []string,
	tr *tracker.Tracker,
) *AIService {
	s := &AIService{
		cfg:              cfg,
		llm:              llm,
		tts:              tts,
		prompts:          prompts,
		audio:            audioMgr,
		poiMgr:           poiMgr,
		beaconSvc:        beaconSvc,
		geoSvc:           geoSvc,
		sim:              simClient,
		st:               st,
		wikipedia:        wikipediaClient,
		langRes:          langRes,
		stats:            make(map[string]any),
		latencies:        make([]time.Duration, 0, 10),
		essayH:           essayH,
		skipCooldown:     false,
		interests:        interests,
		avoid:            avoid,
		fallbackTracker:  tr,
		tripSummary:      "",                          // Initialize tripSummary
		queue:            make([]*model.Narrative, 0), // Initialize playback queue
		priorityGenQueue: make([]*GenerationJob, 0),   // Initialize priority queue
	}
	// Initial default window
	s.sim.SetPredictionWindow(60 * time.Second)
	return s
}

// Start starts the narrator service.
func (s *AIService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	slog.Info("AI Narrator service started")
}

// Stop stops the narrator service.
func (s *AIService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.audio.Shutdown()
	slog.Info("AI Narrator service stopped")
}

// IsActive returns true if narrator is currently active (generating or playing).

// GenerateNarrative creates a narrative from a standardized request.
// It handles:
// 1. Concurrency checks (IsGenerating)
// 2. Cancellation Context
// 3. LLM Generation
// 4. Script Rescue
// 5. TTS Synthesis
// 6. Narrative Construction
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
	// We assume req.Prompt is already fully rendered by the caller
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
	// Use provided SafeID or a fallback
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
				// Remove title line from script to avoid speaking it twice?
				// Legacy behavior did this. Let's do it if we extracted it.
				n.Script = strings.Join(lines[1:], "\n")
			}
		}
	}

	return n, nil
}

// POIManager returns the internal POI manager.
func (s *AIService) POIManager() POIProvider {
	return s.poiMgr
}

// LLMProvider returns the internal LLM provider.
func (s *AIService) LLMProvider() llm.Provider {
	return s.llm
}

// AudioService returns the internal audio service.
func (s *AIService) AudioService() audio.Service {
	return s.audio
}
