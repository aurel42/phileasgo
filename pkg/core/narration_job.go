package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/terrain"
	"phileasgo/pkg/watcher"
)

// POIProvider matches the GetBestCandidate method used by NarrationJob.
type POIProvider interface {
	GetNarrationCandidates(limit int, minScore *float64, isOnGround bool) []*model.POI
	LastScoredPosition() (lat, lon float64)
}

// NarrationJob triggers AI narration for the best available POI.
type NarrationJob struct {
	BaseJob
	cfg        *config.Config
	narrator   narrator.Service
	poiMgr     POIProvider
	sim        sim.Client
	store      store.Store
	losChecker *terrain.LOSChecker
	watcher    *watcher.Service
	lastTime   time.Time

	wasBusy          bool
	lastEssayTime    time.Time
	lastCheckedCount int

	// Post-takeoff grace period tracking
	takeoffTime    time.Time // Track when we left the ground
	lastAGL        float64   // Last known AGL for visibility boost check
	hasCheckedOnce bool      // Flag to handle startup state (e.g. starting mid-flight)
	hasDebriefed   bool      // Flag to ensure debrief runs only once per flight
}

func NewNarrationJob(cfg *config.Config, n narrator.Service, pm POIProvider, simC sim.Client, st store.Store, los *terrain.LOSChecker, w *watcher.Service) *NarrationJob {
	j := &NarrationJob{
		BaseJob:    NewBaseJob("Narration"),
		cfg:        cfg,
		narrator:   n,
		poiMgr:     pm,
		sim:        simC,
		store:      st,
		losChecker: los,
		watcher:    w,
		lastTime:   time.Now(),
	}

	return j
}

// checkNarratorReady returns true if the narrator is ready to accept a new command.
// For pipelining, we allow firing if playing, provided timing is right.
func (j *NarrationJob) checkNarratorReady() bool {
	if j.narrator.IsPaused() {
		return false
	}

	// Active means generating or other non-interruptible state
	if j.narrator.IsActive() && !j.narrator.IsPlaying() {
		// If active but NOT playing (e.g. generating for first time), we block.
		// If playing, we might be "active" in service terms, but we want to allow pipeline.
		// However, AIService.IsActive() returns true for both generating and playing.
		// We need to differentiate:
		// If Generating -> Busy (don't interrupt generation)
		// If Playing -> Potentially Ready (for pipeline)
		if j.narrator.IsGenerating() {
			slog.Debug("NarrationJob: Narrator generating")
			return false
		}
	}

	// Track rising edge/steady state of playback
	if j.narrator.IsPlaying() {
		j.wasBusy = true
	}

	// Playback just finished - start cooldown
	// This logic handles the "falling edge" of IsPlaying
	if !j.narrator.IsPlaying() && j.wasBusy {
		j.wasBusy = false
		j.lastTime = time.Now()
		slog.Debug("NarrationJob: Narration cycle finished (including pause)")
		return false
	}

	return true
}

// CanPreparePOI checks if the system is ready to prepare a POI narration (Manual or Auto).
// This includes checking frequency rules (pipelining) and narrator state.
func (j *NarrationJob) CanPreparePOI(t *sim.Telemetry) bool {
	// 1. Pre-flight checks
	if !j.checkPreConditions(t) {
		return false
	}
	if !j.checkPostTakeoffGrace(t) {
		return false
	}

	// 2. Narrator Activity Check (Base)
	// If generating, we are busy.
	if j.narrator.IsGenerating() {
		return false
	}
	// Also check Pause / Cooldown logic
	if !j.checkNarratorReady() {
		return false
	}

	// 3. Frequency & Pipeline Logic
	return j.checkFrequencyRules()
}

// CanPrepareEssay checks if the system is ready for an essay.
func (j *NarrationJob) CanPrepareEssay(t *sim.Telemetry) bool {
	// 1. Pre-flight
	if !j.checkPreConditions(t) {
		return false
	}
	// 2. State Check - essays require complete silence
	if j.narrator.IsPaused() || j.narrator.IsGenerating() || j.narrator.IsPlaying() {
		return false
	}
	// 3. Essay Logic
	return j.checkEssayEligible(t)
}

// CheckScreenshots polls the watcher for new screenshots and triggers playback.
func (j *NarrationJob) CheckScreenshots(ctx context.Context, t *sim.Telemetry) {
	if j.watcher != nil && j.cfg.Narrator.Screenshot.Enabled {
		if path, ok := j.watcher.CheckNew(); ok {
			slog.Info("NarrationJob: New screenshot detected", "path", path)
			j.narrator.PlayImage(ctx, path, t)
		}
	}
}

