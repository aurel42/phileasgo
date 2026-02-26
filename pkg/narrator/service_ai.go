package narrator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"phileasgo/pkg/audio"
	"phileasgo/pkg/config"
	"phileasgo/pkg/generation"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
	"phileasgo/pkg/wikidata"
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

	ThumbnailURL  string // Presentation metadata
	Summary       string // User-visible summary
	ShowInfoPanel bool
	TwoPass       bool
	PromptData    prompt.Data
}

// ScriptEntry represents a previously generated narration script.
type ScriptEntry struct {
	QID    string
	Title  string
	Script string
}

// AIService is the real implementation of the narrator generator using LLM and TTS.
type AIService struct {
	cfg           config.Provider
	llm           llm.Provider
	tts           tts.Provider
	prompts       *prompts.Manager
	poiMgr        POIProvider
	geoSvc        GeoProvider
	sim           sim.Client
	st            store.Store
	wikipedia     WikipediaProvider
	langRes       LanguageResolver
	categoriesCfg *config.CategoriesConfig // For pregrounding checks
	sessionMgr    *session.Manager
	density       *wikidata.DensityManager

	mu           sync.RWMutex
	running      bool
	generating   bool
	stats        map[string]any
	latencies    []time.Duration
	skipCooldown bool

	// Generation State
	generatingTitle     string
	generatingThumbnail string

	// Pending Manual Override (Queued)
	pendingManualID       string
	pendingManualStrategy string

	// Infrastructure
	promptAssembler *prompt.Assembler

	// Staging State (Pipeline)
	genQ          *generation.Manager // Generation queue manager
	generatingPOI *model.POI          // The POI currently being generated (for UI feedback)

	onPlayback func(n *model.Narrative, priority bool)

	essayH    *EssayHandler
	interests []string
	avoid     []string

	// scriptHistory []ScriptEntry // Removed scriptHistory

	// TTS Fallback State (session-level)
	fallbackTTS     tts.Provider
	useFallbackTTS  bool
	fallbackTracker *tracker.Tracker

	enricher POIEnricher
}

// NewAIService creates a new AI-powered narrator generator.
func NewAIService(
	cfg config.Provider,
	llm llm.Provider,
	tts tts.Provider,
	prompts *prompts.Manager,
	poiMgr POIProvider,
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
	density *wikidata.DensityManager,
	enricher POIEnricher,
) *AIService {
	s := &AIService{
		cfg:             cfg,
		llm:             llm,
		tts:             tts,
		prompts:         prompts,
		poiMgr:          poiMgr,
		geoSvc:          geoSvc,
		sim:             simClient,
		st:              st,
		wikipedia:       wikipediaClient,
		langRes:         langRes,
		categoriesCfg:   categoriesCfg,
		stats:           make(map[string]any),
		latencies:       make([]time.Duration, 0, 10),
		essayH:          essayH,
		interests:       interests,
		avoid:           avoid,
		fallbackTracker: tr,
		sessionMgr:      sessMgr,
		density:         density,
		genQ:            generation.NewManager(),
		enricher:        enricher,
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
		density,
		interests,
		avoid,
	)

	return s
}

// SetOnPlayback sets the callback for when a narrative is ready for playback.
func (s *AIService) SetOnPlayback(cb func(n *model.Narrative, priority bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPlayback = cb
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
	slog.Info("AI Narrator service stopped")
}

// NarratedCount returns the number of narratives generated in the current session.
func (s *AIService) NarratedCount() int {
	return s.session().NarratedCount()
}

// GetLastTransition returns the timestamp of the last transition to the given stage.
func (s *AIService) GetLastTransition(stage string) time.Time {
	if s.sim == nil {
		return time.Time{}
	}
	return s.sim.GetLastTransition(stage)
}

func (s *AIService) session() *session.Manager {
	return s.sessionMgr
}

// AudioService returns the internal audio service.
func (s *AIService) AudioService() audio.Service {
	return nil
}

func (s *AIService) initAssembler() {
	if s.promptAssembler == nil {
		s.promptAssembler = prompt.NewAssembler(s.cfg, s.st, s.prompts, s.geoSvc, s.wikipedia, s.poiMgr, s.llm, s.categoriesCfg, s.langRes, s.density, s.interests, s.avoid)
	}
}

// DataProvider Implementation

func (s *AIService) GetLocation(lat, lon float64) model.LocationInfo {
	return s.geoSvc.GetLocation(lat, lon)
}

func (s *AIService) IsUserPaused() bool {
	return false // Generator itself isn't "paused", the playback orchestrator is.
}

func (s *AIService) GetPOIsNear(lat, lon, radius float64) []*model.POI {
	return s.poiMgr.GetPOIsNear(lat, lon, radius)
}

func (s *AIService) GetRepeatTTL() time.Duration {
	return s.cfg.RepeatTTL(context.Background())
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
