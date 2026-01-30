package poi

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/logging"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
)

// ManagerStore defines the store interface required by the POI Manager.
// It combines POIStore and MSFSPOIStore for the Manager's needs.
type ManagerStore interface {
	store.POIStore
	store.MSFSPOIStore
	store.StateStore
}

// Loader abstracts POI hydration (breaks circular dependency).
type Loader interface {
	// EnsurePOIsLoaded ensures that the requested entities are classified and enriched.
	EnsurePOIsLoaded(ctx context.Context, qids []string, lat, lon float64) error
	// GetPOIsNear returns all cached POIs near the given coordinate.
	GetPOIsNear(ctx context.Context, lat, lon, radiusMeters float64) ([]*model.POI, error)
}

// RiverSentinel abstracts the river engine.
type RiverSentinel interface {
	Update(lat, lon, heading float64) *model.RiverCandidate
}

// Returns *rivers.Candidate or nil

type Manager struct {
	config *config.Config
	store  ManagerStore
	logger *slog.Logger

	// Active Tracking
	mu          sync.RWMutex
	trackedPOIs map[string]*model.POI

	// Config for hydration
	catConfig *config.CategoriesConfig

	// Consistency State
	lastScoredLat float64
	lastScoredLon float64

	// Callbacks
	onScoringComplete func(ctx context.Context, t *sim.Telemetry)
	onValleyAltitude  func(altMeters float64)

	// River Integration (set via setter to break circular dependency)
	poiLoader     Loader
	riverSentinel RiverSentinel
}

// NewManager creates a new POI Manager.
func NewManager(cfg *config.Config, s ManagerStore, catCfg *config.CategoriesConfig) *Manager {
	return &Manager{
		config:      cfg,
		store:       s,
		logger:      slog.With("component", "poi_manager"),
		trackedPOIs: make(map[string]*model.POI),
		catConfig:   catCfg,
	}
}

// SetPOILoader injects the POI loader (typically the WikidataService).
func (m *Manager) SetPOILoader(pl Loader) {
	m.poiLoader = pl
}

// SetRiverSentinel injects the RiverSentinel.
func (m *Manager) SetRiverSentinel(rs RiverSentinel) {
	m.riverSentinel = rs
}

// SetScoringCallback sets the function to be called after each scoring pass.
func (m *Manager) SetScoringCallback(fn func(ctx context.Context, t *sim.Telemetry)) {
	m.onScoringComplete = fn
}

// SetValleyAltitudeCallback sets the function to be called with the calculated valley floor altitude.
func (m *Manager) SetValleyAltitudeCallback(fn func(altMeters float64)) {
	m.onValleyAltitude = fn
}

// UpsertPOI saves a POI to the database and adds it to the active tracking list.
// It performs in-place updates on existing pointers in the active cache to ensure
// pointer consistency across the application (e.g. for in-progress narrations).
func (m *Manager) UpsertPOI(ctx context.Context, p *model.POI) error {
	return m.upsertInternal(ctx, p, true)
}

// TrackPOI adds or updates a POI in the active tracking map without saving to the database.
// This is useful for hydrating the in-memory state from the DB during ingestion.
func (m *Manager) TrackPOI(ctx context.Context, p *model.POI) error {
	return m.upsertInternal(ctx, p, false)
}

func (m *Manager) upsertInternal(ctx context.Context, p *model.POI, shouldSave bool) error {
	// 0. Name Validation: Drop POIs without ANY valid name
	if p.NameEn == "" && p.NameLocal == "" && p.NameUser == "" {
		m.logger.Debug("Dropping nameless POI", "qid", p.WikidataID)
		return nil
	}

	// 0a. Pre-check MSFS Overlap if needed
	if err := m.EnrichWithMSFS(ctx, p); err != nil {
		m.logger.Warn("Failed to check MSFS overlap", "qid", p.WikidataID, "error", err)
	}

	// 1. Ensure Icon Availability (Heal on Load)
	m.ensureIcon(p)

	m.mu.Lock()

	existing, ok := m.trackedPOIs[p.WikidataID]
	isNew := !ok
	if !ok {
		// Not in active cache, check DB
		dbPOI, err := m.store.GetPOI(ctx, p.WikidataID)
		if err == nil && dbPOI != nil {
			existing = dbPOI // Note: existing DB POI might also have missing icon, but we replace/update below
		}
	}

	if existing != nil {
		m.updateExistingPOI(existing, p)
		p = existing
	}

	// 2. Ensure it's in the active cache
	m.trackedPOIs[p.WikidataID] = p
	m.mu.Unlock()

	// 3. Save to DB (optional)
	if shouldSave {
		if err := m.store.SavePOI(ctx, p); err != nil {
			return fmt.Errorf("%w: failed to save POI %s: %v", ErrStoreFailure, p.WikidataID, err)
		}
		logging.Trace(m.logger, "Upserted POI", "qid", p.WikidataID, "name", p.DisplayName())
	} else if isNew {
		// Only log hydration for genuinely new POIs, not duplicates from overlapping tiles
		logging.Trace(m.logger, "Tracked POI (hydrated)", "qid", p.WikidataID, "name", p.DisplayName())
	}

	return nil
}

