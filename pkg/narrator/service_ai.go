package narrator

import (
	"context"
	"log/slog"
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
