package audio

import (
	"math"
	"time"

	"github.com/gopxl/beep/v2"
)

// BiquadFilter implements a basic Biquad digital filter.
type BiquadFilter struct {
	streamer   beep.Streamer
	sampleRate float64

	// Coefficients
	a0, a1, a2 float64
	b0, b1, b2 float64

	// State
	x1, x2 [2]float64
	y1, y2 [2]float64
}

// NewLowPass create a new LowPass Biquad filter.
func NewLowPass(streamer beep.Streamer, sampleRate, cutoff, q float64) *BiquadFilter {
	f := &BiquadFilter{streamer: streamer, sampleRate: sampleRate}
	f.updateLowPass(cutoff, q)
	return f
}

// NewHighPass create a new HighPass Biquad filter.
func NewHighPass(streamer beep.Streamer, sampleRate, cutoff, q float64) *BiquadFilter {
	f := &BiquadFilter{streamer: streamer, sampleRate: sampleRate}
	f.updateHighPass(cutoff, q)
	return f
}

func (f *BiquadFilter) updateLowPass(cutoff, q float64) {
	omega := 2.0 * math.Pi * cutoff / f.sampleRate
	sn := math.Sin(omega)
	cs := math.Cos(omega)
	alpha := sn / (2.0 * q)

	f.b0 = (1.0 - cs) / 2.0
	f.b1 = 1.0 - cs
	f.b2 = (1.0 - cs) / 2.0
	f.a0 = 1.0 + alpha
	f.a1 = -2.0 * cs
	f.a2 = 1.0 - alpha
}

func (f *BiquadFilter) updateHighPass(cutoff, q float64) {
	omega := 2.0 * math.Pi * cutoff / f.sampleRate
	sn := math.Sin(omega)
	cs := math.Cos(omega)
	alpha := sn / (2.0 * q)

	f.b0 = (1.0 + cs) / 2.0
	f.b1 = -(1.0 + cs)
	f.b2 = (1.0 + cs) / 2.0
	f.a0 = 1.0 + alpha
	f.a1 = -2.0 * cs
	f.a2 = 1.0 - alpha
}

func (f *BiquadFilter) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = f.streamer.Stream(samples)
	for i := 0; i < n; i++ {
		for channel := 0; channel < 2; channel++ {
			x := samples[i][channel]
			y := (f.b0/f.a0)*x + (f.b1/f.a0)*f.x1[channel] + (f.b2/f.a0)*f.x2[channel] -
				(f.a1/f.a0)*f.y1[channel] - (f.a2/f.a0)*f.y2[channel]

			f.x2[channel] = f.x1[channel]
			f.x1[channel] = x
			f.y2[channel] = f.y1[channel]
			f.y1[channel] = y

			samples[i][channel] = y
		}
	}
	return n, ok
}

func (f *BiquadFilter) Err() error {
	return f.streamer.Err()
}

// SmoothVolume implements a Streamer that allows for smooth volume changes and fading.
//
// Thread Safety:
// SmoothVolume is NOT internally synchronized. It relies on the caller to provide
// synchronization. When used with the gopxl/beep/speaker package, all methods
// (Stream, SetTargetVolume, FadeTo) must be called while holding speaker.Lock().
// This is because the speaker calls Stream() from its own background goroutine
// while holding the same lock.
type SmoothVolume struct {
	Streamer beep.Streamer

	// targetVolume is the baseline volume (e.g., from the UI slider) 0.0 to 1.0.
	targetVolume float64
	// fadeLevel is the multiplier for fade-in/out effects (0.0 to 1.0).
	fadeLevel float64

	// currentGain is the actual multiplier being applied to the samples.
	currentGain float64

	// step is the amount gain changes per sample to reach (targetVolume * fadeLevel)
	// over a specific time/sample window.
	step float64
}

// NewSmoothVolume creates a new SmoothVolume streamer.
func NewSmoothVolume(s beep.Streamer, initialVol float64) *SmoothVolume {
	return &SmoothVolume{
		Streamer:     s,
		targetVolume: initialVol,
		fadeLevel:    1.0,
		currentGain:  initialVol,
	}
}

// Stream applies the current gain and transitions towards the target gain.
// Note: This is called by the speaker goroutine while holding speaker.Lock().
func (s *SmoothVolume) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = s.Streamer.Stream(samples)

	targetGain := s.targetVolume * s.fadeLevel

	for i := 0; i < n; i++ {
		// Update currentGain towards targetGain
		if s.currentGain != targetGain {
			if s.step == 0 {
				// Instantly jump if no smoothing step is defined (should not happen during fade)
				s.currentGain = targetGain
			} else {
				if s.currentGain < targetGain {
					s.currentGain += s.step
					if s.currentGain > targetGain {
						s.currentGain = targetGain
					}
				} else {
					s.currentGain -= s.step
					if s.currentGain < targetGain {
						s.currentGain = targetGain
					}
				}
			}
		}

		// Apply gain
		samples[i][0] *= s.currentGain
		samples[i][1] *= s.currentGain
	}

	return n, ok
}

func (s *SmoothVolume) Err() error {
	return s.Streamer.Err()
}

// SetTargetVolume updates the baseline volume level.
// Note: Must be called while holding speaker.Lock().
func (s *SmoothVolume) SetTargetVolume(vol, sampleRate float64, duration time.Duration) {
	if vol < 0 {
		vol = 0
	}
	s.targetVolume = vol
	s.updateStep(sampleRate, duration)
}

// FadeTo updates the fade level.
// Note: Must be called while holding speaker.Lock().
func (s *SmoothVolume) FadeTo(level, sampleRate float64, duration time.Duration) {
	if level < 0 {
		level = 0
	} else if level > 1 {
		level = 1
	}
	s.fadeLevel = level
	s.updateStep(sampleRate, duration)
}

func (s *SmoothVolume) updateStep(sampleRate float64, duration time.Duration) {
	if duration <= 0 {
		s.step = 1.0 // Huge step to effectively jump in next Stream call
		return
	}
	numSamples := float64(sampleRate) * duration.Seconds()
	targetGain := s.targetVolume * s.fadeLevel
	diff := math.Abs(targetGain - s.currentGain)
	if diff == 0 {
		s.step = 0
		return
	}
	s.step = diff / numSamples
}

// NewHeadsetFilter creates a bandpass filter optimized for aviation headsets.
func NewHeadsetFilter(streamer beep.Streamer, sampleRate, lowCutoff, highCutoff float64) beep.Streamer {
	// Chain HighPass (cut lows) and LowPass (cut highs)
	// Q=0.707 is a standard Butterworth response (flat passband)
	hp := NewHighPass(streamer, sampleRate, lowCutoff, 0.707)
	lp := NewLowPass(hp, sampleRate, highCutoff, 0.707)
	return lp
}
