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
	minLatStr := r.URL.Query().Get("min_lat")
	maxLatStr := r.URL.Query().Get("max_lat")
	minLonStr := r.URL.Query().Get("min_lon")
	maxLonStr := r.URL.Query().Get("max_lon")

	if minLatStr == "" || maxLatStr == "" || minLonStr == "" || maxLonStr == "" {
		http.Error(w, "min_lat, max_lat, min_lon, max_lon are required", http.StatusBadRequest)
		return
	}

	minLat, err1 := strconv.ParseFloat(minLatStr, 64)
	maxLat, err2 := strconv.ParseFloat(maxLatStr, 64)
	minLon, err3 := strconv.ParseFloat(minLonStr, 64)
	maxLon, err4 := strconv.ParseFloat(maxLonStr, 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		http.Error(w, "invalid bounds", http.StatusBadRequest)
		return
	}

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
	tiles, err := h.service.GetCachedTiles(r.Context(), minLat, maxLat, minLon, maxLon)
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
