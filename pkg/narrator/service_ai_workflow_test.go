package narrator

import (
	"testing"
)

func TestAIService_SkipCooldown(t *testing.T) {
	svc := &AIService{}

	// Test Cooldown Skip
	if svc.ShouldSkipCooldown() {
		t.Error("ShouldSkipCooldown should be false initially")
	}
	svc.SkipCooldown()
	if !svc.ShouldSkipCooldown() {
		t.Error("ShouldSkipCooldown should be true after SkipCooldown()")
	}
	svc.ResetSkipCooldown()
	if svc.ShouldSkipCooldown() {
		t.Error("ShouldSkipCooldown should be false after ResetSkipCooldown()")
	}
}
