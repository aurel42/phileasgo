package geo

import "sync"

// TrackBuffer maintains a rolling window of coordinates and calculates the average ground track.
type TrackBuffer struct {
	mu         sync.RWMutex
	samples    []Point
	windowSize int
}

// NewTrackBuffer creates a new buffer with the specified sample window size.
func NewTrackBuffer(windowSize int) *TrackBuffer {
	if windowSize < 2 {
		windowSize = 2
	}
	return &TrackBuffer{
		windowSize: windowSize,
	}
}

// Push adds a new point to the buffer and returns the current calculated track (bearing).
// If the buffer has fewer than 2 points, it returns the provided default heading.
func (b *TrackBuffer) Push(p Point, defaultHeading float64) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.samples = append(b.samples, p)
	if len(b.samples) > b.windowSize {
		b.samples = b.samples[1:]
	}

	if len(b.samples) < 2 {
		return defaultHeading
	}

	// Calculate bearing from oldest to newest point in window
	return Bearing(b.samples[0], b.samples[len(b.samples)-1])
}

// Reset clears the buffer history.
func (b *TrackBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.samples = nil
}
