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
	cfg              *config.Config
	narrator         narrator.Service
	poiMgr           POIProvider
	sim              sim.Client
	store            store.Store
	losChecker       *terrain.LOSChecker
	lastTime         time.Time
	nextFireDuration time.Duration
	wasBusy          bool
	lastEssayTime    time.Time
	lastCheckedCount int
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
	j.calculateCooldown(1.0, narrator.StrategyUniform)
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
		slog.Debug("NarrationJob: Cooldown started", "duration", j.nextFireDuration)
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

	// 1. Narrator Activity Check
	if !j.checkNarratorReady() {
		return false
	}

	// 2. Cooldown & Pipeline Check
	if j.narrator.IsPlaying() {
		j.wasBusy = true // Mark as busy so we detect falling edge later

		// PIPELINE LOGIC
		// Fire if: Remaining + Cooldown <= AvgLatency
		remaining := j.narrator.Remaining()
		avgLat := j.narrator.AverageLatency()
		cooldown := j.nextFireDuration

		// If cooldown is huge (e.g. 5 mins) and latency is small (10s),
		// we don't want to generate 5 mins early.
		// We want to generate when we are close to the target start time.
		// Target Start Time = Now + Remaining + Cooldown
		// We need generation to finish at Target Start Time.
		// So we start when TimeToTarget <= Latency.
		// TimeToTarget = Remaining + Cooldown.

		if remaining+cooldown > avgLat {
			// Too early
			return false
		}

	} else if !j.checkCooldown() {
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

	candidates := j.poiMgr.GetNarrationCandidates(1, minScore, t.IsOnGround)
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

	// Silence rule: at least 2x CooldownMax
	minSilence := time.Duration(j.cfg.Narrator.CooldownMax) * 2
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

	best := j.getVisibleCandidate(t)
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

	if j.getFilterMode() != "adaptive" {
		if best.Score < j.getMinScore() {
			j.tryEssay(ctx, t)
			return
		}
	}

	strategy := narrator.DetermineSkewStrategy(best, j.poiMgr.(narrator.POIAnalyzer))
	j.calculateCooldown(1.0, strategy)

	// Get current boost for logging
	currentBoost := j.getBoostFactor()

	slog.Info("NarrationJob: Triggering narration",
		"name", best.DisplayName(),
		"score", fmt.Sprintf("%.2f", best.Score),
		"boost", fmt.Sprintf("x%.1f", currentBoost),
		"cooldown_after", j.nextFireDuration,
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

func (j *NarrationJob) checkCooldown() bool {
	if j.lastTime.IsZero() {
		return true // No previous run, so ready
	}

	// If skip requested, bypass timer
	if j.narrator.ShouldSkipCooldown() {
		j.narrator.ResetSkipCooldown()
		slog.Info("NarrationJob: Skipping cooldown check by user request")
		return true
	}

	elapsed := time.Since(j.lastTime)

	// Use the randomized duration
	// If nextFireDuration is 0 (first run or not set), default to Min
	required := j.nextFireDuration
	if required == 0 {
		required = time.Duration(j.cfg.Narrator.CooldownMin)
	}

	// PRE-FETCH COMPENSATION:
	// We want the NEXT playback to START at (lastTime + required).
	// Generation takes 'latency'.
	// So we must trigger generation at (lastTime + required - latency).
	latency := j.narrator.AverageLatency()
	if latency < required {
		required -= latency
	} else {
		required = 0 // Latency > Cooldown implies we should have pipelined (or fire immediately now)
	}

	return elapsed >= required
}

func (j *NarrationJob) tryEssay(ctx context.Context, t *sim.Telemetry) {
	// Check if essays are enabled
	if !j.cfg.Narrator.Essay.Enabled {
		return
	}

	// Safety check: only above 2000ft AGL
	if t != nil && t.AltitudeAGL < 2000 {
		return
	}

	if j.narrator.PlayEssay(ctx, t) {
		// On success, update timers
		now := time.Now()
		j.lastEssayTime = now
		j.lastTime = now

		// We revert to standard cooldown/strategy for the *scheduler* to wake up.
		// The essay cooldown is handled explicitly in ShouldFire via lastEssayTime.
		j.calculateCooldown(1.0, narrator.StrategyUniform)
	}
}

func (j *NarrationJob) calculateCooldown(multiplier float64, strategy string) {
	cMin := int64(j.cfg.Narrator.CooldownMin)
	cMax := int64(j.cfg.Narrator.CooldownMax)

	var base int64
	if cMax <= cMin {
		base = cMin
	} else {
		delta := cMax - cMin
		// Use simple random based on time to avoid seeding math/rand if not needed
		base = cMin + (time.Now().UnixNano() % delta)
	}
	j.nextFireDuration = time.Duration(float64(base) * multiplier)
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
