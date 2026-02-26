package poi

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync/atomic"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/scorer"
	"phileasgo/pkg/sim"
)

// ScoringManager defines the specialized interface required by the ScoringJob.
// It limits the job's access to only what it needs from the POI Manager.
type ScoringManager interface {
	// UpdateScoringState updates the last scored position.
	UpdateScoringState(lat, lon float64)
	// NotifyScoringComplete triggers callbacks after a scoring pass.
	NotifyScoringComplete(ctx context.Context, t *sim.Telemetry, lowestElev float64)
	// GetTrackedPOIs returns the list of current POIs to score.
	GetTrackedPOIs() []*model.POI
	// FetchHistory gets recently played items for variety scoring.
	FetchHistory(ctx context.Context) ([]string, error)
	// GetBoostFactor gets the visibility boost factor from store/state.
	GetBoostFactor(ctx context.Context) float64
}

// minScoringDisplacementM is the minimum distance the aircraft must move before
// a new scoring pass runs. Derived from 50 kts × 5 s ≈ 128 m. This avoids
// re-scoring all POIs when the aircraft is parked or taxiing slowly.
const minScoringDisplacementM = 128.0

// ScoringJob manages the periodic scoring of POIs.
type ScoringJob struct {
	name    string
	running int32 // Atomic lock

	manager ScoringManager
	sim     sim.Client
	scorer  *scorer.Scorer
	cfg     config.Provider
	busyFn  func(qid string) bool
	lastRun time.Time

	// State from the last full scoring pass, used to skip redundant passes.
	lastScoredPos   geo.Point
	lastScoredCount int
	hasScoredOnce   bool
}

// NewScoringJob creates a new ScoringJob.
func NewScoringJob(
	jobName string,
	manager ScoringManager,
	simClient sim.Client,
	sc *scorer.Scorer,
	cfg config.Provider,
	busyFn func(qid string) bool,
	logger *slog.Logger, // Optional
) *ScoringJob {
	return &ScoringJob{
		name:    jobName,
		manager: manager,
		sim:     simClient,
		scorer:  sc,
		cfg:     cfg,
		busyFn:  busyFn,
		lastRun: time.Now(),
	}
}

// Name returns the job name.
func (j *ScoringJob) Name() string {
	return j.name
}

// ShouldFire returns true if 5 seconds have passed since the last run.
func (j *ScoringJob) ShouldFire(t *sim.Telemetry) bool {
	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}
	// Strict 5s interval check
	return time.Since(j.lastRun) >= 5*time.Second
}

func (j *ScoringJob) TryLock() bool {
	return atomic.CompareAndSwapInt32(&j.running, 0, 1)
}

func (j *ScoringJob) Unlock() {
	atomic.StoreInt32(&j.running, 0)
}

// Run executes the scoring logic.
func (j *ScoringJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	// Update lastRun at the START of execution to maintain interval consistency?
	// Or end? Ticker usually fires at intervals.
	j.lastRun = time.Now()

	j.performScoringPass(ctx)
}

