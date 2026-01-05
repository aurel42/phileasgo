package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"phileasgo/pkg/wikidata"
)

// CacheHandler handles tile cache visualization requests.
type CacheHandler struct {
	service *wikidata.Service

	// API Cache (15s TTL)
	mu         sync.RWMutex
	cachedResp []byte
	lastUpdate time.Time
}

// NewCacheHandler creates a new CacheHandler.
func NewCacheHandler(s *wikidata.Service) *CacheHandler {
	return &CacheHandler{
		service: s,
	}
}

func (h *CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse params
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	if latStr == "" || lonStr == "" {
		http.Error(w, "lat and lon are required", http.StatusBadRequest)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)
	if err1 != nil || err2 != nil {
		http.Error(w, "invalid lat/lon", http.StatusBadRequest)
		return
	}

	// Constants
	const searchRadiusKm = 100.0

	// Check API Cache
	h.mu.RLock()
	if time.Since(h.lastUpdate) < 15*time.Second && h.cachedResp != nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(h.cachedResp)
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	// Cache Miss (or Stale): Compute
	tiles, err := h.service.GetCachedTiles(r.Context(), lat, lon, searchRadiusKm)
	if err != nil {
		http.Error(w, "failed to fetch tiles", http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(tiles)
	if err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
		return
	}

	// Update API Cache
	h.mu.Lock()
	h.cachedResp = resp
	h.lastUpdate = time.Now()
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp)
}
