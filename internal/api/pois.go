package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/store"
	"phileasgo/pkg/wikipedia"
)

// POIHandler exposes POI data to the frontend.
type POIHandler struct {
	mgr   *poi.Manager
	wp    *wikipedia.Client
	store store.Store
}

// NewPOIHandler creates a new POI handler.
func NewPOIHandler(mgr *poi.Manager, wp *wikipedia.Client, st store.Store) *POIHandler {
	return &POIHandler{mgr: mgr, wp: wp, store: st}
}

// HandleTracked handles GET /api/pois/tracked.
func (h *POIHandler) HandleTracked(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	pois := h.mgr.GetTrackedPOIs()

	if err := json.NewEncoder(w).Encode(pois); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleThumbnail handles GET /api/pois/{id}/thumbnail.
// Fetches thumbnail from Wikipedia if not cached, persists it, and returns it.
func (h *POIHandler) HandleThumbnail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract POI ID from path: /api/pois/{id}/thumbnail
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/pois/"), "/")
	if len(parts) < 2 || parts[1] != "thumbnail" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	poiID := parts[0]

	// Get POI from manager
	p, err := h.mgr.GetPOI(r.Context(), poiID)
	if err != nil || p == nil {
		http.Error(w, "POI not found", http.StatusNotFound)
		return
	}

	// If thumbnail already cached, return it
	if p.ThumbnailURL != "" {
		h.respondThumbnail(w, p.ThumbnailURL)
		return
	}

	// Fetch new thumbnail
	thumbURL, err := h.fetchAndCacheThumbnail(r.Context(), p)
	if err != nil {
		// Log error but return empty thumbnail to frontend so it stops retrying or shows placeholder
		slog.Warn("Failed to fetch thumbnail", "poi_id", poiID, "error", err)
		h.respondThumbnail(w, "")
		return
	}

	h.respondThumbnail(w, thumbURL)
}

func (h *POIHandler) respondThumbnail(w http.ResponseWriter, url string) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"thumbnail_url": url}); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

func (h *POIHandler) fetchAndCacheThumbnail(ctx context.Context, p *model.POI) (string, error) {
	// Extract title from WPURL
	if p.WPURL == "" {
		return "", nil
	}

	// Parse title and lang from WPURL (e.g., https://en.wikipedia.org/wiki/Title)
	parsed, err := url.Parse(p.WPURL)
	if err != nil {
		return "", err
	}

	lang := strings.Split(parsed.Host, ".")[0]
	title := strings.TrimPrefix(parsed.Path, "/wiki/")
	title, _ = url.PathUnescape(title)

	// Fetch thumbnail from Wikipedia
	thumbURL, err := h.wp.GetThumbnail(ctx, title, lang)
	if err != nil {
		return "", err
	}
	if thumbURL == "" {
		return "", nil
	}

	// Persist to POI
	p.ThumbnailURL = thumbURL
	if h.store != nil {
		if err := h.store.SavePOI(ctx, p); err != nil {
			slog.Warn("Failed to persist thumbnail URL", "poi_id", p.WikidataID, "error", err)
		}
	}
	return thumbURL, nil
}

// HandleResetLastPlayed handles POST /api/pois/reset-last-played
func (h *POIHandler) HandleResetLastPlayed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 100km radius
	if err := h.mgr.ResetLastPlayed(r.Context(), req.Lat, req.Lon, 100000.0); err != nil {
		slog.Error("Failed to reset history", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	slog.Info("Reset last_played timestamp for POIs", "lat", req.Lat, "lon", req.Lon, "radius_m", 100000)

	w.WriteHeader(http.StatusOK)
}
