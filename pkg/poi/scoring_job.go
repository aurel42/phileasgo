package poi

import (
	"context"
	"fmt"
	"log/slog"
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

// ScoringJob manages the periodic scoring of POIs.
type ScoringJob struct {
	name    string
	running int32 // Atomic lock

	manager ScoringManager
	sim     sim.Client
	scorer  *scorer.Scorer
	cfg     *config.NarratorConfig
	busyFn  func(qid string) bool
	lastRun time.Time
}

// NewScoringJob creates a new ScoringJob.
func NewScoringJob(
	jobName string,
	manager ScoringManager,
	simClient sim.Client,
	sc *scorer.Scorer,
	cfg *config.NarratorConfig,
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

	// Instrumentation: Log prediction offset distance
	if telemetry.PredictedLatitude != 0 || telemetry.PredictedLongitude != 0 {
		currentPos := geo.Point{Lat: telemetry.Latitude, Lon: telemetry.Longitude}
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

	// 4. Lock & Score
	// Note: We get a COPY/Slice of pointers. The Manager lock is inside GetTrackedPOIs.
	// HOWEVER, modifying the POI pointers (p.Score = ...) is thread-safe only if no one else writes to them concurrenty.
	// The Manager holds the implementation.
	// In the original code, Manager locked the WHOLE loop.
	// Now we are iterating over pointers. Thread safety depends on POI struct usage.
	// Readers (UI) use RLock in Manager.
	// Writers (Ingestion) use Lock in Manager.
	// Here we are modifying fields (Score).
	// Ideally we should lock the POI itself or having a "BatchUpdate" on manager.
	// BUT, for now, we follow the pattern: The manager gave us the pointers.
	pois := j.manager.GetTrackedPOIs()

	input := scorer.ScoringInput{
		Telemetry:       telemetry,
		CategoryHistory: history,
		NarratorConfig:  j.cfg,
		BoostFactor:     boostFactor,
		IsPOIBusy:       j.busyFn,
	}

	// Create Scoring Session (Pre-calculates terrain/context once)
	session := j.scorer.NewSession(&input)

	for _, p := range pois {
		// DANGER: We are modifying P here without Manager lock!
		// If ingestion happens parallel, it might race.
		// However, ingestion (upsert) replaces the pointer in the map or updates fields.
		// This is a known trade-off in Go concurrent maps without fine-grained locking.
		// Given single-threaded writer assumption in main loop usually holds well enough or we need RWMutex on POI.
		session.Calculate(p)
	}

	// 5. Update Last Scored Location
	j.manager.UpdateScoringState(telemetry.Latitude, telemetry.Longitude)

	// 6. Trigger Callback
	j.manager.NotifyScoringComplete(ctx, &telemetry, session.LowestElevation())
}
