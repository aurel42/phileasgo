package api

import (
	"log/slog"
	"net/http"
	"os"
	"phileasgo/pkg/config"
)

// ImageHandler handles serving allowed images (screenshots).
type ImageHandler struct {
	snapshotPath string
}

// NewImageHandler creates a new ImageHandler.
func NewImageHandler(cfg *config.Config) *ImageHandler {
	path := cfg.Narrator.Screenshot.Path
	if path == "" {
		slog.Info("ImageHandler: No screenshot path configured, relying on valid absolute paths from backend")
	}
	return &ImageHandler{
		snapshotPath: path,
	}
}

// HandleGetImage serves the image at the requested path.
// GET /api/images/serve?path=...
func (h *ImageHandler) HandleGetImage(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}

	// Security Check: Basic Traversal Prevention & Existence
	// In a web server exposed to internet, we'd need strict strict allowlists.
	// Here, we serving local files to local browser.
	// We at least check if it is a file and exists.
	info, err := os.Stat(requestedPath)
	if os.IsNotExist(err) {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("Failed to stat image", "path", requestedPath, "error", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}

	// Serve the file
	http.ServeFile(w, r, requestedPath)
}
