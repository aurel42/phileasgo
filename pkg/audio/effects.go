package audio

import (
	"math"

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

// NewHeadsetFilter creates a bandpass filter optimized for aviation headsets.
func NewHeadsetFilter(streamer beep.Streamer, sampleRate, lowCutoff, highCutoff float64) beep.Streamer {
	// Chain HighPass (cut lows) and LowPass (cut highs)
	// Q=0.707 is a standard Butterworth response (flat passband)
	hp := NewHighPass(streamer, sampleRate, lowCutoff, 0.707)
	lp := NewLowPass(hp, sampleRate, highCutoff, 0.707)
	return lp
}