// ensureIcon populates the POI icon if missing, using config or internal defaults.
func (m *Manager) ensureIcon(p *model.POI) {
	if p.Icon != "" {
		return
	}
	if p.Category == "" {
		return
	}

	// 1. Check Config (Case-Insensitive)
	if m.catConfig != nil {
		if cfg, ok := m.catConfig.Categories[strings.ToLower(p.Category)]; ok {
			if cfg.Icon != "" {
				p.Icon = cfg.Icon
				return
			}
		}
	}

	// 2. Dimension/Internal Fallbacks
	switch strings.ToLower(p.Category) {
	case "area":
		p.Icon = "circle-stroked"
	case "height":
		p.Icon = "cemetery-JP" // Specific reuse of tower-like icon
	case "length":
		p.Icon = "arrow"
	case "landmark":
		p.Icon = "monument"
	}
}

// EnrichWithMSFS checks for MSFS overlap and updates the POI if found.
func (m *Manager) EnrichWithMSFS(ctx context.Context, p *model.POI) error {
	if p.IsMSFSPOI {
		return nil
	}
	size := "M"
	if m.catConfig != nil {
		size = m.catConfig.GetSize(p.Category)
	}
	radius := 0.0
	if m.catConfig != nil {
		radius = m.catConfig.GetMergeDistance(size)
	} else {
		radius = 500.0 // Fallback
	}

	isOverlap, err := m.store.CheckMSFSPOI(ctx, p.Lat, p.Lon, radius)
	if err != nil {
		return err
	}
	if isOverlap {
		p.IsMSFSPOI = true
		m.logger.Debug("MSFS Overlap Detected", "qid", p.WikidataID, "name", p.DisplayName())
	}
	return nil
}

func (m *Manager) updateExistingPOI(existing, p *model.POI) {
	// 1. In-place Update to maintain pointer stability.
	existing.Category = p.Category
	existing.Lat = p.Lat
	existing.Lon = p.Lon
	existing.Sitelinks = p.Sitelinks
	existing.NameEn = p.NameEn
	existing.NameLocal = p.NameLocal
	existing.NameUser = p.NameUser
	existing.WPURL = p.WPURL
	existing.WPArticleLength = p.WPArticleLength
	existing.DimensionMultiplier = p.DimensionMultiplier
	existing.Icon = p.Icon
	existing.IsMSFSPOI = p.IsMSFSPOI // Update flag

	// 2. Metadata Preservation
	if !p.LastPlayed.IsZero() && p.LastPlayed.After(existing.LastPlayed) {
		existing.LastPlayed = p.LastPlayed
	}
	if existing.CreatedAt.IsZero() && !p.CreatedAt.IsZero() {
		existing.CreatedAt = p.CreatedAt
	}
	if existing.TriggerQID == "" {
		existing.TriggerQID = p.TriggerQID
	}
}

// GetPOI returns a POI by its ID, checking active cache first then DB.
func (m *Manager) GetPOI(ctx context.Context, qid string) (*model.POI, error) {
	m.mu.RLock()
	p, ok := m.trackedPOIs[qid]
	m.mu.RUnlock()
	if ok {
		return p, nil
	}
	p, err := m.store.GetPOI(ctx, qid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStoreFailure, err)
	}
	if p == nil {
		return nil, ErrPOINotFound
	}
	return p, nil
}

