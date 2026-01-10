package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestVersionSync ensures the backend (Go) and frontend (package.json) versions match.
func TestVersionSync(t *testing.T) {
	// Get the path to package.json relative to this test file
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Could not determine test file location")
	}

	// Navigate from pkg/version to internal/ui/web/package.json
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	packageJSONPath := filepath.Join(projectRoot, "internal", "ui", "web", "package.json")

	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		t.Fatalf("Failed to read package.json: %v", err)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatalf("Failed to parse package.json: %v", err)
	}

	// Backend version has "v" prefix, frontend doesn't
	backendVersion := strings.TrimPrefix(Version, "v")
	frontendVersion := pkg.Version

	if backendVersion != frontendVersion {
		t.Errorf("Version mismatch!\n  Backend:  %s (pkg/version/version.go)\n  Frontend: %s (internal/ui/web/package.json)", Version, frontendVersion)
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if !strings.HasPrefix(Version, "v") {
		t.Errorf("Version should start with 'v', got: %s", Version)
	}
}
