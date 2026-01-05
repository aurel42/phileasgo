package audio

import "math"

func volumeToPower(vol float64) float64 {
	// vol is 0.0 to 1.0
	// We want to map this to beep's power.
	// Base 2:
	// 0 -> Silent (handled by Silent flag)
	// 1 -> 0 (Unity gain)
	// 0.5 -> -1 (Half power?) No, -1 is half amplitude?
	// Let's use a simple log-like mapping or linear for now if simple.
	// Beep docs say: Volume adds to the exponent.
	// We can use: math.Log2(vol)
	if vol <= 0.01 {
		return -10 // Silent
	}
	return math.Log2(vol)
}