// PreparePOI triggers the finding and playing of a POI.
// Returns true if a POI was successfully found and triggered (or pipelined).
func (j *NarrationJob) PreparePOI(ctx context.Context, t *sim.Telemetry) bool {
	if !j.TryLock() {
		return false
	}
	defer j.Unlock()

	// Pick best (first visible)
	best := j.getVisibleCandidate(t)
	if best == nil {
		// No candidates? Boost visibility for next time.
		// Only if we passed all the readiness checks (which we did to get here).
		j.incrementVisibilityBoost(ctx)
		return false
	}

	// Re-verify playability
	if !j.isPlayable(best) {
		// If best is not playable, we technically found something but rejected it.
		// Should we boost? Probably yes, effectively "nothing playable found".
		j.incrementVisibilityBoost(ctx)
		return false
	}

	strategy := narrator.DetermineSkewStrategy(best, j.poiMgr.(narrator.POIAnalyzer), t.IsOnGround)

	// Logging
	slog.Info("NarrationJob: Triggering POI", "name", best.DisplayName())
	j.resetVisibilityBoost(ctx)

	// Play or Pipeline
	if j.narrator.IsPlaying() {
		if err := j.narrator.PrepareNextNarrative(ctx, best.WikidataID, strategy, t); err != nil {
			slog.Error("NarrationJob: Pipeline preparation failed", "error", err)
			return false
		}
	} else {
		// Auto-play (manual=false)
		j.narrator.PlayPOI(ctx, best.WikidataID, false, false, t, strategy)
	}
	return true
}

// PrepareEssay triggers an essay narration.
func (j *NarrationJob) PrepareEssay(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	if j.narrator.PlayEssay(ctx, t) {
		now := time.Now()
		j.lastEssayTime = now
		j.lastTime = now
	}
}

// CanPrepareDebrief checks if the system is ready for a debrief.
func (j *NarrationJob) CanPrepareDebrief(t *sim.Telemetry) bool {
	// 1. Config Check
	if !j.cfg.Narrator.Debrief.Enabled {
		return false
	}

	// 2. State Check
	if j.narrator.IsPaused() || j.narrator.IsGenerating() {
		return false
	}

	// 3. Already Debriefed?
	if j.hasDebriefed {
		return false
	}

	// 3. Ground Logic
	// Must be on ground
	if !t.IsOnGround {
		return false
	}

	// 4. Flight Logic
	// Must have had a takeoff recently (tracked by takeoffTime)
	// If takeoffTime is zero, we never took off in this session.
	if j.takeoffTime.IsZero() {
		return false
	}

	// Optional: Check if we landed "recently"?
	// Actually, if we are on ground AND takeoffTime is set AND !hasDebriefed,
	// it implies we landed and are waiting to debrief.
	// We might want to wait a few seconds after landing?
	// For now, immediate is fine, PlayDebrief has internal checks too?

	return true
}

// PrepareDebrief triggers a debrief narration.
func (j *NarrationJob) PrepareDebrief(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	if j.narrator.PlayDebrief(ctx, t) {
		j.hasDebriefed = true
		// Reset takeoff time so we don't debrief again until next flight?
		// No, hasDebriefed flag handles this for the current "cycle".
		// But if we take off again, we need to reset hasDebriefed.
		// That reset logic belongs in checkPostTakeoffGrace or similar flight detection logic.
	}
}

// checkPostTakeoffGrace enforces a 1-minute silence after takeoff.
func (j *NarrationJob) checkPostTakeoffGrace(t *sim.Telemetry) bool {
	if t.IsOnGround {
		j.takeoffTime = time.Time{} // Reset
		j.hasCheckedOnce = true
		return true // Allowed on ground (for airport narration)
	}

	// Startup Logic: If first check and we are ALREADY airborne, assume mid-flight
	if !j.hasCheckedOnce {
		j.hasCheckedOnce = true
		if !t.IsOnGround {
			slog.Info("NarrationJob: Started airborne, bypassing takeoff grace period")
			// Set takeoff time to distant past so check passes
			j.takeoffTime = time.Now().Add(-24 * time.Hour)
			return true
		}
	}

	if j.takeoffTime.IsZero() {
		j.takeoffTime = time.Now()
		slog.Debug("NarrationJob: Takeoff detected", "time", j.takeoffTime)
	}

	if time.Since(j.takeoffTime) < 1*time.Minute {
		// Log periodically (every 10s) to avoid spam? relying on debug level.
		slog.Debug("NarrationJob: In post-takeoff grace period", "elapsed", time.Since(j.takeoffTime))
		return false
	}

	return true
}

