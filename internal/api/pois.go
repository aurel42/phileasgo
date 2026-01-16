package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/store"
	"phileasgo/pkg/wikipedia"
)

// POIHandler exposes POI data to the frontend.
type POIHandler struct {
	mgr       *poi.Manager
	wp        *wikipedia.Client
	store     store.Store
	llm       llm.Provider
	promptMgr *prompts.Manager
}

// NewPOIHandler creates a new POI handler.
func NewPOIHandler(mgr *poi.Manager, wp *wikipedia.Client, st store.Store, llmProv llm.Provider, promptMgr *prompts.Manager) *POIHandler {
	return &POIHandler{mgr: mgr, wp: wp, store: st, llm: llmProv, promptMgr: promptMgr}
}

// ... existing handler methods ...

// HandleTracked handles GET /api/pois/tracked.
func (h *POIHandler) HandleTracked(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// 1. Fetch filter settings from store
	filterMode, _ := h.store.GetState(ctx, "filter_mode")
	if filterMode == "" {
		filterMode = "fixed"
	}

	targetCountStr, _ := h.store.GetState(ctx, "target_poi_count")
	targetCount := 20
	if targetCountStr != "" {
		if val, err := strconv.Atoi(targetCountStr); err == nil {
			targetCount = val
		}
	}

	minScoreStr, _ := h.store.GetState(ctx, "min_poi_score")
	minScore := 0.5
	if minScoreStr != "" {
		if val, err := strconv.ParseFloat(minScoreStr, 64); err == nil {
			minScore = val
		}
	}

	// 2. Get filtered POIs (API always uses airborne mode to show all POIs)
	pois, threshold := h.mgr.GetPOIsForUI(filterMode, targetCount, minScore)

	// 3. Optional: Custom response header for threshold
	w.Header().Set("X-Phileas-Effective-Threshold", fmt.Sprintf("%.2f", threshold))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pois); err != nil {
		slog.Error("Failed to encode tracked POIs", "error", err)
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

	// Parse title and lang from WPURL
	parsed, err := url.Parse(p.WPURL)
	if err != nil {
		return "", err
	}

	lang := strings.Split(parsed.Host, ".")[0]
	title := strings.TrimPrefix(parsed.Path, "/wiki/")
	title, _ = url.PathUnescape(title)

	var thumbURL string

	// Option A: LLM-based Smart Selection (if provider available)
	if h.llm != nil {
		thumbURL = h.selectThumbnailWithLLM(ctx, title, lang, p)
	}

	// Option B: Fallback to Heuristic (if LLM failed or not available)
	if thumbURL == "" {
		slog.Debug("Thumbnail: LLM selection failed or unavailable, falling back to heuristics", "poi", p.NameEn)
		thumbURL, err = h.wp.GetThumbnail(ctx, title, lang)
		if err != nil {
			return "", err
		}
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

// findBestFilenameMatch attempts to match the LLM's selection against the valid filenames list.
// It handles exact matches, case-insensitive matches, and "File:" prefix variations.
func findBestFilenameMatch(selected string, filenames []string) string {
	// Verify the LLM returned one of the valid filenames (hallucination check)
	// Also clean up any quotes or markdown
	selected = strings.Trim(selected, "\"`'")

	// Find exact match in original list to ensure valid filename handling
	for _, f := range filenames {
		// Loose match to handle potential LLM formatting differences
		if strings.EqualFold(f, selected) || strings.HasSuffix(f, selected) {
			return f
		}
	}

	// If LLM returned "File:Foo.jpg" but list had "Foo.jpg", try stripping "File:"
	cleanSelected := strings.TrimPrefix(selected, "File:")
	for _, f := range filenames {
		cleanF := strings.TrimPrefix(f, "File:")
		if strings.EqualFold(cleanF, cleanSelected) {
			return f
		}
	}

	return ""
}

func (h *POIHandler) selectThumbnailWithLLM(ctx context.Context, title, lang string, p *model.POI) string {
	filenames, err := h.wp.GetImageFilenames(ctx, title, lang)
	if err != nil {
		slog.Warn("Thumbnail: Failed to fetch filenames for LLM selection", "error", err)
		return ""
	}

	if len(filenames) == 0 {
		return ""
	}

	// Constrain list size to avoid context headers/costs? 50 is fine usually.
	if len(filenames) > 50 {
		filenames = filenames[:50]
	}

	data := struct {
		Name     string
		Category string
		Images   []string
	}{
		Name:     p.NameEn,
		Category: p.Category,
		Images:   filenames,
	}

	var prompt string
	var errPrompt error
	if h.promptMgr != nil {
		prompt, errPrompt = h.promptMgr.Render("context/thumbnail_selector.tmpl", data)
		if errPrompt != nil {
			slog.Error("Thumbnail: Failed to execute prompt template", "error", errPrompt)
			return ""
		}
	} else {
		slog.Warn("Thumbnail: Prompt manager missing")
		return ""
	}

	resp, err := h.llm.GenerateText(ctx, "thumbnails", prompt)
	if err != nil {
		slog.Warn("Thumbnail: LLM generation failed", "error", err)
		return ""
	}

	selected := strings.TrimSpace(resp)
	if selected == "" {
		return ""
	}

	bestMatch := findBestFilenameMatch(selected, filenames)

	if bestMatch != "" {
		// Resolve URL
		slog.Info("Thumbnail: LLM selected image", "poi", p.NameEn, "selected", bestMatch)
		url, err := h.wp.GetImageURL(ctx, bestMatch, lang)
		if err != nil {
			slog.Warn("Thumbnail: Failed to resolve URL for selected image", "image", bestMatch, "error", err)
			return ""
		}
		return url
	}

	slog.Debug("Thumbnail: LLM returned invalid filename", "response", selected)
	return ""
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
