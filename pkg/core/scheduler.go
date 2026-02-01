package core

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
)

// TelemetrySink is an interface for consumers of the high-frequency telemetry stream.
type TelemetrySink interface {
	Update(t *sim.Telemetry)
	UpdateState(s sim.State)
}

// Scheduler manages the central heartbeat and scheduled jobs.
type Scheduler struct {
	cfg              *config.Config
	sim              sim.Client
	sink             TelemetrySink
	jobs             []Job
	resettables      []SessionResettable
	lastTickPos      geo.Point
	locationProvider LocationProvider
}

// NewScheduler creates a new Scheduler.
func NewScheduler(cfg *config.Config, simClient sim.Client, sink TelemetrySink, g LocationProvider) *Scheduler {
	s := &Scheduler{
		cfg:              cfg,
		sim:              simClient,
		sink:             sink,
		jobs:             []Job{},
		resettables:      []SessionResettable{},
		locationProvider: g,
	}

	// Register Core Jobs
	// Register Core Jobs

	return s
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
		// 1Hz for stability; high frequency is unnecessary as SimConnect data updates at 1Hz
		interval = 1 * time.Second
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
		if err == sim.ErrWaitingForTelemetry {
			// Expected state when connected but waiting for data
			return
		}
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
	if s.lastTickPos == (geo.Point{}) {
		// First valid tick - Initialize optimizations
		if s.locationProvider != nil {
			s.locationProvider.ReorderFeatures(currPos.Lat, currPos.Lon)
		}
	} else {
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
			// Optimize lookup for new area
			if s.locationProvider != nil {
				s.locationProvider.ReorderFeatures(currPos.Lat, currPos.Lon)
			}
		}
	}
	s.lastTickPos = currPos

	// 3. Evaluate Jobs
	for _, job := range s.jobs {
		if job.ShouldFire(&tel) {
			// Fire and forget
			go job.Run(ctx, &tel)
		}
	}
}
