package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	// Set CWD to project root for paths to work (configs/..., data/...)
	originalWD, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Logf("Failed to restore WD: %v", err)
		}
	}()

	// Go up to root
	if err := os.Chdir("../../"); err != nil {
		t.Fatalf("Failed to chdir to root: %v", err)
	}

	os.Setenv("TEST_MODE", "true")
	defer os.Unsetenv("TEST_MODE")

	// Create a temp config file with a different port
	tempConfig := `
server:
    address: localhost:0  # 0 lets OS choose free port
ticker:
    telemetry_loop: 100ms
triggers:
    distance: 5km
    time: 30s
log:
    server:
        path: "logs/test_server.log"
        level: "debug"
    requests:
        path: "logs/test_requests.log"
        level: "info"
    gemini:
        path: "logs/test_gemini.log"
        level: "debug"
db:
    path: ":memory:" # Use in-memory DB for test
tts:
    engine: "edge-tts"
`
	f, err := os.CreateTemp("", "phileas_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}
	defer os.Remove(f.Name()) // Clean up

	if _, err := f.WriteString(tempConfig); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}
	f.Close()

	// Create a context that cancels quickly to verify startup sequence
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run with temp config
	if err := run(ctx, f.Name()); err != nil {
		t.Fatalf("run() failed: %v", err)
	}
}
