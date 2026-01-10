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

// POIProvider matches the GetBestCandidate method used by jobs.
type POIProvider interface {
	GetBestCandidate() *model.POI
	GetCandidates(limit int) []*model.POI
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

	// 0. Global Switch
	if !j.cfg.Narrator.AutoNarrate {
		slog.Debug("NarrationJob: AutoNarrate disabled")
		return false
	}

	// 0.5 Location Consistency Check
	if !j.isLocationConsistent(t) {
		slog.Debug("NarrationJob: Location inconsistent")
		return false
	}

	// 0.6 Sim State Check (Ground/Inactive)
	if j.sim.GetState() != sim.StateActive {
		// Even if we are technically "Active" in SimConnect, if the camera state implies inactive (Menu), we skip.
		// However, GetState() already handles this logic via UpdateState().
		return false
	}

	// 0.7 Ground Logic
	// If on ground, only narrate if very close (e.g. 2km) - likely the airport itself.
	if t.IsOnGround {
		// We can't check distance to "best" yet because we haven't selected it.
		// But we can defer this check to after selection?
		// Or we check it here: If we are on ground, we ENFORCE that any candidate must be < 2km.
		// Let's allow ShouldFire to proceed, and filter in Selection?
		// Or easier: ShouldFire returns true, but we filter candidates in Run/getVisibleCandidate.
		// Actuality: ShouldFire is a gate. If we return true here, we commit to TRYING to narrate.
		// If we are on ground, we should peeking at the candidate to see if it's eligible.
		best := j.poiMgr.GetBestCandidate()
		if best == nil {
			return false
		}
		dist := geo.Distance(geo.Point{Lat: t.Latitude, Lon: t.Longitude}, geo.Point{Lat: best.Lat, Lon: best.Lon})
		if dist > 5000.0 { // 5km
			// too far for ground narration
			return false
		}
	}

	// 1. Narrator Activity Check (Playback Aware)
	isPlaying := j.narrator.IsPlaying()

	if isPlaying {
		j.wasBusy = true
		return false
	}

	// 1b. Pause Check (Global Use Pause)
	if j.narrator.IsPaused() {
		return false
	}

	// If it WAS playing but now isn't, playback just finished.
	// Reset the timer to now, so cooldown starts counting from THIS moment.
	if j.wasBusy {
		j.wasBusy = false
		j.lastTime = time.Now()
		slog.Debug("NarrationJob: Cooldown started", "duration", j.nextFireDuration)
		return false
	}

	// If narrator is busy generating (IsActive but maybe not IsPlaying yet?), skip.
	if j.narrator.IsActive() {
		slog.Debug("NarrationJob: Narrator still active")
		return false
	}

	// 2. Cooldown Check
	if !j.checkCooldown() {
		return false
	}

	// 3. Selection
	best := j.poiMgr.GetBestCandidate()
	var bestIsViable bool
	if best != nil {
		if best.Score >= j.cfg.Narrator.MinScoreThreshold {
			bestIsViable = true
		} else {
			slog.Debug("NarrationJob: Best POI below threshold", "poi", best.DisplayName(), "score", best.Score, "threshold", j.cfg.Narrator.MinScoreThreshold)
		}
	}

	// PRIORITY RULE:
	// If we have a viable POI, we fire (to narrate the POI).
	// If NO viable POI, we consider an Essay, but ONLY if silence rules are met.

	if bestIsViable {
		slog.Debug("NarrationJob: Viable POI found", "poi", best.DisplayName())
		return true
	}

	// --- Essay Logic ---
	// If no viable POI, check if we can fill the gap with an Essay.

	if !j.cfg.Narrator.Essay.Enabled {
		return false
	}

	// Check ESSAY-SPECIFIC Cooldown (distinct from general cooldown)
	if !j.lastEssayTime.IsZero() {
		elapsedEssay := time.Since(j.lastEssayTime)
		if elapsedEssay < time.Duration(j.cfg.Narrator.Essay.Cooldown) {
			// slog.Debug("NarrationJob: Essay cooldown active", "remaining", time.Duration(j.cfg.Narrator.Essay.Cooldown)-elapsedEssay)
			return false
		}
	}

	// SILENCE RULE:
	// Only trigger essay if there has been silence for at least 2 * CooldownMax
	// This ensures essays act as "gap fillers" and don't crowd the timeline.
	minSilence := time.Duration(j.cfg.Narrator.CooldownMax) * 2
	if time.Since(j.lastTime) < minSilence {
		// slog.Debug("NarrationJob: Essay suppressed (not enough silence)", "silence", time.Since(j.lastTime), "required", minSilence)
		return false
	}

	// ESSAYS: only above 2000ft AGL.
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

	// Re-verify threshold just in case score changed between ShouldFire and Run
	if best.Score < j.cfg.Narrator.MinScoreThreshold {
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
		return j.poiMgr.GetBestCandidate()
	}

	// Get ALL candidates sorted by score (no arbitrary limit)
	candidates := j.poiMgr.GetCandidates(1000)
	slog.Debug("NarrationJob: LOS checking candidates", "count", len(candidates), "aircraft_alt_ft", t.AltitudeMSL)

	aircraftPos := geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	aircraftAltFt := t.AltitudeMSL

	checkedCount := 0
	for i, poi := range candidates {
		// Optimization: Skip POIs that don't meet the minimum score threshold.
		// Since the list is sorted by Score (descending), if we hit one below threshold,
		// all subsequent ones are also below threshold.
		if poi.Score < j.cfg.Narrator.MinScoreThreshold {
			break
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

// TelemetrySink is an interface for consumers of the high-frequency telemetry stream.
type TelemetrySink interface {
	Update(t *sim.Telemetry)
	UpdateState(s sim.State)
}

// Scheduler manages the central heartbeat and scheduled jobs.
type Scheduler struct {
	cfg         *config.Config
	sim         sim.Client
	sink        TelemetrySink
	jobs        []Job
	resettables []SessionResettable
	lastTickPos geo.Point
}

// NewScheduler creates a new Scheduler.
func NewScheduler(cfg *config.Config, simClient sim.Client, sink TelemetrySink) *Scheduler {
	return &Scheduler{
		cfg:         cfg,
		sim:         simClient,
		sink:        sink,
		jobs:        []Job{},
		resettables: []SessionResettable{},
	}
}

// AddResettable registers a component to be reset on session change (teleport).
func (s *Scheduler) AddResettable(r SessionResettable) {
	s.resettables = append(s.resettables, r)
}

// AddJob registers a job.
func (s *Scheduler) AddJob(j Job) {
	s.jobs = append(s.jobs, j)
}

// Start runs the main loop. It blocks until context is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	interval := time.Duration(s.cfg.Ticker.TelemetryLoop)
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("Scheduler started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	// 0. Get and broadcast SimState
	simState := s.sim.GetState()
	if s.sink != nil {
		s.sink.UpdateState(simState)
	}

	// Skip telemetry processing if not active
	if simState != sim.StateActive {
		return
	}

	// 1. Fetch Telemetry
	tel, err := s.sim.GetTelemetry(ctx)
	if err != nil {
		slog.Debug("failed to read telemetry", "error", err)
		return
	}

	// 2. Broadcast to Sink (API)
	if s.sink != nil {
		s.sink.Update(&tel)
	}

	// 2.5 Teleport Detection
	// Check if we moved exceptionally far in a single tick (teleport/map change)
	currPos := geo.Point{Lat: tel.Latitude, Lon: tel.Longitude}
	if s.lastTickPos != (geo.Point{}) {
		distM := geo.Distance(s.lastTickPos, currPos)
		thresholdM := float64(s.cfg.Sim.TeleportThreshold)
		if thresholdM <= 0 {
			thresholdM = 80000.0 // Default 80km
		}

		if distM > thresholdM {
			slog.Info("Scheduler: Teleport detected", "dist_m", distM, "threshold_m", thresholdM)
			// Trigger Reset
			for _, r := range s.resettables {
				r.ResetSession(ctx)
			}
		}
	}
	s.lastTickPos = currPos

	// 3. Evaluate Jobs
	for _, job := range s.jobs {
		if job.ShouldFire(&tel) {
			// slog.Debug("Job firing", "job", job.Name())
			// Fire and forget
			go job.Run(ctx, &tel)
		}
	}
}

// --- Jobs ---

// Job defines a scheduled task.
type Job interface {
	Name() string
	ShouldFire(t *sim.Telemetry) bool
	Run(ctx context.Context, t *sim.Telemetry)
}

// BaseJob provides atomic running state to prevent re-entry.
type BaseJob struct {
	name    string
	running int32 // 1 if running, 0 otherwise
}

func NewBaseJob(name string) BaseJob {
	return BaseJob{name: name}
}

func (b *BaseJob) Name() string {
	return b.name
}

// TryLock attempts to set running to 1. Returns true if successful.
func (b *BaseJob) TryLock() bool {
	return atomic.CompareAndSwapInt32(&b.running, 0, 1)
}

func (b *BaseJob) Unlock() {
	atomic.StoreInt32(&b.running, 0)
}

// DistanceJob fires when distance traveled exceeds threshold.
type DistanceJob struct {
	BaseJob
	lastPos   geo.Point
	threshold float64 // meters
	action    func(context.Context, sim.Telemetry)
	firstRun  bool
}

func NewDistanceJob(name string, thresholdMeters float64, action func(context.Context, sim.Telemetry)) *DistanceJob {
	return &DistanceJob{
		BaseJob:   NewBaseJob(name),
		threshold: thresholdMeters,
		action:    action,
		firstRun:  true,
	}
}

func (j *DistanceJob) ShouldFire(t *sim.Telemetry) bool {
	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}

	currPos := geo.Point{Lat: t.Latitude, Lon: t.Longitude}

	if j.firstRun {
		return true
	}

	dist := geo.Distance(j.lastPos, currPos)
	return dist >= j.threshold
}

func (j *DistanceJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	// Update state BEFORE or AFTER?
	// Usually after success? Or immediately to reset accumulator?
	// Plan says "Has 5km passed? -> Trigger".
	// We reset lastPos to current pos.
	j.lastPos = geo.Point{Lat: t.Latitude, Lon: t.Longitude}
	j.firstRun = false

	j.action(ctx, *t)
}

// TimeJob fires when time elapsed exceeds threshold.
type TimeJob struct {
	BaseJob
	lastTime  time.Time
	threshold time.Duration
	action    func(context.Context, sim.Telemetry)
	firstRun  bool
}

func NewTimeJob(name string, threshold time.Duration, action func(context.Context, sim.Telemetry)) *TimeJob {
	return &TimeJob{
		BaseJob:   NewBaseJob(name),
		threshold: threshold,
		action:    action,
		firstRun:  true,
	}
}

func (j *TimeJob) ShouldFire(t *sim.Telemetry) bool {
	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}

	if j.firstRun {
		return true
	}

	return time.Since(j.lastTime) >= j.threshold
}

func (j *TimeJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	j.lastTime = time.Now()
	j.firstRun = false

	j.action(ctx, *t)
}
