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
	"phileasgo/pkg/model"
	"phileasgo/pkg/scorer"
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
		m.logger.Debug("Upserted POI", "qid", p.WikidataID, "name", p.DisplayName())
	} else {
		m.logger.Debug("Tracked POI (hydrated)", "qid", p.WikidataID, "name", p.DisplayName())
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

	// 1. Separate "Played" (Blue markers) from "Candidates"
	var played []*model.POI
	var visibleCandidates []*model.POI

	for _, p := range m.trackedPOIs {
		// UI explicitly does NOT filter by ground/air state to allow seeing what's around you on the ground map.
		// Only show played items if they are still within the "Recent History" window (TTL).
		// Once expired, they drop off the "Played" list and must compete by score again.
		if !m.isPlayable(p) {
			played = append(played, p)
		}
		if p.IsVisible {
			visibleCandidates = append(visibleCandidates, p)
		}
	}

	// 2. Calculate Effective Threshold
	effectiveThreshold := minScore
	if filterMode == "adaptive" && len(visibleCandidates) > 0 {
		// Sort by score descending to find the cutoff
		sort.Slice(visibleCandidates, func(i, j int) bool {
			return visibleCandidates[i].Score > visibleCandidates[j].Score
		})

		if len(visibleCandidates) > targetCount {
			effectiveThreshold = visibleCandidates[targetCount-1].Score
		} else {
			effectiveThreshold = 0.0 // All visible qualify
		}
	}

	// 3. Assemble final list: All Played OR (Visible AND Score >= Threshold)
	resultMap := make(map[string]*model.POI)
	for _, p := range played {
		resultMap[p.WikidataID] = p
	}
	for _, p := range visibleCandidates {
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
// Filters: Playable (TTL), Visible, Score >= minScore (if set), Ground Logic (Aerodrome only).
func (m *Manager) GetNarrationCandidates(limit int, minScore *float64, isOnGround bool) []*model.POI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	candidates := make([]*model.POI, 0, len(m.trackedPOIs))

	for _, p := range m.trackedPOIs {
		// 1. Ground Logic
		if isOnGround && !strings.EqualFold(p.Category, "aerodrome") {
			continue
		}

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

// StartScoring starts the background scoring loop.
func (m *Manager) StartScoring(ctx context.Context, simClient sim.Client, sc *scorer.Scorer) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	m.logger.Info("Starting Scoring Loop")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performScoringPass(ctx, simClient, sc)
		}
	}
}

func (m *Manager) performScoringPass(ctx context.Context, simClient sim.Client, sc *scorer.Scorer) {
	// Skip scoring if sim is not active
	if simClient.GetState() != sim.StateActive {
		return
	}

	// 1. Get Telemetry
	telemetry, err := simClient.GetTelemetry(ctx)
	if err != nil {
		m.logger.Warn("Failed to get telemetry for scoring", "error", err)
		return
	}

	// Instrumentation: Log prediction offset distance
	if telemetry.PredictedLatitude != 0 || telemetry.PredictedLongitude != 0 {
		currentPos := geo.Point{Lat: telemetry.Latitude, Lon: telemetry.Longitude}
		predictedPos := geo.Point{Lat: telemetry.PredictedLatitude, Lon: telemetry.PredictedLongitude}
		predDistMeters := geo.Distance(currentPos, predictedPos)
		predDistNM := predDistMeters / 1852.0
		m.logger.Debug("Scoring: Prediction offset",
			"dist_nm", fmt.Sprintf("%.2f", predDistNM),
			"groundspeed_kts", fmt.Sprintf("%.0f", telemetry.GroundSpeed),
		)
	}

	// 2. Fetch History for Variety Scoring
	// We fetch last 1 hour, which is plenty for variety (usually < 10 items)
	since := time.Now().Add(-1 * time.Hour)
	recent, err := m.store.GetRecentlyPlayedPOIs(ctx, since)
	var history []string
	if err == nil {
		// recent is DESC (newest first). Scorer expects Oldest -> Newest.
		for i := len(recent) - 1; i >= 0; i-- {
			history = append(history, recent[i].Category)
		}
	} else {
		m.logger.Warn("Failed to fetch recent history for scoring", "error", err)
	}

	// 3. Lock & Score
	m.mu.Lock()

	// Fetch Boost Factor
	boostFactor := 1.0
	val, ok := m.store.GetState(ctx, "visibility_boost")
	if ok && val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			boostFactor = f
		}
	}

	input := scorer.ScoringInput{
		Telemetry:       telemetry,
		CategoryHistory: history,
		NarratorConfig:  &m.config.Narrator,
		BoostFactor:     boostFactor,
	}

	// Create Scoring Session (Pre-calculates terrain/context once)
	session := sc.NewSession(&input)

	for _, p := range m.trackedPOIs {
		session.Calculate(p)
	}

	// 4. Update Last Scored Location
	// This allows consumers (Scheduler) to verify consistency
	m.lastScoredLat = telemetry.Latitude
	m.lastScoredLon = telemetry.Longitude

	// 5. Trigger Callback (if set) - BEFORE unlocking to ensure consistency?
	// Actually, better AFTER unlocking to avoid blocking the manager lock for the callback duration.
	callback := m.onScoringComplete
	valleyCallback := m.onValleyAltitude
	lowestElev := session.LowestElevation()

	m.mu.Unlock()

	if callback != nil {
		// Execute callback outside the lock
		callback(ctx, &telemetry)
	}
	if valleyCallback != nil {
		valleyCallback(lowestElev)
	}
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
