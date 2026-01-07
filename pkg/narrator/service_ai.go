package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"phileasgo/pkg/audio"
	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tts"
)

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
	wiki      WikiProvider
	langRes   LanguageResolver

	mu            sync.RWMutex
	running       bool
	active        bool
	generating    bool
	skipCooldown  bool
	narratedCount int
	stats         map[string]any
	latencies     []time.Duration

	currentPOI        *model.POI
	currentTopic      *EssayTopic
	currentEssayTitle string
	// Replay State
	lastPOI        *model.POI
	lastEssayTopic *EssayTopic
	lastEssayTitle string

	essayH    *EssayHandler
	interests []string
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
	wikiClient WikiProvider,
	langRes LanguageResolver,
	essayH *EssayHandler,
	interests []string,
) *AIService {
	s := &AIService{
		cfg:          cfg,
		llm:          llm,
		tts:          tts,
		prompts:      prompts, // Keeping concrete for now, hard to interface due to templates
		audio:        audioMgr,
		poiMgr:       poiMgr,
		beaconSvc:    beaconSvc,
		geoSvc:       geoSvc,
		sim:          simClient,
		st:           st,
		wiki:         wikiClient,
		langRes:      langRes,
		stats:        make(map[string]any),
		latencies:    make([]time.Duration, 0, 10),
		essayH:       essayH,
		skipCooldown: false,
		interests:    interests,
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
	defer s.mu.Unlock()
	s.latencies = append(s.latencies, d)
	if len(s.latencies) > 10 {
		s.latencies = s.latencies[1:]
	}
	slog.Debug("Narrator: Updated latency stats", "new_latency", d, "rolling_window_size", len(s.latencies))
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

// PlayPOI triggers narration for a specific POI.
func (s *AIService) PlayPOI(ctx context.Context, poiID string, manual bool, tel *sim.Telemetry) {
	if manual {
		slog.Info("Narrator: Manual play requested", "poi_id", poiID)
	} else {
		slog.Info("Narrator: Automated play triggering", "poi_id", poiID)
	}

	// 1. Synchronous state update to prevent races
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.generating = true
	s.mu.Unlock()

	// Fetch POI from manager
	p, err := s.poiMgr.GetPOI(context.Background(), poiID)
	if err != nil {
		slog.Error("Narrator: Failed to fetch POI", "poi_id", poiID, "error", err)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.mu.Unlock()
		return
	}

	go s.narratePOI(context.Background(), p, tel, time.Now())
}

// PlayEssay triggers a regional essay narration.
func (s *AIService) PlayEssay(ctx context.Context, tel *sim.Telemetry) bool {
	if s.essayH == nil {
		return false
	}

	// 1. Synchronous state update to prevent races
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return false
	}
	s.active = true
	s.generating = true
	s.mu.Unlock()

	slog.Info("Narrator: Triggering Essay")

	topic, err := s.essayH.SelectTopic()
	if err != nil {
		slog.Error("Narrator: Failed to select essay topic", "error", err)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.mu.Unlock()
		return false
	}

	go s.narrateEssay(context.Background(), topic, tel)
	return true
}

func (s *AIService) ReplayLast(ctx context.Context) bool {
	// 1. Check Audio Replay Capability
	if !s.audio.ReplayLastNarration() {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 2. Restore State for UI
	// 2. Restore State for UI
	switch {
	case s.lastPOI != nil:
		slog.Info("Narrator: Replaying last POI", "title", s.lastPOI.NameEn)
		s.currentPOI = s.lastPOI
		s.active = true // Mark active so UI shows "PLAYING"
	case s.lastEssayTopic != nil:
		slog.Info("Narrator: Replaying last Essay", "title", s.lastEssayTitle)
		s.currentTopic = s.lastEssayTopic
		s.currentEssayTitle = s.lastEssayTitle
		s.active = true
	default:
		// Audio replayed but we have no state?
		slog.Warn("Narrator: Replaying audio but no state to restore")
		return true
	}

	// 3. Launch Monitor to clear state when done
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !s.audio.IsBusy() {
					s.mu.Lock()
					s.active = false
					s.currentPOI = nil
					s.currentTopic = nil
					s.currentEssayTitle = ""
					s.mu.Unlock()
					return
				}
			}
		}
	}()

	return true
}

