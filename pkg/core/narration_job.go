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
	"phileasgo/pkg/prompt"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/terrain"
)

// POIProvider matches the GetBestCandidate method used by NarrationJob.
type POIProvider interface {
	GetNarrationCandidates(limit int, minScore *float64) []*model.POI
	LastScoredPosition() (lat, lon float64)
}

// NarrationJob triggers AI narration for the best available POI.
type NarrationJob struct {
	BaseJob
	cfgProv    config.Provider
	narrator   narrator.Service
	poiMgr     POIProvider
	sim        sim.Client
	store      store.Store
	losChecker *terrain.LOSChecker
	lastTime   time.Time

	wasBusy            bool
	lastEssayTime      time.Time
	lastCandidateCount int
	lastLat            float64
	lastLon            float64
	lastAlt            float64
	lastIsPlaying      bool
	lastIsGenerating   bool
	lastMinScore       float64
	cachedBest         *model.POI

	// Flight tracking
	lastAGL float64 // Last known AGL for visibility boost check
}

func NewNarrationJob(cfgProv config.Provider, n narrator.Service, pm POIProvider, simC sim.Client, st store.Store, los *terrain.LOSChecker) *NarrationJob {
	j := &NarrationJob{
		BaseJob:            NewBaseJob("Narration"),
		cfgProv:            cfgProv,
		narrator:           n,
		poiMgr:             pm,
		sim:                simC,
		store:              st,
		losChecker:         los,
		lastTime:           time.Now(),
		lastCandidateCount: -1,
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
func (j *NarrationJob) CanPreparePOI(ctx context.Context, t *sim.Telemetry) bool {
	// 1. Pre-flight checks
	if !j.checkPreConditions(ctx, t) {
		return false
	}
	if !j.checkFlightStagePOI(t) {
		return false
	}

	// 2. Narrator Activity Check (Base)
	// If already have an auto-narration staged or generating, we are busy.
	if j.narrator.HasStagedAuto() {
		return false
	}

	// Also check Pause / Cooldown logic
	if !j.checkNarratorReady() {
		return false
	}

	// 3. Frequency & Pipeline Logic
	return j.checkFrequencyRules(ctx)
}

// CanPrepareEssay checks if the system is ready for an essay.
func (j *NarrationJob) CanPrepareEssay(ctx context.Context, t *sim.Telemetry) bool {
	// 1. Pre-flight
	if !j.checkPreConditions(ctx, t) {
		return false
	}
	// 2. State Check - essays require complete silence and no staged content
	if j.narrator.IsPaused() || j.narrator.HasStagedAuto() || j.narrator.IsPlaying() {
		return false
	}
	// 3. Essay Logic
	return j.checkEssayEligible(ctx, t)
}

// PreparePOI triggers the finding and playing of a POI.
// Returns true if a POI was successfully found and triggered (or pipelined).
func (j *NarrationJob) PreparePOI(ctx context.Context, t *sim.Telemetry) bool {
	if !j.TryLock() {
		return false
	}
	defer j.Unlock()

	j.cachedBest = nil
	// Pick best (first visible)
	best := j.getVisibleCandidate(ctx, t)
	if best == nil {
		// No candidates? Boost visibility for next time.
		// Only if we passed all the readiness checks (which we did to get here).
		j.incrementVisibilityBoost(ctx)
		return false
	}

	// Re-verify playability
	if !j.isPlayable(ctx, best) {
		// If best is not playable, we technically found something but rejected it.
		// Should we boost? Probably yes, effectively "nothing playable found".
		j.incrementVisibilityBoost(ctx)
		return false
	}

	strategy := prompt.DetermineSkewStrategy(best, j.poiMgr.(prompt.POIAnalyzer), t.IsOnGround)

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

// checkFlightStagePOI enforces flight stage restrictions for POI auto-narration.
func (j *NarrationJob) checkFlightStagePOI(t *sim.Telemetry) bool {
	switch t.FlightStage {
	case sim.StageAirborne, sim.StageClimb, sim.StageCruise, sim.StageDescend:
		// [NEW] Reinstate post-takeoff delay to allow 'letsgo' announcement to play
		// and prevent selecting low-value POIs immediately on rotate.
		takeOffTime := j.sim.GetLastTransition(sim.StageTakeOff)
		if !takeOffTime.IsZero() {
			delay := j.cfgProv.TakeoffDelay(context.Background())
			if time.Since(takeOffTime) < delay {
				slog.Debug("NarrationJob: Auto-narration suppressed during post-takeoff delay",
					"elapsed", time.Since(takeOffTime).Round(time.Second),
					"delay", delay)
				return false
			}
		}
		return true
	case sim.StageLanded:
		// Also allowed on ground for airport narration, but NOT once landed (until debriefed)
		// Wait, user said ONLY auto-select if in airborne, cruise, climb, descend.
		return false
	default:
		return false
	}
}

// checkFrequencyRules determines if we can fire based on frequency settings (1-4).
// Handles pipeline/overlap logic.
func (j *NarrationJob) checkFrequencyRules(ctx context.Context) bool {
	freq := j.cfgProv.NarrationFrequency(ctx)
	isPlaying := j.narrator.IsPlaying()

	// Strategies 1 (Rarely) & 2 (Normal): No Overlap
	if freq <= 2 {
		if isPlaying {
			return false
		}
		// If not playing, we are ready (pause is handled by IsPlaying hold)
		return true
	}

	// Strategies 3 (Active), 4 (Hyperactive): Allow Overlap (Pipeline)
	// If not playing, we are obviously ready.
	if !isPlaying {
		return true
	}

	// Calculate Lead Time Multiplier based on Frequency
	// Active (3)      -> 1.0x
	// Hyperactive (4) -> 2.0x
	var leadTimeMultiplier float64
	switch freq {
	case 3:
		leadTimeMultiplier = 1.0
	case 4:
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
func (j *NarrationJob) checkPreConditions(ctx context.Context, t *sim.Telemetry) bool {
	if !j.cfgProv.AutoNarrate(ctx) {
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

func (j *NarrationJob) isPlayable(ctx context.Context, p *model.POI) bool {
	// Check if already in pipeline (Generating, Queued, Playing)
	// This prevents the "double trigger" issue where a POI is selected again while generating/queued
	if j.narrator.IsPOIBusy(p.WikidataID) {
		return false
	}
	return !p.IsOnCooldown(j.cfgProv.RepeatTTL(ctx))
}

// hasEligiblePOI returns true if there is at least one visible POI candidate.
// This is used by checkEssayEligible to ensure essays are gap-fillers only.
func (j *NarrationJob) hasEligiblePOI(ctx context.Context, t *sim.Telemetry) bool {
	return j.getVisibleCandidate(ctx, t) != nil
}

// checkEssayEligible returns true if conditions for essay narration are met.
// Essays are gap-fillers: they only fire when there are NO visible POIs.
func (j *NarrationJob) checkEssayEligible(ctx context.Context, t *sim.Telemetry) bool {
	if !j.cfgProv.EssayEnabled(ctx) {
		return false
	}

	// Disable Essay in "Rarely" mode
	if j.cfgProv.NarrationFrequency(ctx) == 1 {
		return false
	}

	// PRIORITY RULE: Essays only fire when there are NO visible POIs
	// This is the core "gap filler" logic from v0.2.121
	if j.hasEligiblePOI(ctx, t) {
		return false
	}

	// Essay-specific cooldown (DelayBetweenEssays)
	if !j.lastEssayTime.IsZero() {
		if time.Since(j.lastEssayTime) < j.cfgProv.EssayDelayBetweenEssays(ctx) {
			return false
		}
	}

	// Global delay before essay (Time since last narration)
	// Must be quiet for at least DelayBeforeEssay
	delayBeforeEssay := j.cfgProv.EssayDelayBeforeEssay(ctx)
	if time.Since(j.lastTime) < delayBeforeEssay {
		return false
	}

	// Essay stage rules: same as POI but allows some extra padding maybe?
	// For now, let's keep it strictly to the same airborne stages
	if !j.checkFlightStagePOI(t) {
		return false
	}

	// Silence rule: at least 2x PauseDuration (Legacy check, maybe redundant now but safer to keep)
	minSilence := j.cfgProv.PauseDuration(ctx) * 2
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
func (j *NarrationJob) getVisibleCandidate(ctx context.Context, t *sim.Telemetry) *model.POI {
	minScorePtr := j.getPOIQueryThreshold(ctx)
	minScore := 0.0
	if minScorePtr != nil {
		minScore = *minScorePtr
	}

	if j.isCacheValid(t, minScore) {
		return j.cachedBest
	}
	j.updateCacheMetadata(t, minScore)

	if !j.cfgProv.LineOfSight(ctx) || j.losChecker == nil {
		j.cachedBest = j.getBestCandidateFallback(ctx, t)
		return j.cachedBest
	}

	candidates := j.poiMgr.GetNarrationCandidates(1000, minScorePtr)
	j.logCandidateScoping(candidates, t)

	if len(candidates) == 0 {
		return nil
	}

	j.cachedBest = j.findLOSVisibleCandidate(ctx, t, candidates)
	return j.cachedBest
}

func (j *NarrationJob) isCacheValid(t *sim.Telemetry, minScore float64) bool {
	if t == nil {
		return false
	}
	isPlaying := j.narrator.IsPlaying()
	isGenerating := j.narrator.IsGenerating()

	return t.Latitude == j.lastLat && t.Longitude == j.lastLon && t.AltitudeMSL == j.lastAlt &&
		isPlaying == j.lastIsPlaying && isGenerating == j.lastIsGenerating && minScore == j.lastMinScore
}

func (j *NarrationJob) updateCacheMetadata(t *sim.Telemetry, minScore float64) {
	if t != nil {
		j.lastLat = t.Latitude
		j.lastLon = t.Longitude
		j.lastAlt = t.AltitudeMSL
	}
	j.lastIsPlaying = j.narrator.IsPlaying()
	j.lastIsGenerating = j.narrator.IsGenerating()
	j.lastMinScore = minScore
	j.cachedBest = nil
}

func (j *NarrationJob) logCandidateScoping(candidates []*model.POI, t *sim.Telemetry) {
	if len(candidates) != j.lastCandidateCount {
		if len(candidates) == 0 {
			slog.Debug("NarrationJob: No candidates in range")
		} else {
			slog.Debug("NarrationJob: LOS checking candidates", "count", len(candidates), "aircraft_alt_ft", t.AltitudeMSL)
		}
		j.lastCandidateCount = len(candidates)
	}
}

func (j *NarrationJob) findLOSVisibleCandidate(ctx context.Context, t *sim.Telemetry, candidates []*model.POI) *model.POI {
	aircraftPos := geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	aircraftAltFt := t.AltitudeMSL

	var visibleCandidates []*model.POI
	for i, poi := range candidates {
		if poi.IsDeferred || !j.isPlayable(ctx, poi) || !j.isRarelyEligible(ctx, poi, t) {
			continue
		}

		if j.checkPOIInLOS(poi, aircraftPos, aircraftAltFt, i) {
			visibleCandidates = append(visibleCandidates, poi)
			if len(visibleCandidates) >= 3 {
				break
			}
		}
	}

	if len(visibleCandidates) == 0 {
		return nil
	}

	return j.selectBestCandidate(visibleCandidates)
}

// selectBestCandidate picks the most urgent among top candidates.
// Uses combined score (Score × Visibility) for ranking.
func (j *NarrationJob) selectBestCandidate(visibleCandidates []*model.POI) *model.POI {
	best := visibleCandidates[0]
	topCombined := best.Score * best.Visibility

	for i := 1; i < len(visibleCandidates); i++ {
		cand := visibleCandidates[i]
		candCombined := cand.Score * cand.Visibility

		// Tolerance threshold: 30% of top combined score
		if candCombined < topCombined*0.7 {
			continue
		}

		// If candidate is urgent (disappearing in < 5 mins) and more urgent than best
		if cand.TimeToBehind > 0 && (best.TimeToBehind == -1 || cand.TimeToBehind < best.TimeToBehind) {
			if cand.TimeToBehind < 300 { // Only swap if it's actually urgent
				slog.Info("NarrationJob: Selection swap (Urgency)", "original", best.DisplayName(), "urgent", cand.DisplayName(), "tto", cand.TimeToBehind)
				best = cand
			}
		}
	}

	return best
}

func (j *NarrationJob) getBestCandidateFallback(ctx context.Context, t *sim.Telemetry) *model.POI {
	slog.Debug("NarrationJob: LOS disabled or no checker", "los_enabled", j.cfgProv.LineOfSight(ctx), "checker_nil", j.losChecker == nil)
	minScore := j.getPOIQueryThreshold(ctx)
	// Get more candidates to filter out deferred ones
	cands := j.poiMgr.GetNarrationCandidates(10, minScore)
	for _, poi := range cands {
		if !poi.IsDeferred {
			return poi
		}
	}
	return nil
}

func (j *NarrationJob) getPOIQueryThreshold(ctx context.Context) *float64 {
	if j.cfgProv.FilterMode(ctx) != "adaptive" {
		val := j.cfgProv.MinScoreThreshold(ctx)

		// Apply visibility boost if enabled
		boost := j.getVisibilityBoost(ctx)
		if boost > 1.0 {
			threshold := val / boost
			return &threshold
		}

		return &val
	}
	return nil
}

func (j *NarrationJob) getVisibilityBoost(ctx context.Context) float64 {
	if j.store == nil {
		return 1.0
	}
	val, ok := j.store.GetState(ctx, "visibility_boost")
	if ok && val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 1.0
}

func (j *NarrationJob) isRarelyEligible(ctx context.Context, poi *model.POI, t *sim.Telemetry) bool {
	if j.cfgProv.NarrationFrequency(ctx) != 1 {
		return true
	}
	analyzer, ok := j.poiMgr.(prompt.POIAnalyzer)
	if !ok {
		return true
	}
	return prompt.DetermineSkewStrategy(poi, analyzer, t.IsOnGround) == prompt.StrategyMaxSkew
}

func (j *NarrationJob) checkPOIInLOS(poi *model.POI, aircraftPos geo.Point, aircraftAltFt float64, index int) bool {
	// Get POI ground elevation (meters -> feet)
	poiElevM, err := j.losChecker.GetElevation(poi.Lat, poi.Lon)
	if err != nil {
		slog.Debug("NarrationJob: LOS elevation error", "poi", poi.DisplayName(), "error", err)
		return false
	}
	poiAltFt := poiElevM * 3.28084 // meters to feet
	poiPos := geo.Point{Lat: poi.Lat, Lon: poi.Lon}

	// Check LOS with 0.5km step size
	isVisible := j.losChecker.IsVisible(aircraftPos, poiPos, aircraftAltFt, poiAltFt, 0.5)

	if isVisible {
		poi.LOSStatus = model.LOSVisible
	} else {
		poi.LOSStatus = model.LOSBlocked
	}
	return isVisible
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

func (j *NarrationJob) incrementVisibilityBoost(ctx context.Context) {
	if j.store == nil {
		return
	}

	// Don't boost visibility while on ground or below 500ft AGL
	// At low altitudes, visibility is naturally limited; boosting would select inappropriate POIs
	// TESTING: Hardcoded 500ft threshold - tune this value based on testing
	const boostThresholdFt = 500.0
	if j.lastAGL < boostThresholdFt {
		return
	}

	current := j.getVisibilityBoost(ctx)

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
