package sim

import (
	"sync"
	"time"
)

// VerticalSpeedBuffer maintains a rolling window of altitude samples to calculate smoothed VSI.
type VerticalSpeedBuffer struct {
	mu         sync.RWMutex
	samples    []altSample
	windowSize time.Duration
}

type altSample struct {
	time time.Time
	alt  float64
}

// NewVerticalSpeedBuffer creates a buffer with the specified time window (e.g. 5s).
func NewVerticalSpeedBuffer(window time.Duration) *VerticalSpeedBuffer {
	return &VerticalSpeedBuffer{
		windowSize: window,
	}
}

// Update adds a new altitude sample and returns the calculated vertical speed in ft/min.
func (b *VerticalSpeedBuffer) Update(now time.Time, alt float64) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.samples = append(b.samples, altSample{time: now, alt: alt})

	// Remove old samples outside window
	cutoff := now.Add(-b.windowSize)
	for len(b.samples) > 2 && b.samples[1].time.Before(cutoff) {
		b.samples = b.samples[1:]
	}

	if len(b.samples) < 2 {
		return 0
	}

	// Calculate rate of change over the window
	first := b.samples[0]
	last := b.samples[len(b.samples)-1]

	dt := last.time.Sub(first.time).Seconds()
	if dt <= 0 {
		return 0
	}

	da := last.alt - first.alt

	// ft/s to ft/min
	return (da / dt) * 60.0
}

// Reset clears the buffer.
func (b *VerticalSpeedBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.samples = nil
}
