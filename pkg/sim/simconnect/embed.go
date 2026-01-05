package simconnect

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed lib/SimConnect.dll
var embeddedDLL embed.FS

// extractEmbeddedDLL extracts the embedded SimConnect.dll to a temp directory.
// Returns the path to the extracted DLL.
func extractEmbeddedDLL() (string, error) {
	// Read embedded DLL
	data, err := embeddedDLL.ReadFile("lib/SimConnect.dll")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded SimConnect.dll: %w", err)
	}

	// Create temp directory for DLL
	tempDir := filepath.Join(os.TempDir(), "phileasgo")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Write DLL to temp location
	dllPath := filepath.Join(tempDir, "SimConnect.dll")
	if err := os.WriteFile(dllPath, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write SimConnect.dll: %w", err)
	}

	return dllPath, nil
}
