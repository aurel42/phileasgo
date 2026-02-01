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
	"phileasgo/pkg/narrator/generation"
	"phileasgo/pkg/narrator/playback"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/session"
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

	// Location Context (Snapshot)
	Lat float64
	Lon float64

	// Constraints
	MaxWords      int
	Manual        bool
	SkipBusyCheck bool // If true, handleGenerationState will skip the busy check (assumes caller claimed it)

	ThumbnailURL string // Presentation metadata
	Summary      string // User-visible summary
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
	categoriesCfg *config.CategoriesConfig // For pregrounding checks
	sessionMgr    *session.Manager

	mu           sync.RWMutex
	running      bool
	active       bool
	generating   bool
	skipCooldown bool
	stats        map[string]any
	latencies    []time.Duration

	// Configuration
	pacingDuration time.Duration

	// Playback State
	currentPOI          *model.POI
	currentTopic        *EssayTopic
	currentEssayTitle   string
	currentType         model.NarrativeType // The type of the currently playing narrative
	currentImagePath    string              // Added field
	currentThumbnailURL string              // Primary UI driver
	currentLat          float64             // Location snapshot
	currentLon          float64             // Location snapshot

	// Generation State
	generatingTitle     string
	generatingThumbnail string

	// Pending Manual Override (Queued)
	pendingManualID       string
	pendingManualStrategy string

	// Infrastructure
	promptAssembler *prompt.Assembler

	// Replay State
	lastPOI        *model.POI
	lastEssayTopic *EssayTopic
	lastEssayTitle string
	lastImagePath  string // Added field
	lastLat        float64
	lastLon        float64

	// Staging State (Pipeline)
	genQ          *generation.Manager // Generation queue manager
	playbackQ     *playback.Manager   // Playback queue manager
	generatingPOI *model.POI          // The POI currently being generated (for UI feedback)

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
	categoriesCfg *config.CategoriesConfig,
	essayH *EssayHandler,
	interests []string,
	avoid []string,
	tr *tracker.Tracker,
	sessMgr *session.Manager,
) *AIService {
	s := &AIService{
		cfg:             cfg,
		llm:             llm,
		tts:             tts,
		prompts:         prompts,
		audio:           audioMgr,
		poiMgr:          poiMgr,
		beaconSvc:       beaconSvc,
		geoSvc:          geoSvc,
		sim:             simClient,
		st:              st,
		wikipedia:       wikipediaClient,
		langRes:         langRes,
		categoriesCfg:   categoriesCfg,
		stats:           make(map[string]any),
		latencies:       make([]time.Duration, 0, 10),
		essayH:          essayH,
		skipCooldown:    false,
		interests:       interests,
		avoid:           avoid,
		fallbackTracker: tr,
		sessionMgr:      sessMgr,
		playbackQ:       playback.NewManager(), // Initialize playback queue
		genQ:            generation.NewManager(),
		pacingDuration:  3 * time.Second,
	}
	// Initial default window
	s.sim.SetPredictionWindow(60 * time.Second)

	s.promptAssembler = prompt.NewAssembler(
		cfg,
		st,
		prompts,
		geoSvc,
		wikipediaClient,
		poiMgr,
		llm,
		categoriesCfg,
		langRes,
	)

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

// Pause pauses the narration playback.
func (s *AIService) Pause() {
	s.audio.SetUserPaused(true)
	s.audio.Pause()
}

// NarratedCount returns the number of narratives generated in the current session.
func (s *AIService) NarratedCount() int {
	return s.session().NarratedCount()
}

// GetTripSummary returns the combined summary of all POIs visited in the current session.
func (s *AIService) GetTripSummary() string {
	return s.session().GetState().TripSummary
}

// GetLastTransition returns the timestamp of the last transition to the given stage.
func (s *AIService) GetLastTransition(stage string) time.Time {
	if s.sim == nil {
		return time.Time{}
	}
	return s.sim.GetLastTransition(stage)
}

// Resume resumes the narration playback.
func (s *AIService) Resume() {
	s.audio.ResetUserPause()
	s.audio.Resume()
}

// Skip skips the current narration.
func (s *AIService) Skip() {
	slog.Info("Narrator: skipping current narration")
	s.audio.Stop()
}

// TriggerIdentAction triggers the action configured for the transponder Ident button.
func (s *AIService) TriggerIdentAction() {
	s.mu.RLock()
	action := s.cfg.Transponder.IdentAction
	s.mu.RUnlock()

	slog.Info("Transponder: IDENT triggered", "action", action)

	switch action {
	case "pause_toggle":
		if s.audio.IsPaused() {
			s.Resume()
		} else {
			s.Pause()
		}
	case "stop":
		s.audio.Stop()
	case "skip":
		s.Skip()
	default:
		slog.Warn("Transponder: unknown ident action", "action", action)
	}
}

// IsActive returns true if narrator is currently active (generating or playing).

// POIManager returns the internal POI manager.
func (s *AIService) POIManager() POIProvider {
	return s.poiMgr
}

func (s *AIService) session() *session.Manager {
	return s.sessionMgr
}

func (s *AIService) persistSession(lat, lon float64) {
	if s.st == nil {
		return
	}

	data, err := s.session().GetPersistentState(lat, lon)
	if err != nil {
		slog.Error("Narrator: Failed to serialize session state", "error", err)
		return
	}

	if err := s.st.SetState(context.Background(), "session_context", string(data)); err != nil {
		slog.Error("Narrator: Failed to persist session state", "error", err)
	}
}

// LLMProvider returns the internal LLM provider.
func (s *AIService) LLMProvider() llm.Provider {
	return s.llm
}

// AudioService returns the internal audio service.
func (s *AIService) AudioService() audio.Service {
	return s.audio
}

func (s *AIService) initAssembler() {
	if s.promptAssembler == nil {
		s.promptAssembler = prompt.NewAssembler(s.cfg, s.st, s.prompts, s.geoSvc, s.wikipedia, s.poiMgr, s.llm, s.categoriesCfg, nil)
	}
}

// DataProvider Implementation

func (s *AIService) GetLocation(lat, lon float64) model.LocationInfo {
	return s.geoSvc.GetLocation(lat, lon)
}

func (s *AIService) GetPOIsNear(lat, lon, radius float64) []*model.POI {
	return s.poiMgr.GetPOIsNear(lat, lon, radius)
}

func (s *AIService) GetRepeatTTL() time.Duration {
	return time.Duration(s.cfg.Narrator.RepeatTTL)
}

func (s *AIService) AddEvent(event *model.TripEvent) {
	s.session().AddEvent(event)
}

func (s *AIService) AssemblePOI(ctx context.Context, p *model.POI, t *sim.Telemetry, strategy string) prompt.Data {
	s.initAssembler()
	return s.promptAssembler.ForPOI(ctx, p, t, strategy, s.getSessionState())
}

func (s *AIService) AssembleGeneric(ctx context.Context, t *sim.Telemetry) prompt.Data {
	s.initAssembler()
	return s.promptAssembler.ForGeneric(ctx, t, s.getSessionState())
}

func (s *AIService) NewContext() map[string]any {
	s.initAssembler()
	return map[string]any(s.promptAssembler.NewPromptData(s.getSessionState()))
}

func (s *AIService) getSessionState() prompt.SessionState {
	return s.session().GetState()
}