// GetTrackedPOIs returns a thread-safe copy of currently tracked POIs.
func (m *Manager) GetTrackedPOIs() []*model.POI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*model.POI, 0, len(m.trackedPOIs))
	for _, p := range m.trackedPOIs {
		list = append(list, p)
	}
	return list
}

// PruneTracked removes POIs that are too far or too old.
// For now, simple active list management. We can implement distance-based pruning later if needed.
func (m *Manager) PruneTracked(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for id, p := range m.trackedPOIs {
		// Use CreatedAt or LastPlayed? For ingestion/tracking context, CreatedAt is when we fetched it.
		// If we haven't touched it in a while, maybe drop it?
		// For now simple time-based pruning to keep memory sane.
		if now.Sub(p.CreatedAt) > olderThan {
			delete(m.trackedPOIs, id)
			count++
		}
	}
	if count > 0 {
		m.logger.Debug("Pruned tracked POIs (Time)", "removed", count, "remaining", len(m.trackedPOIs))
	}
	return count
}

// PruneByDistance removes POIs that are beyond the threshold distance and in the rear semi-circle.
func (m *Manager) PruneByDistance(lat, lon, heading, thresholdKm float64) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	// Pre-calculate threshold in meters for geo.Distance check? geo.Distance returns meters.
	// thresholdKm is in KM.
	thresholdM := thresholdKm * 1000.0

	for id, p := range m.trackedPOIs {
		// 1. Distance Check
		distM := geo.Distance(geo.Point{Lat: lat, Lon: lon}, geo.Point{Lat: p.Lat, Lon: p.Lon})
		if distM <= thresholdM {
			continue // Keep it
		}

		// 2. Relative Bearing Check (Are we moving AWAY from it?)
		// Calculate bearing FROM aircraft TO POI
		bearingToPOI := geo.Bearing(geo.Point{Lat: lat, Lon: lon}, geo.Point{Lat: p.Lat, Lon: p.Lon})

		// Diff = Abs(Heading - Bearing)
		// We want to know if it's "Behind" us.
		// Behind means the angle difference is > 90 degrees.
		diff := math.Abs(heading - bearingToPOI)
		if diff > 180 {
			diff = 360 - diff
		}

		if diff > 90 {
			// It is behind us and far away. Evict.
			delete(m.trackedPOIs, id)
			count++
		}
	}

	if count > 0 {
		m.logger.Debug("Pruned tracked POIs (Distance)", "removed", count, "remaining", len(m.trackedPOIs))
	}
	return count
}

// CountScoredAbove returns the number of tracked POIs with a score strictly greater than the threshold.
// It stops counting once the limit is reached to save resources.
// GetPOIsForUI returns a list of POIs base on visibility, quality and persistence rules.
// It returns the filtered list and the effective threshold used (for logging/UI).
// Unlike GetNarrationCandidates, this includes "Played" items for history display and ignores ground logic.
func (m *Manager) GetPOIsForUI(filterMode string, targetCount int, minScore float64) (pois []*model.POI, threshold float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Separate "Played" (Blue markers) from "Playable Candidates"
	var played []*model.POI
	var playableVisible []*model.POI

	for _, p := range m.trackedPOIs {
		// Only show played items if they are still within the "Recent History" window (TTL).
		// Once expired, they drop off the "Played" list and must compete by score again.
		if !m.isPlayable(p) {
			played = append(played, p)
		} else if p.IsVisible {
			playableVisible = append(playableVisible, p)
		}
	}

	// 2. Calculate Effective Threshold (Only based on Playable candidates)
	effectiveThreshold := minScore
	if filterMode == "adaptive" && len(playableVisible) > 0 {
		// Sort by score descending to find the cutoff
		sort.Slice(playableVisible, func(i, j int) bool {
			return playableVisible[i].Score > playableVisible[j].Score
		})

		if len(playableVisible) > targetCount {
			effectiveThreshold = playableVisible[targetCount-1].Score
		} else {
			effectiveThreshold = -math.MaxFloat64 // All playable qualify
		}
	}

	// 3. Assemble final list: All Recently Played OR (Visible AND Playable AND Score >= Threshold)
	resultMap := make(map[string]*model.POI)
	for _, p := range played {
		resultMap[p.WikidataID] = p
	}
	for _, p := range playableVisible {
		if p.Score >= effectiveThreshold {
			resultMap[p.WikidataID] = p
		}
	}

	result := make([]*model.POI, 0, len(resultMap))
	for _, p := range resultMap {
		result = append(result, p)
	}

	// 4. Stable sort for API output consistency
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].WikidataID < result[j].WikidataID
	})

	return result, effectiveThreshold
}

