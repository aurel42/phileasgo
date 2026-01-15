package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/terrain"
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
	lastTime   time.Time

	wasBusy          bool
	lastEssayTime    time.Time
	lastCheckedCount int

	// Post-takeoff grace period tracking
	hasReachedClimbAlt bool    // True once we've reached climb threshold after takeoff
	lastAGL            float64 // Last known AGL for visibility boost check
}

func NewNarrationJob(cfg *config.Config, n narrator.Service, pm POIProvider, simC sim.Client, st store.Store, los *terrain.LOSChecker) *NarrationJob {
	j := &NarrationJob{
		BaseJob:    NewBaseJob("Narration"),
		cfg:        cfg,
		narrator:   n,
		poiMgr:     pm,
		sim:        simC,
		store:      st,
		losChecker: los,
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

func (j *NarrationJob) ShouldFire(t *sim.Telemetry) bool {
	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}

	// 0. Pre-flight checks
	if !j.checkPreConditions(t) {
		return false
	}

	// Track AGL for visibility boost decisions
	j.lastAGL = t.AltitudeAGL

	// Post-takeoff grace period logic
	// TESTING: Hardcoded 500ft threshold - tune this value based on testing
	const climbThresholdFt = 500.0

	if t.IsOnGround {
		// Reset climb flag when on ground
		j.hasReachedClimbAlt = false
	} else if !j.hasReachedClimbAlt {
		// Just took off, check if we've reached the threshold
		if t.AltitudeAGL >= climbThresholdFt {
			j.hasReachedClimbAlt = true
			slog.Debug("NarrationJob: Reached climb threshold, enabling in-flight POI selection", "agl", t.AltitudeAGL)
		} else {
			// Haven't reached threshold yet - skip non-airport POI selection
			// Airport narration on ground is handled separately (ground candidates)
			slog.Debug("NarrationJob: Post-takeoff grace period", "agl", t.AltitudeAGL, "threshold", climbThresholdFt)
			return false
		}
	}

	// 1. Narrator Activity Check
	if !j.checkNarratorReady() {
		return false
	}

	// 2. Frequency & Pipeline Logic
	// Replaces legacy Cooldown logic.
	// The inter-narration pause is now handled by the Audio Service holding IsPlaying=true.
	if !j.checkFrequencyRules() {
		return false
	}

	// 3. POI Selection (Dynamic Check)
	// We ask the Manager for *any* candidate that meets the criteria.
	// We use limit=1 just to see if one exists.
	var minScore *float64
	if j.getFilterMode() != "adaptive" {
		val := j.getMinScore()
		minScore = &val
	}

	candidates := j.poiMgr.GetNarrationCandidates(10, minScore, t.IsOnGround)

	// Apply "Rarely" Strategy Filter (Local Hero / Lone Wolf)
	// Only accept POIs that are dominant in their area.
	if j.getNarrationFrequency() == 1 { // Rarely
		filtered := make([]*model.POI, 0, len(candidates))
		analyzer, ok := j.poiMgr.(narrator.POIAnalyzer)
		if !ok {
			slog.Error("NarrationJob: Failed to cast poiMgr to POIAnalyzer", "type", fmt.Sprintf("%T", j.poiMgr))
		}
		if ok {
			for _, p := range candidates {
				// We reuse DetermineSkewStrategy to find "Lone Wolf" candidates
				// StrategyMaxSkew is returned when a POI has no strong competitors
				if narrator.DetermineSkewStrategy(p, analyzer, t.IsOnGround) == narrator.StrategyMaxSkew {
					filtered = append(filtered, p)
				}
			}
			candidates = filtered
		}
	}

	if len(candidates) > 0 {
		return true
	}

	// No candidates found? Boost visibility!
	// Only boost if we were actually ready to narrate (passed all checks)
	// and we are NOT in essay fallback mode (which might happen next).
	// Actually, if we boost, we might find something next time.
	// We increment boost, and return false (so we don't fire Narration yet, unless Essay triggers).
	// Essay trigger is separate.
	j.incrementVisibilityBoost(context.Background())

	// 4. Essay fallback
	// Don't pipeline essays for now (keeps it simple)
	if j.narrator.IsPlaying() {
		return false
	}
	return j.checkEssayEligible(t)
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
	if p.LastPlayed.IsZero() {
		return true
	}
	return time.Since(p.LastPlayed) >= time.Duration(j.cfg.Narrator.RepeatTTL)
}

// checkEssayEligible returns true if conditions for essay narration are met.
func (j *NarrationJob) checkEssayEligible(t *sim.Telemetry) bool {
	if !j.cfg.Narrator.Essay.Enabled {
		return false
	}

	// Essay-specific cooldown
	if !j.lastEssayTime.IsZero() {
		if time.Since(j.lastEssayTime) < time.Duration(j.cfg.Narrator.Essay.Cooldown) {
			return false
		}
	}

	// Silence rule: at least 2x PauseDuration
	// We use PauseDuration as the base unit for silence
	minSilence := time.Duration(j.cfg.Narrator.PauseDuration) * 2
	if time.Since(j.lastTime) < minSilence {
		return false
	}

	// Altitude check
	if t.AltitudeAGL < 2000 {
		return false
	}

	slog.Debug("NarrationJob: Essay eligible (Silence & Cooldown met)")
	return true
}

func (j *NarrationJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	// 0. Check for Staged/Prepared Narrative (Pipeline)
	// If the narrator has a POI ready (or generating), we MUST play that one
	// to avoid the "Jumping Beacon" issue (Scheduler calculating X while Narrator plays Y).
	if staged := j.narrator.GetPreparedPOI(); staged != nil {
		slog.Info("NarrationJob: Activating staged narrative", "poi", staged.DisplayName())
		// We call PlayPOI with the STAGED ID.
		// PlayPOI will see the ID, set the beacon correctly, and then (re)discover the staged content.
		// Strategy is less relevant here as it's already baked into the narrative, but we pass "uniform" or reuse.
		j.narrator.PlayPOI(ctx, staged.WikidataID, false, t, narrator.StrategyUniform)
		return
	}

	// Determine Min Score dynamically (for Phase 2 fallback logic)
	// Note: checking logic is mostly in ShouldFire, but we re-check here.
	var minScore *float64
	if j.getFilterMode() != "adaptive" {
		val := j.getMinScore()
		minScore = &val
	}

	candidates := j.poiMgr.GetNarrationCandidates(10, minScore, t.IsOnGround)

	// Re-apply "Rarely" filter
	if j.getNarrationFrequency() == 1 {
		filtered := make([]*model.POI, 0, len(candidates))
		analyzer, ok := j.poiMgr.(narrator.POIAnalyzer)
		if ok {
			for _, p := range candidates {
				if narrator.DetermineSkewStrategy(p, analyzer, t.IsOnGround) == narrator.StrategyMaxSkew {
					filtered = append(filtered, p)
				}
			}
			candidates = filtered
		}
	}

	// Pick best (first visible)
	var best *model.POI
	// Reuse logic from getVisibleCandidate if possible, but we need to pass candidates list
	// Since getVisibleCandidate refetches, we might duplicate work or diverge.
	// Let's rely on getVisibleCandidate but UPDATE it to support the list or just call it directly.
	// Actually, getVisibleCandidate DOES re-fetch.
	// To ensure consistency, we should use getVisibleCandidate's logic but applied to our pre-filtered criteria?
	// Or just call getVisibleCandidate and assume it does the same thing?
	// getVisibleCandidate checks 1000 items. ShouldFire checked 10.

	// Correct approach:
	// Use getVisibleCandidate but make it robust to Frequency logic.
	// getVisibleCandidate needs to valid candidates.
	best = j.getVisibleCandidate(t)

	// If best is nil, try essay directly
	if best == nil {
		j.tryEssay(ctx, t)
		return
	}

	// Re-verify threshold (Dynamic) and playability
	// State might have changed or getVisibleCandidate might be used differently
	if !j.isPlayable(best) {
		j.tryEssay(ctx, t)
		return
	}

	// Note: getVisibleCandidate already respects dynamic minScore from config
	// But it does NOT respect the "Rarely" filter yet.
	// We need to inject that into getVisibleCandidate or check it here.
	if j.getNarrationFrequency() == 1 {
		analyzer, ok := j.poiMgr.(narrator.POIAnalyzer)
		if ok && narrator.DetermineSkewStrategy(best, analyzer, t.IsOnGround) != narrator.StrategyMaxSkew {
			// Best visible candidate is NOT a Lone Wolf, so skip.
			// Try Essay? Or just return?
			// ShouldFire said "true", implying SOME candidate was valid.
			// But getVisibleCandidate might have picked a different one (visible vs score).
			// If the visible one isn't Lone Wolf, we shouldn't play it in "Rarely" mode.
			j.tryEssay(ctx, t)
			return
		}
	}

	strategy := narrator.DetermineSkewStrategy(best, j.poiMgr.(narrator.POIAnalyzer), t.IsOnGround)
	// No more cooldown calculation here!

	// Get current boost for logging
	currentBoost := j.getBoostFactor()

	slog.Info("NarrationJob: Triggering narration",
		"name", best.DisplayName(),
		"score", fmt.Sprintf("%.2f", best.Score),
		"boost", fmt.Sprintf("x%.1f", currentBoost),
		"freq", j.getNarrationFrequency(),
	)

	// Successful narration selection -> Reset Boost
	j.resetVisibilityBoost(ctx)

	// Pipeline vs Direct Play
	if j.narrator.IsPlaying() {
		if err := j.narrator.PrepareNextNarrative(ctx, best.WikidataID, strategy, t); err != nil {
			slog.Error("NarrationJob: Pipeline preparation failed", "error", err)
		}
	} else {
		j.narrator.PlayPOI(ctx, best.WikidataID, false, t, strategy)
	}
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
		slog.Warn("NarrationJob: All POIs blocked by LOS", "checked", checkedCount, "total_candidates", len(candidates))
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

func (j *NarrationJob) tryEssay(ctx context.Context, t *sim.Telemetry) {
	// Re-check eligibility (Silence, Cooldown, Altitude)
	// This is critical because Run() can fall through here even if ShouldFire() triggered for a POI candidate
	// (e.g. if the candidate turned out to be invisible). In that case, we MUST re-verify essay rules
	// to prevent bypassing startup silence.
	if !j.checkEssayEligible(t) {
		return
	}

	if j.narrator.PlayEssay(ctx, t) {
		// On success, update timers
		now := time.Now()
		j.lastEssayTime = now
		j.lastTime = now

		// We revert to standard cooldown/strategy for the *scheduler* to wake up.
		// The essay cooldown is handled explicitly in ShouldFire via lastEssayTime.
		// j.calculateCooldown(1.0, narrator.StrategyUniform) // Removed in Phase 2
	}
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

func (j *NarrationJob) getBoostFactor() float64 {
	if j.store == nil {
		return 1.0
	}
	val, ok := j.store.GetState(context.Background(), "visibility_boost")
	if ok && val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 1.0
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