func (s *AIService) narrateEssay(ctx context.Context, topic *EssayTopic, tel *sim.Telemetry) {
	// active is already set true by PlayEssay
	s.mu.Lock()
	s.currentTopic = topic
	s.currentEssayTitle = "" // Reset title until generated
	s.lastPOI = nil          // Clear last POI since this is new
	s.lastEssayTopic = topic // Set for replay
	s.lastEssayTitle = ""    // Will update if generated
	s.mu.Unlock()

	defer func() {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.currentTopic = nil
		s.currentEssayTitle = ""
		s.mu.Unlock()
	}()

	if s.beaconSvc != nil {
		s.beaconSvc.Clear()
	}

	slog.Info("Narrator: Narrating Essay", "topic", topic.Name)

	// Gather Context
	if tel == nil {
		t, _ := s.sim.GetTelemetry(ctx)
		tel = &t
	}

	loc := s.geoSvc.GetLocation(tel.Latitude, tel.Longitude)
	region := loc.CityName
	if loc.CityName != "Unknown" {
		region = "Near " + loc.CityName
	}

	pd := NarrationPromptData{
		TourGuideName:    "Ava", // TODO: Config
		FemalePersona:    "Intelligent, fascinating",
		FemaleAccent:     "Neutral",
		TargetLanguage:   s.cfg.Narrator.TargetLanguage,
		TargetCountry:    loc.CountryCode,
		TargetRegion:     region,
		Lat:              tel.Latitude,
		Lon:              tel.Longitude,
		UnitsInstruction: s.fetchUnitsInstruction(),
	}
	pd.TTSInstructions = s.fetchTTSInstructions(&pd)

	prompt, err := s.essayH.BuildPrompt(ctx, topic, &pd)
	if err != nil {
		slog.Error("Narrator: Failed to render essay prompt", "error", err)
		return
	}

	// Generate Script
	script, err := s.llm.GenerateText(ctx, "essay", prompt)
	if err != nil {
		slog.Error("Narrator: LLM essay script generation failed", "error", err)
		return
	}

	// Parse Title if present (Format: "TITLE: ...")
	lines := strings.Split(script, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "TITLE:") {
			title := strings.TrimSpace(strings.TrimPrefix(firstLine, "TITLE:"))
			s.mu.Lock()
			s.currentEssayTitle = title
			s.lastEssayTitle = title // Capture for replay
			s.mu.Unlock()

			// Remove title line from script for TTS
			script = strings.Join(lines[1:], "\n")
		}
	}

	// Synthesis
	cacheDir := os.TempDir()
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("phileas_essay_%s_%d", topic.ID, time.Now().UnixNano()))
	format, err := s.tts.Synthesize(ctx, script, "", outputPath)
	if err != nil {
		slog.Error("Narrator: TTS essay synthesis failed", "error", err)
		return
	}

	audioFile := outputPath + "." + format

	// Playback
	if err := s.audio.Play(audioFile, false); err != nil {
		slog.Error("Narrator: Playback failed", "path", audioFile, "error", err)
		return
	}

	s.mu.Lock()
	s.generating = false
	s.mu.Unlock()

	// Wait for finish
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.audio.Stop()
			return
		case <-ticker.C:
			if !s.audio.IsBusy() {
				return
			}
		}
	}
}

// SkipCooldown forces the cooldown to expire (not strictly needed by AIService itself, but by the job).
func (s *AIService) SkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = true
	slog.Info("Narrator: Skip cooldown requested")
}

// ShouldSkipCooldown returns true if the cooldown should be skipped.
func (s *AIService) ShouldSkipCooldown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.skipCooldown
}

// ResetSkipCooldown resets the skip cooldown flag.
func (s *AIService) ResetSkipCooldown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipCooldown = false
}