func (j *ScoringJob) performScoringPass(ctx context.Context) {
	// Skip scoring if sim is not active
	if j.sim.GetState() != sim.StateActive {
		return
	}

	// 1. Get Telemetry
	telemetry, err := j.sim.GetTelemetry(ctx)
	if err != nil {
		if err != sim.ErrWaitingForTelemetry {
			slog.Warn("ScoringJob: Failed to get telemetry", "error", err)
		}
		return
	}

	// Get tracked POIs once (used for both the skip-check and the scoring loop).
	pois := j.manager.GetTrackedPOIs()

	// Skip scoring if nothing meaningful changed since the last pass:
	// the aircraft hasn't moved enough AND the POI set hasn't changed.
	currentPos := geo.Point{Lat: telemetry.Latitude, Lon: telemetry.Longitude}
	if j.hasScoredOnce {
		displacement := geo.Distance(j.lastScoredPos, currentPos)
		countChanged := len(pois) != j.lastScoredCount
		if displacement < minScoringDisplacementM && !countChanged {
			return
		}
	}

	// Instrumentation: Log prediction offset distance
	if telemetry.PredictedLatitude != 0 || telemetry.PredictedLongitude != 0 {
		predictedPos := geo.Point{Lat: telemetry.PredictedLatitude, Lon: telemetry.PredictedLongitude}
		predDistMeters := geo.Distance(currentPos, predictedPos)
		predDistNM := predDistMeters / 1852.0
		logging.Trace(slog.Default(), "Scoring: Prediction offset",
			"dist_nm", fmt.Sprintf("%.2f", predDistNM),
			"groundspeed_kts", fmt.Sprintf("%.0f", telemetry.GroundSpeed),
		)
	}

	// 2. Fetch History for Variety Scoring
	history, err := j.manager.FetchHistory(ctx)
	if err != nil {
		slog.Warn("ScoringJob: Failed to fetch recent history", "error", err)
		history = []string{}
	}

	// 3. Fetch Boost Factor
	boostFactor := j.manager.GetBoostFactor(ctx)

	input := scorer.ScoringInput{
		Telemetry:       telemetry,
		CategoryHistory: history,
		RepeatTTL:       j.cfg.RepeatTTL(ctx),
		BoostFactor:     boostFactor,
		IsPOIBusy:       j.busyFn,
	}

	// Create Scoring Session (Pre-calculates terrain/context once)
	session := j.scorer.NewSession(&input)

	// Distance pre-filter: skip POIs clearly beyond max visibility range.
	// Use predicted position (same reference the scorer uses internally).
	const distancePaddingNM = 5.0
	maxDistNM := session.MaxRadiusNM() + distancePaddingNM
	predPos := geo.Point{Lat: telemetry.PredictedLatitude, Lon: telemetry.PredictedLongitude}
	if predPos.Lat == 0 && predPos.Lon == 0 {
		predPos = currentPos
	}

	// Phase 1: Score all POIs (visibility + intrinsic, no deferral).
	for _, p := range pois {
		poiDistNM := geo.Distance(predPos, geo.Point{Lat: p.Lat, Lon: p.Lon}) / 1852.0
		if poiDistNM > maxDistNM {
			p.IsVisible = false
			p.Score = 0
			p.Visibility = 0
			continue
		}
		session.Calculate(p)
	}

	// Phase 2: Lazy deferral — only compute for POIs that would be visible
	// on the map. Deferral involves 9 future-position visibility checks per
	// POI, so restricting it to actual candidates saves significant CPU.
	j.applyLazyDeferral(ctx, session, pois)

	// 5. Update Last Scored State
	j.lastScoredPos = currentPos
	j.lastScoredCount = len(pois)
	j.hasScoredOnce = true
	j.manager.UpdateScoringState(telemetry.Latitude, telemetry.Longitude)

	// 6. Trigger Callback
	j.manager.NotifyScoringComplete(ctx, &telemetry, session.LowestElevation())
}

// applyLazyDeferral computes deferral only for POIs that would actually appear
// on the map, respecting the current visibility settings (fixed score threshold
// or adaptive top-N count).
func (j *ScoringJob) applyLazyDeferral(ctx context.Context, session scorer.Session, pois []*model.POI) {
	// Collect visible candidates with positive scores.
	visible := make([]*model.POI, 0, 64)
	for _, p := range pois {
		if p.IsVisible && p.Score > 0 {
			visible = append(visible, p)
		}
	}
	if len(visible) == 0 {
		return
	}

	// Sort by combined score descending (same ranking the UI/narrator uses).
	combinedScore := func(p *model.POI) float64 { return p.Score * p.Visibility }
	sort.Slice(visible, func(i, k int) bool {
		return combinedScore(visible[i]) > combinedScore(visible[k])
	})

	// Determine candidate count based on the current filter mode.
	limit := len(visible)
	filterMode := j.cfg.FilterMode(ctx)
	if filterMode == "adaptive" {
		target := j.cfg.TargetPOICount(ctx)
		if target > 0 && target < limit {
			limit = target
		}
	} else {
		// Fixed mode: only POIs above the minimum score threshold.
		minScore := j.cfg.MinScoreThreshold(ctx)
		for i, p := range visible {
			if combinedScore(p) < minScore {
				limit = i
				break
			}
		}
	}

	for _, p := range visible[:limit] {
		session.CalculateDeferral(p)
	}
}
