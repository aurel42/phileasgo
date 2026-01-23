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

// Service monitors multiple directories for new files.
type Service struct {
	paths          []string
	lastChecked    time.Time
	mu             sync.Mutex
	lastNewestFile string
}

// NewService creates a new monitor for multiple directories.
// If paths is empty, it attempts to resolve the default "Pictures/Screenshots" folder.
func NewService(paths []string) (*Service, error) {
	if len(paths) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		// Default Windows/Generic path
		paths = []string{filepath.Join(home, "Pictures", "Screenshots")}
	}

	for _, path := range paths {
		// Verify it exists, or try to create it? better just warn if missing.
		if _, err := os.Stat(path); os.IsNotExist(err) {
			slog.Warn("Watcher: Directory does not verify", "path", path)
		}
	}

	return &Service{
		paths:       paths,
		lastChecked: time.Now(),
	}, nil
}

// CheckNew returns the path to the newest file created since the last successful check across all monitored paths.
// It returns (path, true) if a new file is found, (empty, false) otherwise.
func (s *Service) CheckNew() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newestFile string
	var newestTime time.Time
	var newestDir string

	for _, path := range s.paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}

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
			if modTime.After(s.lastChecked) {
				if modTime.After(newestTime) {
					newestTime = modTime
					newestFile = name
					newestDir = path
				}
			}
		}
	}

	if newestFile != "" && newestFile != s.lastNewestFile {
		s.lastChecked = newestTime
		s.lastNewestFile = newestFile
		fullPath := filepath.Join(newestDir, newestFile)
		slog.Info("Watcher: New screenshot detected", "file", newestFile, "dir", newestDir)
		return fullPath, true
	}

	return "", false
}
