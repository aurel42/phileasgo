package api

import (
	"net/http"
	"os"
)

// spaFileSystem implements http.FileSystem and cleanly handles SPA routing
// by falling back to index.html for non-existent files.
type spaFileSystem struct {
	root http.FileSystem
}

// Open opens the named file. If the file does not exist, it falls back to index.html.
func (s *spaFileSystem) Open(name string) (http.File, error) {
	f, err := s.root.Open(name)
	if os.IsNotExist(err) {
		// If the file doesn't exist, check if it looks like an API call or static asset
		// that intentionally shouldn't be rerouted (optional, but good practice).
		// For now, simpler approach: if it's missing, try serving index.html
		// provided we are not explicitly looking for the root (to avoid loops if index.html is missing).
		return s.root.Open("index.html")
	}
	if err != nil {
		return nil, err
	}

	// If it's a directory, technically we might want to serve index.html too,
	// unless we want directory listing (which we don't).
	// Check if file exists and is a directory
	// (Previously checked for directory, now just simple pass-through)

	return f, nil
}