func (s *AIService) narratePOI(ctx context.Context, p *model.POI, tel *sim.Telemetry, startTime time.Time) {
	// active is already set true by PlayPOI
	s.mu.Lock()
	s.currentPOI = p
	s.lastPOI = p          // Capture for replay
	s.lastEssayTopic = nil // Clear essay since this is new
	s.lastEssayTitle = ""
	s.mu.Unlock()

	defer func() {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		s.active = false
		s.generating = false
		s.currentPOI = nil
		s.mu.Unlock()
	}()

	// 0a. Mark as played immediately to prevent re-selection during generation/skip
	p.LastPlayed = time.Now()
	if err := s.st.SavePOI(ctx, p); err != nil {
		slog.Error("Narrator: Failed to save narrated POI state", "qid", p.WikidataID, "error", err)
	}

	// 1. Gather Context & Build Prompt
	promptData := s.buildPromptData(ctx, p, tel)

	slog.Info("Narrator: Narrating POI", "name", p.DisplayName(), "qid", p.WikidataID, "relative_dominance", promptData.DominanceStrategy)

	prompt, err := s.prompts.Render("narrator/script.tmpl", promptData)
	if err != nil {
		slog.Error("Narrator: Failed to render prompt", "error", err)
		return
	}

	// 2. Optional: Marker Spawning (Before LLM to give immediate visual feedback)
	if s.beaconSvc != nil {
		// Spawn target beacon at POI. Altitude 0 (ground) for now.
		_ = s.beaconSvc.SetTarget(ctx, p.Lat, p.Lon)
	}

	// 3. Generate LLM Script
	script, err := s.llm.GenerateText(ctx, "narration", prompt)
	if err != nil {
		slog.Error("Narrator: LLM script generation failed", "error", err)
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		return
	}

	// 4. TTS Synthesis
	// Use system temp directory instead of persistent cache
	cacheDir := os.TempDir()

	// Sanitize filename
	safeID := strings.ReplaceAll(p.WikidataID, "/", "_")
	// Use unique name to avoid conflicts and persistence
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("phileas_narration_%s_%d", safeID, time.Now().UnixNano()))

	format, err := s.tts.Synthesize(ctx, script, "", outputPath)
	if err != nil {
		slog.Error("Narrator: TTS synthesis failed", "error", err)
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		return
	}

	audioFile := outputPath + "." + format

	// 5. Update Latency before Playback
	latency := time.Since(startTime)
	s.updateLatency(latency)

	// 6. Playback
	if err := s.audio.Play(audioFile, false); err != nil {
		slog.Error("Narrator: Playback failed", "path", audioFile, "error", err)
		if s.beaconSvc != nil {
			s.beaconSvc.Clear()
		}
		return
	}

	s.mu.Lock()
	s.generating = false
	s.mu.Unlock()

	// Log Stats
	genWords := len(strings.Fields(script))
	duration := s.audio.Duration()
	slog.Info("Narrator: Narration stats",
		"name", p.DisplayName(),
		"qid", p.WikidataID,
		"max_words_requested", promptData.MaxWords,
		"words_received", genWords,
		"audio_duration", duration,
	)

	// Block until playback finishes so state remains active
	// We check every 100ms
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	func() {
		for {
			select {
			case <-ctx.Done():
				s.audio.Stop()
				return
			case <-ticker.C:
				if !s.audio.IsBusy() {
					return
				}
			}
		}
	}()

	s.mu.Lock()
	s.narratedCount++
	s.mu.Unlock()
}

