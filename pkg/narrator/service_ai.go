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
	"phileasgo/pkg/tts/edgetts"
)

// ScriptEntry represents a previously generated narration script.
type ScriptEntry struct {
	QID    string
	Title  string
	Script string
}

// AIService is the real implementation of the narrator service using LLM and TTS.
type AIService struct {
	cfg       *config.Config
	llm       llm.Provider
	tts       tts.Provider
	prompts   *prompts.Manager
	audio     audio.Service
	poiMgr    POIProvider
	beaconSvc BeaconProvider
	geoSvc    GeoProvider
	sim       sim.Client
	st        store.Store
	wikipedia WikipediaProvider
	langRes   LanguageResolver

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

	// Replay State
	lastPOI        *model.POI
	lastEssayTopic *EssayTopic
	lastEssayTitle string

	essayH    *EssayHandler
	interests []string

	scriptHistory []ScriptEntry

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
	tr *tracker.Tracker,
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
		stats:           make(map[string]any),
		latencies:       make([]time.Duration, 0, 10),
		essayH:          essayH,
		skipCooldown:    false,
		interests:       interests,
		fallbackTracker: tr,
	}
	// Initial default window
	s.sim.SetPredictionWindow(60 * time.Second)
	s.scriptHistory = make([]ScriptEntry, 0, cfg.Narrator.ContextHistorySize)
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
	slog.Info("AI Narrator service stopped")
}

// IsActive returns true if narrator is currently active (generating or playing).
func (s *AIService) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// IsGenerating returns true if narrator is currently generating script/audio.
func (s *AIService) IsGenerating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generating
}

// IsPlaying returns true if narrator is currently playing audio (or checking busy state).
func (s *AIService) IsPlaying() bool {
	// We delegate to audio manager's IsBusy because "paused" also means "playing" in context of scheduler
	return s.audio.IsBusy()
}

// IsPaused returns true if the narrator is globally paused by the user.
func (s *AIService) IsPaused() bool {
	return s.audio.IsUserPaused()
}

// CurrentPOI returns the POI currently being narrated, if any.
func (s *AIService) CurrentPOI() *model.POI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPOI
}

// CurrentTitle returns the title of the current narration.
func (s *AIService) CurrentTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentPOI != nil {
		return s.currentPOI.DisplayName()
	}
	if s.currentTopic != nil {
		if s.currentEssayTitle != "" {
			return s.currentEssayTitle
		}
		return "Essay about " + s.currentTopic.Name
	}
	return ""
}

// NarratedCount returns the number of narrated POIs.
func (s *AIService) NarratedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.narratedCount
}

// Stats returns narrator statistics.
func (s *AIService) Stats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	res := make(map[string]any, len(s.stats))
	for k, v := range s.stats {
		res[k] = v
	}
	// Add prediction window stats
	if len(s.latencies) > 0 {
		var sum time.Duration
		for _, d := range s.latencies {
			sum += d
		}
		avg := sum / time.Duration(len(s.latencies))
		res["latency_avg_ms"] = avg.Milliseconds()
	}
	return res
}

func (s *AIService) updateLatency(d time.Duration) {
	s.mu.Lock()
	s.latencies = append(s.latencies, d)
	if len(s.latencies) > 10 {
		s.latencies = s.latencies[1:]
	}

	// Calculate rolling average and update prediction window
	var sum time.Duration
	for _, lat := range s.latencies {
		sum += lat
	}
	avg := sum / time.Duration(len(s.latencies))
	s.mu.Unlock()

	// Update the sim's prediction window with the observed latency
	s.sim.SetPredictionWindow(avg)
	slog.Debug("Narrator: Updated latency stats", "new_latency", d, "rolling_window_size", len(s.latencies), "new_prediction_window", avg)
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

// activateFallback switches to edge-tts for the remainder of this session.
// Called when Azure TTS returns a fatal error (429, 5xx, etc.)
func (s *AIService) activateFallback() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.useFallbackTTS {
		return // Already activated
	}

	slog.Warn("Narrator: Activating edge-tts fallback for this session")
	s.fallbackTTS = edgetts.NewProvider(s.fallbackTracker) // With tracker for stats
	s.useFallbackTTS = true
}

// getTTSProvider returns the active TTS provider (fallback if activated).
func (s *AIService) getTTSProvider() tts.Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.useFallbackTTS && s.fallbackTTS != nil {
		return s.fallbackTTS
	}
	return s.tts
}

// isUsingFallbackTTS returns true if fallback TTS is active.
func (s *AIService) isUsingFallbackTTS() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.useFallbackTTS
}

func (s *AIService) addScriptToHistory(qid, title, script string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scriptHistory = append(s.scriptHistory, ScriptEntry{
		QID:    qid,
		Title:  title,
		Script: script,
	})

	// We no longer discard strictly here, as the user wants to keep a "log"
	// and we filter/evict at read-time based on manager state.
	// But we still respect the limit for the PROMPT context window.
}

func (s *AIService) getScriptHistory() []ScriptEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := s.cfg.Narrator.ContextHistorySize
	if limit <= 0 {
		return nil
	}

	// Filter based on tracked POIs (Spatial Eviction Sync)
	var filtered []ScriptEntry
	for i := len(s.scriptHistory) - 1; i >= 0 && len(filtered) < limit; i-- {
		entry := s.scriptHistory[i]

		// Essay has no QID, keep it if it's recent
		if entry.QID == "" {
			filtered = append([]ScriptEntry{entry}, filtered...)
			continue
		}

		// Check if POI is still tracked
		if _, err := s.poiMgr.GetPOI(context.Background(), entry.QID); err == nil {
			filtered = append([]ScriptEntry{entry}, filtered...)
		}
	}

	return filtered
}
