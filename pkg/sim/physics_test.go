package sim

import (
	"testing"
	"time"
)

func TestVerticalSpeedBuffer(t *testing.T) {
	buf := NewVerticalSpeedBuffer(5 * time.Second)
	now := time.Now()

	// 1. Initial state
	if vs := buf.Update(now, 1000); vs != 0 {
		t.Errorf("Expected 0 VS for first sample, got %.2f", vs)
	}

	// 2. Constant altitude
	now = now.Add(1 * time.Second)
	if vs := buf.Update(now, 1000); vs != 0 {
		t.Errorf("Expected 0 VS for constant altitude, got %.2f", vs)
	}

	// 3. Simple climb (1000ft in 60s -> 1000 fpm)
	// Let's do 100ft in 6s
	buf.Reset()
	start := time.Now()
	buf.Update(start, 1000)

	target := start.Add(6 * time.Second)
	vs := buf.Update(target, 1100)

	// da=100, dt=6s -> 16.66 ft/s -> 1000 ft/min
	if vs < 999 || vs > 1001 {
		t.Errorf("Expected ~1000 fpm, got %.2f", vs)
	}

	// 4. Jitter rejection (small fluctuation)
	// Window is 5s. samples: [t=0, alt=1000], [t=6, alt=1100]
	// Add jitter sample at t=7
	vs = buf.Update(target.Add(1*time.Second), 1090)
	// Window: cutoff = 7-5 = 2.
	// samples: [0, 1000], [6, 1100], [7, 1090]. dt=7, da=90 -> 12.85 ft/s -> 771.43 fpm
	if vs < 770 || vs > 772 {
		t.Errorf("Expected ~771 fpm for jitter tick, got %.2f", vs)
	}
}