func (s *AIService) buildPromptData(ctx context.Context, p *model.POI, tel *sim.Telemetry) NarrationPromptData {
	// CC & Lang
	loc := s.geoSvc.GetLocation(p.Lat, p.Lon)
	cc := loc.CountryCode
	region := loc.CityName
	if loc.CityName != "Unknown" {
		region = "Near " + loc.CityName
	}

	// Navigation Instruction
	if tel == nil {
		t, _ := s.sim.GetTelemetry(ctx)
		tel = &t
	}
	nav := s.calculateNavInstruction(p, tel)
	maxWords, domStrat := s.sampleNarrationLength(p)

	// Language Logic (User's Target Language)
	targetLang := s.cfg.Narrator.TargetLanguage
	langCode := "en"
	langName := "English"
	langLocale := targetLang

	// Parse "de-DE" -> "DE"
	parts := strings.Split(targetLang, "-")
	if len(parts) == 2 {
		// Valid locale format
		targetCC := parts[1]
		if s.langRes != nil {
			info := s.langRes.GetLanguageInfo(targetCC)
			langCode = info.Code
			langName = info.Name
		} else {
			// Fallback if resolver missing (unlikely in prod)
			langCode = parts[0]
		}
	} else if len(parts) > 0 {
		// Fallback for non-standard config (though validation should catch this)
		langCode = parts[0]
	}

	pd := NarrationPromptData{
		TourGuideName:        "Ava", // TODO: Get from voice profile
		Persona:              "Intelligent, fascinating",
		Accent:               "Neutral",
		Language:             targetLang,
		Language_code:        langCode,
		Language_name:        langName,
		Language_region_code: langLocale,
		FemalePersona:        "Intelligent, fascinating",
		FemaleAccent:         "Neutral",
		PassengerMale:        "Andrew",
		MalePersona:          "Curious traveler",
		MaleAccent:           "Neutral",
		FlightStage:          determineFlightStage(tel),
		NameNative:           p.NameLocal,
		POINameNative:        p.NameLocal,
		NameUser:             p.DisplayName(),
		POINameUser:          p.DisplayName(),
		Category:             p.Category,
		WikipediaText:        s.fetchWikipediaText(ctx, p),
		NavInstruction:       nav,
		TargetLanguage:       s.cfg.Narrator.TargetLanguage,
		TargetCountry:        cc,
		Country:              cc,
		TargetRegion:         region,
		Region:               region,
		MaxWords:             maxWords,
		DominanceStrategy:    domStrat,
		RecentPoisContext:    s.fetchRecentContext(ctx, p.Lat, p.Lon),
		RecentContext:        s.fetchRecentContext(ctx, p.Lat, p.Lon),
		Lat:                  tel.Latitude,
		Lon:                  tel.Longitude,
		UnitsInstruction:     s.fetchUnitsInstruction(),
		Interests:            s.interests,
		AltitudeMSL:          tel.AltitudeMSL,
		AltitudeAGL:          tel.AltitudeAGL,
		Heading:              tel.Heading,
		GroundSpeed:          tel.GroundSpeed,
		PredictedLat:         tel.PredictedLatitude,
		PredictedLon:         tel.PredictedLongitude,
	}
	// Fetch TTS instructions with full context
	pd.TTSInstructions = s.fetchTTSInstructions(&pd)

	return pd
}

func (s *AIService) fetchTTSInstructions(data any) string {
	var tmplName string
	// engines: sapi, windows-sapi, edge, edge-tts, fish-audio
	switch strings.ToLower(s.cfg.TTS.Engine) {
	case "fish-audio":
		tmplName = "tts/fish-audio.tmpl"
	case "azure", "azure-speech":
		tmplName = "tts/azure.tmpl"
	default:
		// Default to edge-tts for clean output (no speaker labels) which is good for most
		tmplName = "tts/edge-tts.tmpl"
	}

	content, err := s.prompts.Render(tmplName, data)
	if err != nil {
		// Fallback if template missing
		slog.Warn("Narrator: Failed to render TTS template, using fallback", "template", tmplName, "error", err)
		return "Do not use speaker labels."
	}
	return content
}

func (s *AIService) calculateNavInstruction(p *model.POI, tel *sim.Telemetry) string {
	// Source coordinates: Use predicted if available (1 min ahead), else current
	latSrc, lonSrc := tel.Latitude, tel.Longitude
	if tel.PredictedLatitude != 0 || tel.PredictedLongitude != 0 {
		latSrc, lonSrc = tel.PredictedLatitude, tel.PredictedLongitude
	}

	pSrc := geo.Point{Lat: latSrc, Lon: lonSrc}
	pTarget := geo.Point{Lat: p.Lat, Lon: p.Lon}

	// Distance in NM (used for logic)
	distMeters := geo.Distance(pSrc, pTarget)
	distNm := distMeters * 0.000539957

	// 1. Distance String
	var distStr string
	unitSys := strings.ToLower(s.cfg.Narrator.Units)
	// Hybrid is considered metric for this case
	if unitSys == "metric" || unitSys == "hybrid" {
		distKm := distMeters / 1000.0
		switch {
		case distKm < 0.5:
			distStr = "just ahead"
		case distKm < 1.0:
			distStr = "less than a kilometer"
		default:
			distStr = fmt.Sprintf("about %.0f kilometers", distKm)
		}
	} else {
		// Imperial is default
		switch {
		case distNm < 0.5:
			distStr = "just ahead"
		case distNm < 1.0:
			distStr = "less than a mile"
		default:
			distStr = fmt.Sprintf("about %.0f miles", distNm)
		}
	}

	// 2. Ground Logic
	if tel.IsOnGround {
		return s.formatGroundInstruction(pSrc, pTarget, distNm, distStr)
	}

	// 3. Airborne Logic
	return s.formatAirborneInstruction(pSrc, pTarget, tel.Heading, distNm, distStr)
}