// checkFrequencyRules determines if we can fire based on frequency settings (1-5).
// Handles pipeline/overlap logic.
func (j *NarrationJob) checkFrequencyRules() bool {
	freq := j.getNarrationFrequency()
	isPlaying := j.narrator.IsPlaying()

	// Strategies 1 (Rarely) & 2 (Normal): No Overlap
	if freq <= 2 {
		if isPlaying {
			return false
		}
		// If not playing, we are ready (pause is handled by IsPlaying hold)
		return true
	}

	// Strategies 3 (Active), 4 (Busy), 5 (Constant): Allow Overlap (Pipeline)
	// If not playing, we are obviously ready.
	if !isPlaying {
		return true
	}

	// Calculate Lead Time Multiplier based on Frequency
	// Active (3) -> 1.0x
	// Busy (4)   -> 1.5x
	// Constant (5) -> 2.0x
	var leadTimeMultiplier float64
	switch freq {
	case 3:
		leadTimeMultiplier = 1.0
	case 4:
		leadTimeMultiplier = 1.5
	case 5:
		leadTimeMultiplier = 2.0
	default:
		leadTimeMultiplier = 1.0
	}

	// Pipeline Logic: Fire if remaining time is within the lead time window.
	// Target Start Time = Now + Remaining.
	// Prep Time = AvgLatency.
	// We want (Target Start - Prep Time) <= Now.
	// => Remaining <= AvgLatency.
	//
	// With Lead Time Multiplier (M):
	// We allow starting earlier, effectively assuming latency is M times larger?
	// OR we want to buffer M times latency?
	// User said: "Lead Time = 1.5x AverageLatency"
	// This usually means we start when Remaining <= 1.5 * AvgLatency.
	// This creates a potential overlap if AvgLatency is accurate.

	remaining := j.narrator.Remaining()
	avgLatency := j.narrator.AverageLatency()

	// Dynamic Lead Time Threshold
	threshold := time.Duration(float64(avgLatency) * leadTimeMultiplier)

	// If Remaining time is small enough (we are close to end), trigger next.
	if remaining <= threshold {
		// Prevent double-firing: if we already fired for this cycle?
		// checkNarratorReady handles "Generating" state (returns false).
		// So if we are "Playing" but NOT "Generating", we are eligible.
		return true
	}

	return false
}

// checkPreConditions validates global switches, location consistency, sim state, and ground proximity.
func (j *NarrationJob) checkPreConditions(t *sim.Telemetry) bool {
	if !j.cfg.Narrator.AutoNarrate {
		slog.Debug("NarrationJob: AutoNarrate disabled")
		return false
	}

	if !j.isLocationConsistent(t) {
		slog.Debug("NarrationJob: Location inconsistent")
		return false
	}

	if j.sim.GetState() != sim.StateActive {
		return false
	}

	// Ground logic is now handled during POI candidate selection.
	// If t.IsOnGround, the POI provider will only return Aerodromes.
	return true
}

func (j *NarrationJob) isPlayable(p *model.POI) bool {
	// Check if already in pipeline (Generating, Queued, Playing)
	// This prevents the "double trigger" issue where a POI is selected again while generating/queued
	if j.narrator.IsPOIBusy(p.WikidataID) {
		return false
	}

	if p.LastPlayed.IsZero() {
		return true
	}
	return time.Since(p.LastPlayed) >= time.Duration(j.cfg.Narrator.RepeatTTL)
}

// hasEligiblePOI returns true if there is at least one visible POI candidate.
// This is used by checkEssayEligible to ensure essays are gap-fillers only.
func (j *NarrationJob) hasEligiblePOI(t *sim.Telemetry) bool {
	return j.getVisibleCandidate(t) != nil
}

