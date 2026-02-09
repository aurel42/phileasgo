package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/store"
	"phileasgo/pkg/wikipedia"
)

// thumbnailFlight holds an in-flight thumbnail fetch for request coalescing.
type thumbnailFlight struct {
	done   chan struct{} // Closed when fetch completes
	result string        // Result URL (empty string if failed)
	err    error         // Error if fetch failed
}

// POIHandler exposes POI data to the frontend.
type POIHandler struct {
	mgr       *poi.Manager
	wp        *wikipedia.Client
	store     store.Store
	llm       llm.Provider
	promptMgr *prompts.Manager

	// thumbnailFlights coalesces concurrent thumbnail requests for the same POI.
	// Key is POI WikidataID, value is the in-flight request.
	thumbnailFlightMu sync.Mutex
	thumbnailFlights  map[string]*thumbnailFlight
}

// NewPOIHandler creates a new POI handler.
func NewPOIHandler(mgr *poi.Manager, wp *wikipedia.Client, st store.Store, llmProv llm.Provider, promptMgr *prompts.Manager) *POIHandler {
	return &POIHandler{
		mgr:              mgr,
		wp:               wp,
		store:            st,
		llm:              llmProv,
		promptMgr:        promptMgr,
		thumbnailFlights: make(map[string]*thumbnailFlight),
	}
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
// Uses singleflight pattern to coalesce concurrent requests for the same POI.
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

	// If thumbnail already cached, return it immediately
	if p.ThumbnailURL != "" {
		h.respondThumbnail(w, p.ThumbnailURL)
		return
	}

	// Singleflight: Check if a fetch is already in progress for this POI
	h.thumbnailFlightMu.Lock()
	if flight, ok := h.thumbnailFlights[poiID]; ok {
		// Another request is already fetching; wait for it
		h.thumbnailFlightMu.Unlock()
		<-flight.done // Wait for completion
		if flight.err != nil {
			slog.Warn("Failed to fetch thumbnail (waited)", "poi_id", poiID, "error", flight.err)
			h.respondThumbnail(w, "")
			return
		}
		h.respondThumbnail(w, flight.result)
		return
	}

	// We are the first request; create a flight record
	flight := &thumbnailFlight{done: make(chan struct{})}
	h.thumbnailFlights[poiID] = flight
	h.thumbnailFlightMu.Unlock()

	// Fetch thumbnail (this is the expensive LLM call)
	thumbURL, fetchErr := h.fetchAndCacheThumbnail(r.Context(), p)

	// Store result in flight struct
	flight.result = thumbURL
	flight.err = fetchErr

	// Signal completion to all waiters
	close(flight.done)

	// Clean up the flight record
	h.thumbnailFlightMu.Lock()
	delete(h.thumbnailFlights, poiID)
	h.thumbnailFlightMu.Unlock()

	if fetchErr != nil {
		slog.Warn("Failed to fetch thumbnail", "poi_id", poiID, "error", fetchErr)
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

	images, err := h.wp.GetImagesWithURLs(ctx, title, lang)
	if err != nil {
		slog.Warn("Thumbnail: Failed to fetch image candidates", "poi", p.NameEn, "error", err)
		return "", err
	}

	if len(images) == 0 {
		return "", nil
	}

	var thumbURL string

	// 1. Try LLM-based Smart Selection (if provider available)
	if h.llm != nil {
		thumbURL = h.selectThumbnailFromCandidates(ctx, images, title, lang, p)
	}

	// 2. Fallback to Heuristic (if LLM failed or not available)
	if thumbURL == "" {
		slog.Debug("Thumbnail: Falling back to heuristics", "poi", p.NameEn)
		// Simply pick the first one that isn't unwanted (GetImagesWithURLs already filters most)
		if len(images) > 0 {
			thumbURL = images[0].URL
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

func (h *POIHandler) selectThumbnailFromCandidates(ctx context.Context, images []wikipedia.ImageResult, title, lang string, p *model.POI) string {
	// Constrain list size (though GetImagesWithURLs already limits to 50)
	if len(images) > 50 {
		images = images[:50]
	}

	// Construct Article URL if missing
	articleURL := p.WPURL
	if articleURL == "" {
		articleURL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, title)
	}

	data := struct {
		Name       string
		Category   string
		ArticleURL string
		Images     []wikipedia.ImageResult
	}{
		Name:       p.DisplayName(),
		Category:   p.Category,
		ArticleURL: articleURL,
		Images:     images,
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
	selected = strings.Trim(selected, "\"`'")

	// Find match in our list (LLM returns the URL or filename)
	for _, img := range images {
		if strings.EqualFold(img.URL, selected) {
			slog.Debug("Thumbnail: LLM selected image", "poi", p.DisplayName(), "url", img.URL)
			return img.URL
		}
		if strings.EqualFold(img.Title, selected) || strings.EqualFold(strings.TrimPrefix(img.Title, "File:"), selected) {
			slog.Debug("Thumbnail: LLM selected image by filename", "poi", p.DisplayName(), "url", img.URL)
			return img.URL
		}
	}

	slog.Debug("Thumbnail: LLM returned invalid selection", "response", selected)
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

// HandleSettlements handles GET /api/map/settlements.
// It returns a list of tracked POIs within the given bounds,
// prioritized by "Tier" (City > Town > Village).
// Query Params: minLat, maxLat, minLon, maxLon, zoom
func (h *POIHandler) HandleSettlements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse Bounds
	bounds, err := parseBounds(r)
	if err != nil {
		http.Error(w, "Missing or invalid bounds", http.StatusBadRequest)
		return
	}

	// 2. Get Tracked POIs (Thread-safe copy)
	tracked := h.mgr.GetTrackedPOIs()

	// 3. Filter by Bounds
	candidates := filterByBounds(tracked, bounds)

	// 4. Apply Tier Strategy
	finalResult := applyTierStrategy(candidates)

	// Sort by Score/Population (using Sitelinks or Score as proxy)
	sort.Slice(finalResult, func(i, j int) bool {
		return finalResult[i].Score > finalResult[j].Score
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(finalResult); err != nil {
		slog.Error("Failed to encode settlements", "error", err)
	}
}

type mapBounds struct {
	minLat, maxLat, minLon, maxLon float64
}

func parseBounds(r *http.Request) (mapBounds, error) {
	q := r.URL.Query()
	minLat, _ := strconv.ParseFloat(q.Get("minLat"), 64)
	maxLat, _ := strconv.ParseFloat(q.Get("maxLat"), 64)
	minLon, _ := strconv.ParseFloat(q.Get("minLon"), 64)
	maxLon, _ := strconv.ParseFloat(q.Get("maxLon"), 64)

	if minLat == 0 && maxLat == 0 && minLon == 0 && maxLon == 0 {
		return mapBounds{}, fmt.Errorf("missing bounds")
	}
	return mapBounds{minLat, maxLat, minLon, maxLon}, nil
}

func filterByBounds(pois []*model.POI, b mapBounds) []*model.POI {
	var candidates []*model.POI
	for _, p := range pois {
		if p.Lat >= b.minLat && p.Lat <= b.maxLat && p.Lon >= b.minLon && p.Lon <= b.maxLon {
			candidates = append(candidates, p)
		}
	}
	return candidates
}

func applyTierStrategy(candidates []*model.POI) []*model.POI {
	hasCity := false
	hasTown := false

	for _, p := range candidates {
		cat := strings.ToLower(p.Category)
		if cat == "city" {
			hasCity = true
		} else if cat == "town" {
			hasTown = true
		}
	}

	var finalResult []*model.POI
	for _, p := range candidates {
		cat := strings.ToLower(p.Category)

		if hasCity {
			if cat == "city" {
				finalResult = append(finalResult, p)
			}
			continue
		}

		if hasTown {
			if cat == "town" {
				finalResult = append(finalResult, p)
			}
			continue
		}

		finalResult = append(finalResult, p)
	}
	return finalResult
}