func (s *AIService) formatGroundInstruction(pSrc, pTarget geo.Point, distNm float64, distStr string) string {
	if distNm < 1.6 {
		return ""
	}
	// Cardinal Directions
	bearing := geo.Bearing(pSrc, pTarget)
	normBearing := math.Mod(bearing+360, 360)
	dirs := []string{"North", "North-East", "East", "South-East", "South", "South-West", "West", "North-West"}
	idx := int((normBearing+22.5)/45.0) % 8
	direction := fmt.Sprintf("to the %s", dirs[idx])

	final := fmt.Sprintf("%s, %s away", direction, distStr)
	if distNm < 1.0 {
		final = fmt.Sprintf("%s, %s", direction, distStr)
	}
	return capitalizeStart(final)
}

func (s *AIService) formatAirborneInstruction(pSrc, pTarget geo.Point, userHdg, distNm float64, distStr string) string {
	bearing := geo.Bearing(pSrc, pTarget)
	relBearing := math.Mod(bearing-userHdg+360, 360)

	var direction string

	if distNm >= 3.0 {
		// Clock Position for > 3NM
		clock := int((relBearing + 15) / 30)
		if clock == 0 {
			clock = 12
		}
		direction = fmt.Sprintf("at your %d o'clock", clock)
	} else {
		// Relative Directions for < 3NM
		switch {
		case relBearing >= 345 || relBearing <= 15:
			direction = "straight ahead"
		case relBearing > 15 && relBearing <= 135:
			direction = "on your right" // 15-135 right (covers 15-45 and 45-135)
		case relBearing > 135 && relBearing <= 225:
			direction = "behind you"
		case relBearing > 225 && relBearing < 345:
			direction = "on your left" // 225-345 left (covers 225-315 and 315-345)
		}
	}

	final := fmt.Sprintf("%s, %s", direction, distStr)
	return capitalizeStart(final)
}

