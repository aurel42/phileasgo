package simconnect

import (
	"context"
	"testing"
	"time"
)

// TestClient_GetTelemetry tests telemetry retrieval from SimConnect.
// If the simulator is not running, the test is skipped gracefully.
func TestClient_GetTelemetry(t *testing.T) {
	// Try to load the DLL
	err := LoadDLL("SimConnect.dll")
	if err != nil {
		t.Skipf("SimConnect.dll not available, skipping: %v", err)
		return
	}

	// Try to connect
	handle, err := Open("PhileasGo-Test")
	if err != nil {
		t.Skipf("Simulator not running, skipping: %v", err)
		return
	}
	defer func() { _ = Close(handle) }()

	// Create client (but don't start background loops)
	client := &Client{
		handle:       handle,
		connected:    true,
		stopChan:     make(chan struct{}),
		reconnectInt: 5 * time.Second,
	}

	// Setup data definitions
	err = client.setupDataDefinitions()
	if err != nil {
		t.Fatalf("Failed to setup data definitions: %v", err)
	}

	// Wait for some data (give sim time to respond)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var gotData bool
	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		ppData, _, err := GetNextDispatch(handle)
		if err != nil {
			t.Fatalf("GetNextDispatch error: %v", err)
		}

		if ppData != nil {
			client.handleMessage(ppData)

			// Check if we got telemetry
			tel, _ := client.GetTelemetry(ctx)
			if tel.Latitude != 0 || tel.Longitude != 0 {
				gotData = true
				t.Logf("Got telemetry: Lat=%.5f, Lon=%.5f, Alt=%.1f, Hdg=%.1f",
					tel.Latitude, tel.Longitude, tel.AltitudeMSL, tel.Heading)
				break
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	if !gotData {
		t.Log("No telemetry received within timeout (sim may be paused or on menu)")
	}
}

// TestLoadDLL tests that the DLL can be loaded if present.
func TestLoadDLL(t *testing.T) {
	err := LoadDLL("SimConnect.dll")
	if err != nil {
		t.Skipf("SimConnect.dll not available: %v", err)
		return
	}
	t.Log("SimConnect.dll loaded successfully")
}
