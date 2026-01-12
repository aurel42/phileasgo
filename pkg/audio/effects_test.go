package audio

import (
	"testing"
)

type dummyStreamer struct {
	samples [][2]float64
	pos     int
}

func (s *dummyStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	if s.pos >= len(s.samples) {
		return 0, false
	}
	n = copy(samples, s.samples[s.pos:])
	s.pos += n
	return n, true
}

func (s *dummyStreamer) Err() error { return nil }

func TestBandpassFilter_Stream(t *testing.T) {
	// Create a simple impulse or constant signal
	input := make([][2]float64, 100)
	for i := range input {
		input[i] = [2]float64{1.0, 1.0}
	}

	ds := &dummyStreamer{samples: input}

	// Create filter: 400Hz - 3500Hz at 48kHz
	filter := NewHeadsetFilter(ds, 48000, 400, 3500)

	output := make([][2]float64, 100)
	n, ok := filter.Stream(output)

	if n != 100 {
		t.Errorf("Expected 100 samples, got %d", n)
	}
	if !ok {
		t.Error("Stream returned ok=false")
	}

	// For a constant 1.0 signal, a High Pass filter at 400Hz should eventually attenuate it to 0 (DC blocking)
	// Biquad filters take time to settle, but after 100 samples we should see significant change from 1.0.
	lastSample := output[99][0]
	if lastSample == 1.0 {
		t.Error("Filter did not modify constant signal (DC should be filtered)")
	}

	// Verify it's not NaN or Inf
	if lastSample != lastSample { // NaN check
		t.Error("Filter produced NaN")
	}
}

func TestBiquadFilter_Consistency(t *testing.T) {
	ds := &dummyStreamer{samples: [][2]float64{{1.0, 1.0}}}
	f := NewLowPass(ds, 44100, 1000, 0.707)

	samples := make([][2]float64, 1)
	f.Stream(samples)

	val := samples[0][0]
	if val == 1.0 {
		t.Error("LowPass filter did not modify signal")
	}
}
