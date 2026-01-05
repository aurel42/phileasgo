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
)

// POIProvider matches the GetBestCandidate method used by jobs.
type POIProvider interface {
	GetBestCandidate() *model.POI
}

// NarrationJob triggers AI narration for the best available POI.
type NarrationJob struct {
	BaseJob
	cfg              *config.Config
	narrator         narrator.Service
	poiMgr           POIProvider
	lastTime         time.Time
	nextFireDuration time.Duration
	wasBusy          bool
}

func NewNarrationJob(cfg *config.Config, n narrator.Service, pm POIProvider) *NarrationJob {
	j := &NarrationJob{
		BaseJob:  NewBaseJob("Narration"),
		cfg:      cfg,
		narrator: n,
		poiMgr:   pm,
		lastTime: time.Now(),
	}
	j.calculateNextDuration(1.0)
	return j
}

func (j *NarrationJob) ShouldFire(t *sim.Telemetry) bool {
	if atomic.LoadInt32(&j.running) == 1 {
		return false
	}

	// 0. Global Switch
	if !j.cfg.Narrator.AutoNarrate {
		return false
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
		return false
	}

	// 2. Cooldown Check
	if !j.lastTime.IsZero() {
		// If skip requested, bypass timer
		if j.narrator.ShouldSkipCooldown() {
			j.narrator.ResetSkipCooldown()
			slog.Info("NarrationJob: Skipping cooldown check by user request")
		} else {
			elapsed := time.Since(j.lastTime)

			// Use the randomized duration
			// If nextFireDuration is 0 (first run or not set), default to Min
			required := j.nextFireDuration
			if required == 0 {
				required = time.Duration(j.cfg.Narrator.CooldownMin)
			}

			if elapsed < required {
				return false
			}
		}
	}

	// 3. Selection
	best := j.poiMgr.GetBestCandidate()
	if best == nil {
		// No POI? Check if we can do an essay.
		// ESSAYS: only above 2000ft AGL.
		if t.AltitudeAGL < 2000 {
			return false
		}

		// We return TRUE to let Run() verify and decide.
		return true
	}

	// 4. Threshold Check
	// If score is low, we normally fail. But now we might want to run an essay.
	// ESSAYS: only above 2000ft AGL.
	if best.Score < j.cfg.Narrator.MinScoreThreshold && t.AltitudeAGL < 2000 {
		return false
	}

	// So we return TRUE, and let Run() decide between POI and Essay.
	// We assume essays are enabled if config is present.
	return true
}

func (j *NarrationJob) Run(ctx context.Context, t *sim.Telemetry) {
	if !j.TryLock() {
		return
	}
	defer j.Unlock()

	best := j.poiMgr.GetBestCandidate()
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

	j.narrator.PlayPOI(ctx, best.WikidataID, false, t)
	j.calculateNextDuration(1.0)
}

func (j *NarrationJob) tryEssay(ctx context.Context, t *sim.Telemetry) {
	// Safety check: only above 2000ft AGL
	if t != nil && t.AltitudeAGL < 2000 {
		return
	}

	if j.narrator.PlayEssay(ctx, t) {
		j.calculateNextDuration(2.0)
	}
}

func (j *NarrationJob) calculateNextDuration(multiplier float64) {
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
	cfg  *config.Config
	sim  sim.Client
	sink TelemetrySink
	jobs []Job
}

// NewScheduler creates a new Scheduler.
func NewScheduler(cfg *config.Config, simClient sim.Client, sink TelemetrySink) *Scheduler {
	return &Scheduler{
		cfg:  cfg,
		sim:  simClient,
		sink: sink,
		jobs: []Job{},
	}
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

	// 3. Evaluate Jobs
	for _, job := range s.jobs {
		if job.ShouldFire(&tel) {
			slog.Debug("Job firing", "job", job.Name())
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
