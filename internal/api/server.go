package api

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"phileasgo/internal/ui"
	"phileasgo/pkg/version"
)

// NewServer creates and configures the HTTP server.
// It accepts handlers for all API endpoints and a shutdownFunc for graceful shutdown.
func NewServer(addr string, tel *TelemetryHandler, cfg *ConfigHandler, stats *StatsHandler, cache *CacheHandler, pois *POIHandler, vis *VisibilityHandler, audioH *AudioHandler, narratorH *NarratorHandler, imageH *ImageHandler, geo *GeographyHandler, tripH *TripHandler, labelH *MapLabelsHandler, simH *SimCommandHandler, regionalH *RegionalCategoriesHandler, featuresH *FeaturesHandler, shutdown func()) *http.Server {
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
		mux.HandleFunc("POST /api/narrator/play-city", narratorH.HandlePlayCity)
		mux.HandleFunc("POST /api/narrator/play-feature", narratorH.HandlePlayFeature)
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

	// 2l. Label Endpoint (New)
	if labelH != nil {
		mux.HandleFunc("POST /api/map/labels/sync", labelH.HandleSync)
	}

	// 2n. Simulator Command Endpoint
	if simH != nil {
		mux.HandleFunc("POST /api/sim/command", simH.HandleCommand)
	}

	// 2o. Regional Endpoint
	if regionalH != nil {
		mux.HandleFunc("GET /api/regional", regionalH.HandleGet)
	}

	// 2p. Features Endpoint
	if featuresH != nil {
		mux.HandleFunc("GET /api/features", featuresH.HandleGet)
	}

	// 2m. Profiling Endpoints (pprof)
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	mux.Handle("GET /debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("GET /debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("GET /debug/pprof/allocs", pprof.Handler("allocs"))

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

	// CORS Middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "coui://html_ui")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		mux.ServeHTTP(w, r)
	})

	return &http.Server{
		Addr:         addr,
		Handler:      handler,
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
