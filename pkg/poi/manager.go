package poi

import (
	"context"
	"fmt"
	"log/slog"
	"math"
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

type Manager struct {
	config *config.Config
	store  store.Store
	logger *slog.Logger

	// Active Tracking
	mu          sync.RWMutex
	trackedPOIs map[string]*model.POI

	// Config for hydration
	catConfig *config.CategoriesConfig
}

// NewManager creates a new POI Manager.
func NewManager(cfg *config.Config, s store.Store, catCfg *config.CategoriesConfig) *Manager {
	return &Manager{
		config:      cfg,
		store:       s,
		logger:      slog.With("component", "poi_manager"),
		trackedPOIs: make(map[string]*model.POI),
		catConfig:   catCfg,
	}
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
	// 0. Pre-check MSFS Overlap if needed
	if err := m.EnrichWithMSFS(ctx, p); err != nil {
		m.logger.Warn("Failed to check MSFS overlap", "qid", p.WikidataID, "error", err)
	}

	m.mu.Lock()

	existing, ok := m.trackedPOIs[p.WikidataID]
	if !ok {
		// Not in active cache, check DB
		dbPOI, err := m.store.GetPOI(ctx, p.WikidataID)
		if err == nil && dbPOI != nil {
			existing = dbPOI
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
			return fmt.Errorf("failed to save POI %s: %w", p.WikidataID, err)
		}
		m.logger.Debug("Upserted POI", "qid", p.WikidataID, "name", p.DisplayName())
	} else {
		m.logger.Debug("Tracked POI (hydrated)", "qid", p.WikidataID, "name", p.DisplayName())
	}

	return nil
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
	return m.store.GetPOI(ctx, qid)
}

// GetTrackedPOIs returns a thread-safe copy of currently tracked POIs.
func (m *Manager) GetTrackedPOIs() []*model.POI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*model.POI, 0, len(m.trackedPOIs))
	for _, p := range m.trackedPOIs {
		// Runtime Hydration: If Icon is missing, try to resolve from config or dimension defaults
		if p.Icon == "" && p.Category != "" {
			// 1. Check Config
			if m.catConfig != nil {
				if cfg, ok := m.catConfig.Categories[strings.ToLower(p.Category)]; ok {
					p.Icon = cfg.Icon
				}
			}
			// 2. Dimension Fallbacks (if still empty)
			if p.Icon == "" {
				switch p.Category {
				case "Area":
					p.Icon = "circle-stroked"
				case "Height":
					p.Icon = "cemetery-JP"
				case "Length":
					p.Icon = "arrow"
				}
			}
		}
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

// GetBestCandidate returns the currently tracked POI with the highest score.
func (m *Manager) GetBestCandidate() *model.POI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *model.POI
	maxScore := -1.0

	for _, p := range m.trackedPOIs {
		if p.Score > maxScore {
			maxScore = p.Score
			best = p
		}
	}
	return best
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
			// 1. Get Telemetry
			telemetry, err := simClient.GetTelemetry(ctx)
			if err != nil {
				m.logger.Warn("Failed to get telemetry for scoring", "error", err)
				continue
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

			input := scorer.ScoringInput{
				Telemetry:       telemetry,
				CategoryHistory: history,
				NarratorConfig:  &m.config.Narrator,
			}

			for _, p := range m.trackedPOIs {
				sc.Calculate(p, &input)
			}
			m.mu.Unlock()
		}
	}
}
