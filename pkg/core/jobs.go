package core

import (
	"context"
	"sync/atomic"
	"time"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
)

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
