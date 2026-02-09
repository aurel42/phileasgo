package api

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"phileasgo/internal/ui"
	"phileasgo/pkg/version"
)

// NewServer creates and configures the HTTP server.
// It accepts handlers for all API endpoints and a shutdownFunc for graceful shutdown.
func NewServer(addr string, tel *TelemetryHandler, cfg *ConfigHandler, stats *StatsHandler, cache *CacheHandler, pois *POIHandler, vis *VisibilityHandler, audioH *AudioHandler, narratorH *NarratorHandler, imageH *ImageHandler, geo *GeographyHandler, tripH *TripHandler, shutdown func()) *http.Server {
	mux := http.NewServeMux()

	// 1. Health Endpoint
	mux.HandleFunc("GET /health", handleHealth)

	// 2. Telemetry Endpoint
	mux.HandleFunc("GET /api/telemetry", tel.handleTelemetry)

	// 2b. Version Endpoint
	mux.HandleFunc("GET /api/version", handleVersion)

	// 2c. Config Endpoints
	mux.HandleFunc("/api/config", cfg.HandleConfig)

	// 2d. Stats Endpoint
	mux.Handle("GET /api/stats", stats)

	// 2d. Logs Endpoint
	mux.HandleFunc("GET /api/log/latest", handleLatestLog)

	// 2e. Cache Endpoint
	mux.Handle("GET /api/wikidata/cache", cache)

	// 2f. POI Endpoints
	mux.HandleFunc("GET /api/pois/tracked", pois.HandleTracked)
	mux.HandleFunc("GET /api/pois/{id}/thumbnail", pois.HandleThumbnail)
	mux.HandleFunc("POST /api/pois/reset-last-played", pois.HandleResetLastPlayed)
	mux.HandleFunc("GET /api/map/settlements", pois.HandleSettlements)

	// 2g. Visibility Endpoint
	mux.HandleFunc("GET /api/map/visibility", vis.Handler)
	mux.HandleFunc("GET /api/map/visibility-mask", vis.HandleMask)
	mux.HandleFunc("GET /api/map/coverage", vis.HandleGetCoverage)

	// 2h. Geography Endpoint
	mux.HandleFunc("GET /api/geography", geo.Handle)

	// 2i. Audio Endpoints
	if audioH != nil {
		mux.HandleFunc("POST /api/audio/control", audioH.HandleControl)
		mux.HandleFunc("POST /api/audio/volume", audioH.HandleVolume)
		mux.HandleFunc("GET /api/audio/status", audioH.HandleStatus)
	}

	// 2i. Narrator Endpoints
	if narratorH != nil {
		mux.HandleFunc("POST /api/narrator/play", narratorH.HandlePlay)
		mux.HandleFunc("GET /api/narrator/status", narratorH.HandleStatus)
		mux.HandleFunc("POST /api/narrator/clear-image", narratorH.HandleClearImage)
	}

	// 2j. Image Endpoint
	if imageH != nil {
		mux.HandleFunc("GET /api/images/serve", imageH.HandleGetImage)
	}

	// 2k. Trip Endpoint
	if tripH != nil {
		mux.HandleFunc("GET /api/trip/events", tripH.HandleEvents)
	}

	// 3. Shutdown Endpoint

	// 3. Shutdown Endpoint
	mux.HandleFunc("POST /api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Graceful shutdown initiated via API")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Shutting down...")); err != nil {
			slog.Error("Failed to write shutdown response", "error", err)
		}
		// Call shutdown in a goroutine to allow response to flush
		go func() {
			time.Sleep(100 * time.Millisecond)
			shutdown()
		}()
	})

	// 4. Static Frontend Serving (SPA)
	// We need to serve from the "dist" subdirectory of the embedded FS
	distFS, err := fs.Sub(ui.DistFS, "dist")
	if err != nil {
		panic(fmt.Sprintf("Failed to subtree dist from embedded assets: %v", err))
	}

	spaFS := &spaFileSystem{root: http.FS(distFS)}
	mux.Handle("/", http.FileServer(spaFS))

	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(w, `{"version": "%s"}`, version.Version); err != nil {
		slog.Error("Failed to write version response", "error", err)
	}
}
