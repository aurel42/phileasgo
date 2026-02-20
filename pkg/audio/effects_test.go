package audio

import (
	"testing"
	"time"
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

func TestSmoothVolume_Stream(t *testing.T) {
	// 1. Test basic gain application
	ds := &dummyStreamer{samples: [][2]float64{{1.0, 1.0}, {1.0, 1.0}}}
	sv := NewSmoothVolume(ds, 0.5)

	samples := make([][2]float64, 2)
	n, ok := sv.Stream(samples)

	if n != 2 || !ok {
		t.Fatalf("Stream failed: n=%d, ok=%v", n, ok)
	}
	if samples[0][0] != 0.5 || samples[1][0] != 0.5 {
		t.Errorf("Expected samples to be 0.5, got %v, %v", samples[0][0], samples[1][0])
	}

	// 2. Test volume ramping (smoothing)
	ds2 := &dummyStreamer{samples: [][2]float64{{1.0, 1.0}, {1.0, 1.0}, {1.0, 1.0}, {1.0, 1.0}}}
	sv2 := NewSmoothVolume(ds2, 0.0)

	// Set target to 1.0 over 4 samples (at 1Hz for simplicity)
	sv2.SetTargetVolume(1.0, 1.0, 4*time.Second) // step = 1.0 / 4 = 0.25

	samples2 := make([][2]float64, 4)
	sv2.Stream(samples2)

	// Samples should be [0.25, 0.5, 0.75, 1.0]
	expected := []float64{0.25, 0.5, 0.75, 1.0}
	for i, exp := range expected {
		if samples2[i][0] != exp {
			t.Errorf("Sample %d: expected %f, got %f", i, exp, samples2[i][0])
		}
	}

	// 3. Test FadeTo
	ds3 := &dummyStreamer{samples: [][2]float64{{1.0, 1.0}, {1.0, 1.0}}}
	sv3 := NewSmoothVolume(ds3, 1.0)
	sv3.FadeTo(0.0, 1.0, 2*time.Second) // step = 1.0 / 2 = 0.5

	samples3 := make([][2]float64, 2)
	sv3.Stream(samples3)

	if samples3[0][0] != 0.5 {
		t.Errorf("Fade sample 0: expected 0.5, got %f", samples3[0][0])
	}
	if samples3[1][0] != 0.0 {
		t.Errorf("Fade sample 1: expected 0.0, got %f", samples3[1][0])
	}
}