// checkEssayEligible returns true if conditions for essay narration are met.
// Essays are gap-fillers: they only fire when there are NO visible POIs.
func (j *NarrationJob) checkEssayEligible(t *sim.Telemetry) bool {
	if !j.cfg.Narrator.Essay.Enabled {
		return false
	}

	// Disable Essay in "Rarely" mode
	if j.getNarrationFrequency() == 1 {
		return false
	}

	// PRIORITY RULE: Essays only fire when there are NO visible POIs
	// This is the core "gap filler" logic from v0.2.121
	if j.hasEligiblePOI(t) {
		return false
	}

	// Essay-specific cooldown (DelayBetweenEssays)
	if !j.lastEssayTime.IsZero() {
		if time.Since(j.lastEssayTime) < time.Duration(j.cfg.Narrator.Essay.DelayBetweenEssays) {
			return false
		}
	}

	// Global delay before essay (Time since last narration)
	// Must be quiet for at least DelayBeforeEssay
	delayBeforeEssay := time.Duration(j.cfg.Narrator.Essay.DelayBeforeEssay)
	if time.Since(j.lastTime) < delayBeforeEssay {
		return false
	}

	// Takeoff grace: Wait at least DelayBeforeEssay after takeoff
	// This gives POIs a chance to be narrated before falling back to essays
	if !j.takeoffTime.IsZero() && time.Since(j.takeoffTime) < delayBeforeEssay {
		return false
	}

	// Silence rule: at least 2x PauseDuration (Legacy check, maybe redundant now but safer to keep)
	minSilence := time.Duration(j.cfg.Narrator.PauseDuration) * 2
	if time.Since(j.lastTime) < minSilence {
		return false
	}

	// Altitude check
	if t.AltitudeAGL < 2000 {
		return false
	}

	slog.Debug("NarrationJob: Essay eligible (No POIs, Silence & Cooldown met)")
	return true
}

// getVisibleCandidate returns the highest-scoring POI that has line-of-sight.
// If LOS is disabled or no checker is available, falls back to GetBestCandidate.
func (j *NarrationJob) getVisibleCandidate(t *sim.Telemetry) *model.POI {
	// If LOS is disabled or checker unavailable, use simple best candidate
	if !j.cfg.Terrain.LineOfSight || j.losChecker == nil {
		slog.Debug("NarrationJob: LOS disabled or no checker", "los_enabled", j.cfg.Terrain.LineOfSight, "checker_nil", j.losChecker == nil)
		// Use dynamic config here too: Get top 1 respecting filter
		var minScore *float64
		if j.getFilterMode() != "adaptive" {
			val := j.getMinScore()
			minScore = &val
		}
		cands := j.poiMgr.GetNarrationCandidates(1, minScore, t.IsOnGround)
		if len(cands) > 0 {
			return cands[0]
		}
		return nil
	}

	// Get ALL candidates sorted by score (no arbitrary limit)
	// We pass nil for minScore because we want to filter ourselves later?
	// Actually no, we should filter at source if possible to reduce count,
	// BUT we need to potentially check adaptive mode inside the loop OR we just pass the threshold here.
	// Since checking LOS is expensive, filtering by score FIRST is good.
	// Wait, if adaptive mode is ON, minScore is effectively nil.
	// If adaptive mode is OFF, minScore is set.
	// So we can compute minScore and pass it!
	var minScore *float64
	if j.getFilterMode() != "adaptive" {
		val := j.getMinScore()
		minScore = &val
	}

	candidates := j.poiMgr.GetNarrationCandidates(1000, minScore, t.IsOnGround)
	slog.Debug("NarrationJob: LOS checking candidates", "count", len(candidates), "aircraft_alt_ft", t.AltitudeMSL)

	aircraftPos := geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	aircraftAltFt := t.AltitudeMSL

	// Dynamic Config reading (once per run)
	// threshold := j.getMinScore()
	// isAdaptive := j.getFilterMode() == "adaptive"

	checkedCount := 0
	for i, poi := range candidates {
		// Optimization: Score threshold already applied by GetNarrationCandidates
		// Only adaptive vs fixed logic was handled there too via minScore arg.

		// Also skip if not playable
		if !j.isPlayable(poi) {
			continue
		}

		// RARELY FILTER (Frequency 1): Ensure we only pick "Lone Wolves"
		if j.getNarrationFrequency() == 1 {
			analyzer, ok := j.poiMgr.(narrator.POIAnalyzer)
			if ok && narrator.DetermineSkewStrategy(poi, analyzer, t.IsOnGround) != narrator.StrategyMaxSkew {
				// Skip this candidate as it doesn't meet the "Rarely" criteria
				continue
			}
		}

		checkedCount++

		// Get POI ground elevation (meters -> feet)
		poiElevM, err := j.losChecker.GetElevation(poi.Lat, poi.Lon)
		if err != nil {
			slog.Debug("NarrationJob: LOS elevation error", "poi", poi.DisplayName(), "error", err)
			continue // Skip if we can't get elevation
		}
		poiAltFt := poiElevM * 3.28084 // meters to feet

		poiPos := geo.Point{Lat: poi.Lat, Lon: poi.Lon}

		// Check LOS with 0.5km step size
		isVisible := j.losChecker.IsVisible(aircraftPos, poiPos, aircraftAltFt, poiAltFt, 0.5)
		if i < 5 { // Log first 5 candidates only to avoid spam
			slog.Debug("NarrationJob: LOS check", "poi", poi.DisplayName(), "score", fmt.Sprintf("%.2f", poi.Score), "visible", isVisible, "poi_elev_ft", poiAltFt)
		}
		if isVisible {
			slog.Debug("NarrationJob: Selected visible POI", "poi", poi.DisplayName(), "score", fmt.Sprintf("%.2f", poi.Score))
			return poi // First visible POI wins
		}
	}

	if checkedCount != j.lastCheckedCount {
		slog.Warn("NarrationJob: All POIs blocked by LOS or Filter", "checked", checkedCount, "total_candidates", len(candidates))
		j.lastCheckedCount = checkedCount
	}
	return nil // All blocked
}