// GetNarrationCandidates returns a list of POIs strictly filtered for narration eligibility.
// Filters: Playable (TTL), Visible, Score >= minScore (if set).
func (m *Manager) GetNarrationCandidates(limit int, minScore *float64) []*model.POI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	candidates := make([]*model.POI, 0, len(m.trackedPOIs))

	for _, p := range m.trackedPOIs {

		// 2. Playability (Cooldown)
		if !m.isPlayable(p) {
			continue
		}

		// 3. Visibility
		if !p.IsVisible {
			continue
		}

		// 4. Score Threshold (if provided)
		if minScore != nil && p.Score < *minScore {
			continue
		}

		candidates = append(candidates, p)
	}

	// Sort Descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > limit {
		return candidates[:limit]
	}
	return candidates
}

// isPlayable helper checks if a POI is on cooldown.
func (m *Manager) isPlayable(p *model.POI) bool {
	if p.LastPlayed.IsZero() {
		return true
	}
	return time.Since(p.LastPlayed) >= time.Duration(m.config.Narrator.RepeatTTL)
}

// CountScoredAbove returns the number of tracked POIs with a score strictly greater than the threshold.
// It stops counting once the limit is reached to save resources.
func (m *Manager) CountScoredAbove(threshold float64, limit int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, p := range m.trackedPOIs {
		// Only count valid competitors (Playable POIs).
		// If a neighbor is on cooldown, they are "silent" and don't contribute to competition clatter.
		if !m.isPlayable(p) {
			continue
		}

		if p.Score > threshold {
			count++
			if count >= limit {
				return limit
			}
		}
	}
	return count
}

// ActiveCount returns the number of currently tracked POIs.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.trackedPOIs)
}

// UpdateScoringState updates the last scored position in a thread-safe way.
func (m *Manager) UpdateScoringState(lat, lon float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastScoredLat = lat
	m.lastScoredLon = lon
}

// NotifyScoringComplete triggers the registered callbacks.
func (m *Manager) NotifyScoringComplete(ctx context.Context, t *sim.Telemetry, lowestElev float64) {
	// Callbacks are function pointers, safe to read if set at startup.
	// If dynamic setting is needed, we should lock/atomic load.
	// Assuming setup happens before runtime or we lock.
	// Current SetScoringCallback is not locked, but usually called at init.
	if m.onScoringComplete != nil {
		m.onScoringComplete(ctx, t)
	}
	if m.onValleyAltitude != nil {
		m.onValleyAltitude(lowestElev)
	}
}

// FetchHistory returns the list of recently played categories for variety scoring.
func (m *Manager) FetchHistory(ctx context.Context) ([]string, error) {
	// Fetch last 1 hour
	since := time.Now().Add(-1 * time.Hour)
	recent, err := m.store.GetRecentlyPlayedPOIs(ctx, since)
	if err != nil {
		return nil, err
	}
	var history []string
	// recent is DESC (newest first). Scorer expects Oldest -> Newest?
	// Original code:
	// for i := len(recent) - 1; i >= 0; i-- { history = append(history, recent[i].Category) }
	for i := len(recent) - 1; i >= 0; i-- {
		history = append(history, recent[i].Category)
	}
	return history, nil
}

// GetBoostFactor retrisves the visibility boost factor.
func (m *Manager) GetBoostFactor(ctx context.Context) float64 {
	val, ok := m.store.GetState(ctx, "visibility_boost")
	if ok && val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 1.0
}

// ResetLastPlayed resets the last_played timestamp for POIs within the given radius (meters).
func (m *Manager) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error {
	// 1. Reset in-memory state for immediate feedback.
	// We reset ALL tracked POIs (simplified approach) to ensure immediate UI update
	// for everything currently loaded/visible.
	m.mu.Lock()
	for _, p := range m.trackedPOIs {
		p.LastPlayed = time.Time{} // Clear
	}
	m.mu.Unlock()

	// 2. Reset in DB (remains spatial to preserve far history if relevant)
	return m.store.ResetLastPlayed(ctx, lat, lon, radius)
}