func capitalizeStart(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	// Simple upper for first rune
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

func (s *AIService) fetchWikipediaText(ctx context.Context, p *model.POI) string {
	// 1. Try Store using QID as UUID
	art, _ := s.st.GetArticle(ctx, p.WikidataID)
	if art != nil && art.Text != "" {
		return art.Text
	}

	// 2. Fetch if missing
	if p.WPURL == "" {
		return ""
	}
	// Safeguard: If URL is still pointing to Wikidata (failed rescue), do not attempt fetch
	if strings.Contains(p.WPURL, "wikidata.org") {
		return ""
	}

	// Parse Title/Lang from URL: https://en.wikipedia.org/wiki/Title
	parts := strings.Split(p.WPURL, "/")
	if len(parts) < 5 {
		return ""
	}
	title := parts[len(parts)-1]
	lang := "en"
	if strings.Contains(parts[2], ".") {
		lang = strings.Split(parts[2], ".")[0]
	}

	text, err := s.wiki.GetArticleContent(ctx, title, lang)
	if err != nil {
		slog.Warn("Narrator: Failed to fetch Wikipedia extract", "title", title, "error", err)
		return ""
	}

	// 3. Cache it
	_ = s.st.SaveArticle(ctx, &model.Article{
		UUID:  p.WikidataID,
		Title: title,
		URL:   p.WPURL,
		Text:  text,
	})

	return text
}

func (s *AIService) fetchRecentContext(ctx context.Context, lat, lon float64) string {
	since := time.Now().Add(-1 * time.Hour)
	pois, err := s.st.GetRecentlyPlayedPOIs(ctx, since)
	if err != nil {
		slog.Warn("Narrator: Failed to fetch recent POIs for context", "error", err)
		return "None"
	}

	var contextParts []string
	p1 := geo.Point{Lat: lat, Lon: lon}
	for _, p := range pois {
		// Filter by distance (50km) in Go
		p2 := geo.Point{Lat: p.Lat, Lon: p.Lon}
		dist := geo.Distance(p1, p2)
		if dist < 50000 {
			contextParts = append(contextParts, fmt.Sprintf("%s (%s)", p.NameEn, p.Category))
		}
	}

	if len(contextParts) == 0 {
		return "None"
	}

	return strings.Join(contextParts, ", ")
}

// NarrationPromptData struct for templates
type NarrationPromptData struct {
	TourGuideName        string
	Persona              string
	Accent               string
	Language             string
	Language_code        string
	Language_name        string
	Language_region_code string
	FemalePersona        string
	FemaleAccent         string
	PassengerMale        string
	MalePersona          string
	MaleAccent           string
	FlightStage          string
	NameNative           string
	POINameNative        string
	NameUser             string
	POINameUser          string
	Category             string
	WikipediaText        string
	NavInstruction       string
	TargetLanguage       string
	TargetCountry        string
	Country              string
	TargetRegion         string
	Region               string
	Lat                  float64
	Lon                  float64
	MaxWords             int
	RecentPoisContext    string
	RecentContext        string
	UnitsInstruction     string
	TTSInstructions      string
	Interests            []string
	AltitudeMSL          float64
	AltitudeAGL          float64
	Heading              float64
	GroundSpeed          float64
	PredictedLat         float64
	PredictedLon         float64
	DominanceStrategy    string
}

func (s *AIService) sampleNarrationLength(p *model.POI) (words int, strategy string) {
	minL := s.cfg.Narrator.NarrationLengthMin
	maxL := s.cfg.Narrator.NarrationLengthMax
	if minL == 0 {
		minL = 400
	}
	if maxL == 0 {
		maxL = 600
	}
	if maxL <= minL {
		return minL, "fixed"
	}

	// Dynamic Length Logic: Relative Dominance
	// "Rivals" are other POIs with > 50% of the winner's score.
	// Note: CountScoredAbove includes the winner itself if score > 0.
	threshold := 0.0
	if p != nil {
		threshold = p.Score * 0.5
	}

	// We only need to know if there are > 1 rivals (so limit=2 is sufficient to know if count >= 2)
	// Actually, if we want to confirm "rivals > 1", that means Total >= 2 (Me + 1).
	// So finding 2 is enough to trigger the "min_skew".
	rivals := s.poiMgr.CountScoredAbove(threshold, 2)

	// Default Strategy: Uniform Random
	strategy = "uniform"

	// If p.Score is 0 (manual play?), handle gracefully (rivals will be ALL POIs > 0)
	// Assuming normal flow:
	// If rivals > 1 (Winner + at least 1 other) -> Skew Short (High Competition)
	if rivals > 1 {
		strategy = "min_skew"
	} else if rivals <= 1 {
		// Winner is alone (or score is 0 and no others > 0) -> Skew Long (Lone Wolf)
		strategy = "max_skew"
	}

	score := 0.0
	if p != nil {
		score = p.Score
	}
	slog.Debug("Narrator: Sampling Length", "strategy", strategy, "rivals", rivals, "score", score, "threshold", threshold)

	// Helper to get a random value in range
	randomVal := func() int {
		steps := (maxL - minL) / 10
		step := rand.Intn(steps + 1)
		return minL + step*10
	}

	// Pool Selection
	poolSize := 3
	pool := make([]int, poolSize)
	for i := 0; i < poolSize; i++ {
		pool[i] = randomVal()
	}

	var result int
	switch strategy {
	case "min_skew":
		// Pick smallest
		minVal := pool[0]
		for _, v := range pool {
			if v < minVal {
				minVal = v
			}
		}
		result = minVal
	case "max_skew":
		// Pick largest
		maxVal := pool[0]
		for _, v := range pool {
			if v > maxVal {
				maxVal = v
			}
		}
		result = maxVal
	default:
		// Pick first (which is random)
		result = pool[0]
	}
	return result, strategy
}
