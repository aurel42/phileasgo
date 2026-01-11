package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/terrain"
)

// POIProvider matches the GetBestCandidate method used by NarrationJob.
type POIProvider interface {
	GetBestCandidate(isOnGround bool) *model.POI
	GetCandidates(limit int, isOnGround bool) []*model.POI
	LastScoredPosition() (lat, lon float64)
}

// NarrationJob triggers AI narration for the best available POI.
type NarrationJob struct {
	BaseJob
	cfg              *config.Config
	narrator         narrator.Service
	poiMgr           POIProvider
	sim              sim.Client
	losChecker       *terrain.LOSChecker
	lastTime         time.Time
	nextFireDuration time.Duration
	wasBusy          bool
	lastEssayTime    time.Time
	lastCheckedCount int
}

func NewNarrationJob(cfg *config.Config, n narrator.Service, pm POIProvider, simC sim.Client, los *terrain.LOSChecker) *NarrationJob {
	j := &NarrationJob{
		BaseJob:    NewBaseJob("Narration"),
		cfg:        cfg,
		narrator:   n,
		poiMgr:     pm,
		sim:        simC,
		losChecker: los,
		lastTime:   time.Now(),
	}
	j.calculateCooldown(1.0, narrator.StrategyUniform)
	return j
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

	// 2. Cooldown Check
	if !j.checkCooldown() {
		return false
	}

	// 3. POI Selection - Priority over essays
	if j.hasViablePOI(t) {
		return true
	}

	// 4. Essay fallback
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

// checkNarratorReady returns true if the narrator is not playing, paused, or generating.
func (j *NarrationJob) checkNarratorReady() bool {
	if j.narrator.IsPlaying() {
		j.wasBusy = true
		return false
	}

	if j.narrator.IsPaused() {
		return false
	}

	// Playback just finished - start cooldown
	if j.wasBusy {
		j.wasBusy = false
		j.lastTime = time.Now()
		slog.Debug("NarrationJob: Cooldown started", "duration", j.nextFireDuration)
		return false
	}

	if j.narrator.IsActive() {
		slog.Debug("NarrationJob: Narrator still active")
		return false
	}

	return true
}

// hasViablePOI returns true if there's a POI above the score threshold and is playable.
func (j *NarrationJob) hasViablePOI(t *sim.Telemetry) bool {
	best := j.poiMgr.GetBestCandidate(t.IsOnGround)
	if best == nil {
		return false
	}

	// 1. Playable check (RepeatTTL)
	if !j.isPlayable(best) {
		slog.Debug("NarrationJob: Best POI on cooldown", "poi", best.DisplayName(), "last_played", best.LastPlayed)
		return false
	}

	// 2. Score check
	if best.Score < j.cfg.Narrator.MinScoreThreshold {
		slog.Debug("NarrationJob: Best POI below threshold", "poi", best.DisplayName(), "score", best.Score, "threshold", j.cfg.Narrator.MinScoreThreshold)
		return false
	}

	slog.Debug("NarrationJob: Viable POI found", "poi", best.DisplayName())
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

	best := j.getVisibleCandidate(t)
	// If best is nil, try essay directly
	if best == nil {
		j.tryEssay(ctx, t)
		return
	}

	// Re-verify threshold and playability just in case state changed between ShouldFire and Run
	if best.Score < j.cfg.Narrator.MinScoreThreshold || !j.isPlayable(best) {
		// Try Essay
		j.tryEssay(ctx, t)
		return
	}

	slog.Info("NarrationJob: Triggering narration", "name", best.DisplayName(), "score", fmt.Sprintf("%.2f", best.Score))

	strategy := narrator.DetermineSkewStrategy(best, j.poiMgr.(narrator.POIAnalyzer))
	j.narrator.PlayPOI(ctx, best.WikidataID, false, t, strategy)
	j.calculateCooldown(1.0, strategy)
}

// getVisibleCandidate returns the highest-scoring POI that has line-of-sight.
// If LOS is disabled or no checker is available, falls back to GetBestCandidate.
func (j *NarrationJob) getVisibleCandidate(t *sim.Telemetry) *model.POI {
	// If LOS is disabled or checker unavailable, use simple best candidate
	if !j.cfg.Terrain.LineOfSight || j.losChecker == nil {
		slog.Debug("NarrationJob: LOS disabled or no checker", "los_enabled", j.cfg.Terrain.LineOfSight, "checker_nil", j.losChecker == nil)
		return j.poiMgr.GetBestCandidate(t.IsOnGround)
	}

	// Get ALL candidates sorted by score (no arbitrary limit)
	candidates := j.poiMgr.GetCandidates(1000, t.IsOnGround)
	slog.Debug("NarrationJob: LOS checking candidates", "count", len(candidates), "aircraft_alt_ft", t.AltitudeMSL)

	aircraftPos := geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	aircraftAltFt := t.AltitudeMSL

	checkedCount := 0
	for i, poi := range candidates {
		// Optimization: Skip POIs that don't meet the minimum score threshold.
		if poi.Score < j.cfg.Narrator.MinScoreThreshold {
			break
		}

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
