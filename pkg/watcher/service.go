package watcher

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Service monitors a directory for new files.
type Service struct {
	path           string
	lastChecked    time.Time
	knownFiles     map[string]time.Time
	mu             sync.Mutex
	lastNewestFile string
}

// NewService creates a new directory monitor.
// If path is empty, it attempts to resolve the default "Pictures/Screenshots" folder.
func NewService(path string) (*Service, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		// Default Windows/Generic path
		path = filepath.Join(home, "Pictures", "Screenshots")
	}

	// Verify it exists, or try to create it? better just warn if missing.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		slog.Warn("Watcher: Directory does not verify", "path", path)
	}

	return &Service{
		path:        path,
		lastChecked: time.Now(),
		knownFiles:  make(map[string]time.Time),
	}, nil
}

// CheckNew returns the path to the newest file created since the last successful check.
// It returns (path, true) if a new file is found, (empty, false) otherwise.
func (s *Service) CheckNew() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.path)
	if err != nil {
		// Log rarely to avoid spam?
		return "", false
	}

	var newestFile string
	var newestTime time.Time

	// Filter for images
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		modTime := info.ModTime()

		// Only care about files created/modified AFTER our service started/last check
		// Actually, to avoid re-triggering, we just track the newest one we've seen.
		// If we find one that is NEWER than s.lastChecked, it's a candidate.
		if modTime.After(s.lastChecked) {
			if modTime.After(newestTime) {
				newestTime = modTime
				newestFile = name
			}
		}
	}

	if newestFile != "" && newestFile != s.lastNewestFile {
		s.lastChecked = newestTime
		s.lastNewestFile = newestFile
		fullPath := filepath.Join(s.path, newestFile)
		slog.Info("Watcher: New screenshot detected", "file", newestFile)
		return fullPath, true
	}

	return "", false
}
