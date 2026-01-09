package mocksim

import (
	"context"
	"testing"
	"time"
)

func TestAltitudeOffset(t *testing.T) {
	// Start at 5200ft. Bottom should be 5000.
	// First climb target was 1500 -> becomes 6500.
	cfg := Config{
		DurationParked: 0,
		DurationTaxi:   0,
		DurationHold:   0,
		StartAlt:       5200.0,
	}
	client := NewClient(cfg)
	defer client.Close()

	// Wait for climb to start
	waitForReq(t, func() bool {
		tel, _ := client.GetTelemetry(context.Background())
		// If offset works, target is 6500.
		// If offset failed, target is 1500. Since we start at 5200,
		// if target is 1500, we would DESCEND?
		// Logic:
		// if step.Rate > 0 (500)
		// if tel.Altitude + delta >= step.Target -> reached
		// If target is 1500 and we are at 5200, 5200 >= 1500 is TRUE.
		// So we would instantly snap to 1500? Or mark reached and go to next step (WAIT)?
		// Then next step is WAIT.
		// So VerticalSpeed would become 0 very quickly if offset logic is broken.
		// If offset logic works, target is 6500, so 5200 < 6500, so we keep climbing.
		return tel.VerticalSpeed == 500.0 && tel.AltitudeMSL > 5200.0
	}, 2*time.Second, "Climbing from high altitude")

	// Verify we climb towards 6500
	ctx := context.Background()
	tel, _ := client.GetTelemetry(ctx)
	if tel.VerticalSpeed != 500.0 {
		t.Errorf("Expected positive climb rate, got %f", tel.VerticalSpeed)
	}
}