func (j *NarrationJob) isLocationConsistent(t *sim.Telemetry) bool {
	// Ensure the scores are fresh relative to our CURRENT position.
	// If the scorer hasn't run since we moved here (e.g. teleport), we wait.
	// We use a generous threshold (e.g. 10km) to allow for some movement during scoring.
	lastLat, lastLon := j.poiMgr.LastScoredPosition()

	// If 0,0 (never scored), wait.
	if lastLat == 0 && lastLon == 0 {
		return false
	}

	dist := geo.Distance(geo.Point{Lat: t.Latitude, Lon: t.Longitude}, geo.Point{Lat: lastLat, Lon: lastLon})
	return dist <= 10000 // 10km
}

func (j *NarrationJob) getNarrationFrequency() int {
	fallback := j.cfg.Narrator.Frequency
	if fallback < 1 {
		fallback = 3 // Default to Active if not set
	}

	if j.store == nil {
		return fallback
	}

	val, ok := j.store.GetState(context.Background(), "narration_frequency")
	if !ok || val == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}

	// Clamp to 1-5
	if parsed < 1 {
		return 1
	}
	if parsed > 5 {
		return 5
	}
	return parsed
}

// Helpers for Dynamic Config Reading
func (j *NarrationJob) getFilterMode() string {
	if j.store == nil {
		return "fixed"
	}
	val, ok := j.store.GetState(context.Background(), "filter_mode")
	if !ok || val == "" {
		return "fixed"
	}
	return val
}

func (j *NarrationJob) getMinScore() float64 {
	fallback := j.cfg.Narrator.MinScoreThreshold

	if j.store == nil {
		return fallback
	}
	val, ok := j.store.GetState(context.Background(), "min_poi_score")
	if !ok || val == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func (j *NarrationJob) incrementVisibilityBoost(ctx context.Context) {
	if j.store == nil {
		return
	}

	// Don't boost visibility while on ground or below 500ft AGL
	// At low altitudes, visibility is naturally limited; boosting would select inappropriate POIs
	// TESTING: Hardcoded 500ft threshold - tune this value based on testing
	const boostThresholdFt = 500.0
	if j.lastAGL < boostThresholdFt {
		slog.Debug("NarrationJob: Skipping visibility boost (low altitude)", "agl", j.lastAGL, "threshold", boostThresholdFt)
		return
	}

	current := 1.0
	val, ok := j.store.GetState(ctx, "visibility_boost")
	if ok && val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			current = f
		}
	}

	if current >= 1.5 {
		return // Max reached
	}

	newVal := current + 0.1
	if newVal > 1.5 {
		newVal = 1.5
	}

	_ = j.store.SetState(ctx, "visibility_boost", fmt.Sprintf("%.1f", newVal))
	slog.Debug("NarrationJob: Increasing visibility boost", "new_factor", newVal)
}

func (j *NarrationJob) resetVisibilityBoost(ctx context.Context) {
	if j.store == nil {
		return
	}
	// Only write if not already 1.0 to save DB writes
	val, ok := j.store.GetState(ctx, "visibility_boost")
	if ok && val == "1.0" {
		return
	}

	_ = j.store.SetState(ctx, "visibility_boost", "1.0")
	// Only log reset if it was actually boosted (optimization: check val != 1.0)
	if val != "1.0" && val != "" {
		slog.Debug("NarrationJob: Reset visibility boost", "previous", val)
	}
}