// ResetSession clears the in-memory cache of tracked POIs.
// This is called on teleportation to remove POIs from the previous location.
// It does NOT clear the database history (preserved for "seen" filtering).
func (m *Manager) ResetSession(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear all tracked POIs
	// Go optimized map clearing:
	for k := range m.trackedPOIs {
		delete(m.trackedPOIs, k)
	}
	// Or simply reallocate: m.trackedPOIs = make(map[string]*model.POI)
	// Reallocation is safer to avoid GC overhead of large maps if map size varies wildly.
	m.trackedPOIs = make(map[string]*model.POI)

	// Reset consistency state
	m.lastScoredLat = 0
	m.lastScoredLon = 0

	m.logger.Info("POIManager: Session reset (cache cleared)")
}

// LastScoredPosition returns the location used for the most recent scoring pass.
// Returns 0,0 if no scoring has occurred yet.
func (m *Manager) LastScoredPosition() (lat, lon float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastScoredLat, m.lastScoredLon
}

// UpdateRivers is called periodically to detect nearby rivers and hydrate matching POIs.
// It should only be called from a dedicated 15s ticker, NOT the main scoring loop.
func (m *Manager) UpdateRivers(ctx context.Context, lat, lon, heading float64) (*model.POI, error) {
	// 1. Check dependencies
	if m.riverSentinel == nil || m.poiLoader == nil {
		return nil, nil // Not configured, skip silently
	}

	// 2. Call Sentinel
	candidate := m.riverSentinel.Update(lat, lon, heading)
	if candidate == nil {
		return nil, nil // No river ahead
	}

	m.logger.Debug("River candidate detected", "name", candidate.Name, "qid", candidate.WikidataID, "dist", candidate.Distance)

	// 3. Check if already tracked to avoid redundant hydration
	m.mu.RLock()
	p, alreadyTracked := m.trackedPOIs[candidate.WikidataID]
	m.mu.RUnlock()

	if alreadyTracked {
		// Just update the position and context silently
		p.RiverContext = &model.RiverContext{
			IsActive:   true,
			DistanceM:  candidate.Distance,
			ClosestLat: candidate.ClosestLat,
			ClosestLon: candidate.ClosestLon,
		}
		p.Lat = candidate.ClosestLat
		p.Lon = candidate.ClosestLon
		m.logger.Debug("Updated existing river POI coordinates", "qid", p.WikidataID, "name", p.DisplayName())
		return p, nil
	}

	// 4. Hydrate by QID (First time discovery)
	if err := m.poiLoader.EnsurePOIsLoaded(ctx, []string{candidate.WikidataID}, lat, lon); err != nil {
		m.logger.Warn("Failed to hydrate river by QID", "qid", candidate.WikidataID, "err", err)
	}

	// 5. Fetch and Track (Discovery path)
	p, err := m.GetPOI(ctx, candidate.WikidataID)
	if err == nil && p != nil {
		// Found! Attach context, update position, and TRACK
		p.RiverContext = &model.RiverContext{
			IsActive:   true,
			DistanceM:  candidate.Distance,
			ClosestLat: candidate.ClosestLat,
			ClosestLon: candidate.ClosestLon,
		}
		// Jump coordinates to aircraft vicinity (15s design)
		p.Lat = candidate.ClosestLat
		p.Lon = candidate.ClosestLon

		// Ensure it's in the active cache (Tracker)
		if err := m.TrackPOI(ctx, p); err != nil {
			m.logger.Warn("Failed to track river POI", "qid", p.WikidataID, "error", err)
		}

		m.logger.Info("Hydrated and tracked river POI", "qid", p.WikidataID, "name", p.DisplayName())
		return p, nil
	}

	// 5. Fallback: No POI found
	m.logger.Debug("River detected but no matching POI found", "name", candidate.Name, "qid", candidate.WikidataID)
	return nil, nil
}

// GetPOIsNear returns POIs from the tracked cache within radiusMeters of the given point.
func (m *Manager) GetPOIsNear(lat, lon, radiusMeters float64) []*model.POI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*model.POI
	for _, p := range m.trackedPOIs {
		dist := geo.Distance(geo.Point{Lat: lat, Lon: lon}, geo.Point{Lat: p.Lat, Lon: p.Lon})
		if dist <= radiusMeters {
			result = append(result, p)
		}
	}
	return result
}
