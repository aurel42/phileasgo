package labels

import (
	"context"
	"log/slog"
	"math"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/poi"
	"sort"
	"strings"
	"sync"
)

// LabelLimitProvider is consumed by the Manager to read the user's settlement density and tier settings.
type LabelLimitProvider interface {
	SettlementLabelLimit(ctx context.Context) int
	SettlementTier(ctx context.Context) int
}

// LabelCandidate represents a city that is a potential label on the map.
type LabelCandidate struct {
	City       geo.City
	GenericID  string // e.g. "lat-lon" or Name
	Category   string // e.g. "city", "town", "village"
	FinalScore float64
	Importance float64
	Direction  float64
	IsShadow   bool // True if this item is in the "Deep Inventory" outside the viewport
	Fading     bool // Near/past viewport edge; tracked but not counted or returned
}

// bbox holds axis-aligned bounding box coordinates.
type bbox struct {
	minLat, minLon, maxLat, maxLon float64
}

func (b bbox) contains(lat, lon float64) bool {
	return lat >= b.minLat && lat <= b.maxLat && lon >= b.minLon && lon <= b.maxLon
}

// Manager coordinates the selection of map labels.
// It is stateful: activeSettlements persists across calls to provide shadow blocking
// and stable label sets during panning.
type Manager struct {
	geoSvc  *geo.Service
	poiSvc  *poi.Manager
	scorer  *Scorer
	cfgProv LabelLimitProvider

	mu                sync.Mutex
	activeSettlements map[string]*LabelCandidate
	currentZoomFloor  int
	lastLimit         int
}

// NewManager creates a new Label Manager.
func NewManager(g *geo.Service, p *poi.Manager, cfgProv LabelLimitProvider) *Manager {
	return &Manager{
		geoSvc:            g,
		poiSvc:            p,
		scorer:            NewScorer(),
		cfgProv:           cfgProv,
		activeSettlements: make(map[string]*LabelCandidate),
		currentZoomFloor:  -1,
		lastLimit:         -1,
	}
}

// expandBbox calculates an expanded bounding box for shadow discovery (~30% along heading).
func expandBbox(vp bbox, heading float64) bbox {
	latSpan := vp.maxLat - vp.minLat
	lonSpan := vp.maxLon - vp.minLon
	hdgRad := heading * math.Pi / 180
	expandLat := latSpan * 0.3 * math.Cos(hdgRad)
	expandLon := lonSpan * 0.3 * math.Sin(hdgRad)

	return bbox{
		minLat: math.Min(vp.minLat, vp.minLat+expandLat),
		maxLat: math.Max(vp.maxLat, vp.maxLat+expandLat),
		minLon: math.Min(vp.minLon, vp.minLon+expandLon),
		maxLon: math.Max(vp.maxLon, vp.maxLon+expandLon),
	}
}

// pruneActive removes all shadows and normal settlements that drifted outside the expanded bbox.
func (m *Manager) pruneActive(exp bbox, bufferLat, bufferLon float64) {
	for id, s := range m.activeSettlements {
		if s.IsShadow {
			delete(m.activeSettlements, id)
			continue
		}
		lat, lon := s.City.Lat, s.City.Lon
		if lat < exp.minLat-bufferLat || lat > exp.maxLat+bufferLat ||
			lon < exp.minLon-bufferLon || lon > exp.maxLon+bufferLon {
			delete(m.activeSettlements, id)
		}
	}
}

// getLimit reads the settlement density limit from config (or falls back to TargetLabelCount).
func (m *Manager) getLimit() int {
	if m.cfgProv != nil {
		if n := m.cfgProv.SettlementLabelLimit(context.Background()); n >= 0 {
			return n
		}
	}
	return TargetLabelCount
}

// getTier reads the settlement tier from config (default 3 = all).
func (m *Manager) getTier() int {
	if m.cfgProv != nil {
		return m.cfgProv.SettlementTier(context.Background())
	}
	return 3
}

// filterByTier removes candidates that don't meet the tier threshold.
// Tier 0 = none, 1 = city only (pop >= PopCity), 2 = city+town (pop >= PopTown), 3 = all.
func filterByTier(candidates []LabelCandidate, tier int) []LabelCandidate {
	if tier >= 3 {
		return candidates
	}
	if tier <= 0 {
		return nil
	}
	minPop := PopTown // tier 2
	if tier == 1 {
		minPop = PopCity
	}
	filtered := candidates[:0:0]
	for i := range candidates {
		if candidates[i].City.Population >= minPop {
			filtered = append(filtered, candidates[i])
		}
	}
	return filtered
}

// normalCount returns the number of active, non-shadow, non-fading settlements.
func (m *Manager) normalCount() int {
	n := 0
	for _, s := range m.activeSettlements {
		if !s.IsShadow && !s.Fading {
			n++
		}
	}
	return n
}

// visibleSettlements returns active settlements that are neither shadows nor fading.
func (m *Manager) visibleSettlements() []*LabelCandidate {
	var result []*LabelCandidate
	for _, s := range m.activeSettlements {
		if !s.IsShadow && !s.Fading {
			result = append(result, s)
		}
	}
	return result
}

// SelectLabels picks the best labels for a given viewport and aircraft state.
// It maintains state across calls: active settlements persist and shadow items
// block future candidates from appearing in their exclusion zone.
func (m *Manager) SelectLabels(
	minLat, minLon, maxLat, maxLon float64, // Viewport BBox
	acLat, acLon, heading float64, // Aircraft State
	existingLabels []geo.Point, // Locations of labels we MUST respect (already on screen)
	zoom float64,
) []*LabelCandidate {
	m.mu.Lock()
	defer m.mu.Unlock()

	vp := bbox{minLat, minLon, maxLat, maxLon}
	latSpan := maxLat - minLat
	lonSpan := maxLon - minLon

	activeBeforePrune := len(m.activeSettlements)
	_ = activeBeforePrune

	// 1. Zoom-level change: clear all state
	zoomFloor := int(math.Floor(zoom))
	if zoomFloor != m.currentZoomFloor {
		slog.Debug("[labels] zoom change, clearing state", "old", m.currentZoomFloor, "new", zoomFloor)
		m.activeSettlements = make(map[string]*LabelCandidate)
		m.currentZoomFloor = zoomFloor
	}

	// 1b. Limit change: clear all state
	limit := m.getLimit()
	if limit != m.lastLimit {
		slog.Debug("[labels] limit change, clearing state", "old", m.lastLimit, "new", limit)
		m.activeSettlements = make(map[string]*LabelCandidate)
		m.lastLimit = limit
	}

	// 2. Expanded bbox and pruning
	exp := expandBbox(vp, heading)
	m.pruneActive(exp, latSpan*0.2, lonSpan*0.2)
	activeAfterPrune := len(m.activeSettlements)
	_ = activeAfterPrune

	// 3. Get candidates from the expanded bbox
	var scored []LabelCandidate
	globalCands := m.collectGlobalCandidates(exp.minLat, exp.minLon, exp.maxLat, exp.maxLon, acLat, acLon, heading)
	localCands := m.collectLocalCandidates(exp.minLat, exp.minLon, exp.maxLat, exp.maxLon, acLat, acLon, heading)
	scored = append(scored, globalCands...)
	scored = append(scored, localCands...)

	// 3b. Filter by settlement tier
	tier := m.getTier()
	beforeTier := len(scored)
	_ = beforeTier
	scored = filterByTier(scored, tier)

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].FinalScore > scored[j].FinalScore
	})

	// 4. Greedy selection
	shortSpan := math.Min(latSpan, lonSpan)
	msrRatio := MinSeparationRatio // 0.3 for ≤6 labels
	if limit < 0 {
		msrRatio = 0.10 // "infinite" mode
	} else if limit > 6 {
		msrRatio = math.Max(0.10, MinSeparationRatio-float64(limit-6)*0.01)
	}
	msrDegSq := math.Pow(shortSpan*msrRatio, 2)

	// NEW: Track which settlements were "Inner" (visible and not fading/shadow) before the greedy selection
	// to implement directional/stateful fading.
	wasInner := make(map[string]bool)
	for id, s := range m.activeSettlements {
		if !s.IsShadow && !s.Fading {
			wasInner[id] = true
		}
	}

	_ = m.greedySelect(scored, vp, existingLabels, limit, msrDegSq)

	// 5. Mark fading: settlements leaving the viewport move to Margin and free up slots.
	// To prevent newly entering labels from being dropped, we only toggle Fading if the label
	// was previously in the "Inner" area.
	insetVp := bbox{
		minLat: vp.minLat + latSpan*0.05,
		minLon: vp.minLon + lonSpan*0.05,
		maxLat: vp.maxLat - latSpan*0.05,
		maxLon: vp.maxLon - lonSpan*0.05,
	}
	fadingCount := 0
	for _, s := range m.activeSettlements {
		if !s.IsShadow {
			inMargin := !insetVp.contains(s.City.Lat, s.City.Lon)
			// Stateful Fade: Only fade if it was "Stable" and moved to Margin, or if it was already Fading.
			// Labels entering from Shadow (IsEntering) remain non-fading even if they hit the margin first.
			s.Fading = inMargin && (s.Fading || wasInner[s.GenericID])

			if s.Fading {
				fadingCount++
			}
		}
	}

	visible := m.visibleSettlements()

	return visible
}

func (m *Manager) collectGlobalCandidates(minLat, minLon, maxLat, maxLon, acLat, acLon, heading float64) []LabelCandidate {
	globalCandidates := m.getCitiesInBbox(minLat, minLon, maxLat, maxLon)
	var scored []LabelCandidate
	for _, city := range globalCandidates {
		cat := "village"
		if city.Population >= PopCity {
			cat = "city"
		} else if city.Population >= PopTown {
			cat = "town"
		}

		scored = append(scored, LabelCandidate{
			City:       city,
			GenericID:  city.Name,
			Category:   cat,
			FinalScore: m.scorer.CalculateFinalScore(&city, acLat, acLon, heading),
			Importance: m.scorer.CalculateImportance(city.Population, city.Name),
			Direction:  m.scorer.CalculateDirectionalWeight(city.Lat, city.Lon, acLat, acLon, heading),
		})
	}
	return scored
}

func (m *Manager) collectLocalCandidates(minLat, minLon, maxLat, maxLon, acLat, acLon, heading float64) []LabelCandidate {
	var scored []LabelCandidate
	if m.poiSvc == nil {
		return scored
	}

	localPOIs := m.poiSvc.GetTrackedPOIs()
	for _, p := range localPOIs {
		if p.Lat < minLat || p.Lat > maxLat || p.Lon < minLon || p.Lon > maxLon {
			continue
		}

		cat := strings.ToLower(p.Category)
		if cat != "city" && cat != "town" && cat != "village" {
			continue
		}

		pop := PopVillage
		if cat == "city" {
			pop = PopCity
		} else if cat == "town" {
			pop = PopTown
		}

		city := geo.City{
			Name:       p.DisplayName(),
			Lat:        p.Lat,
			Lon:        p.Lon,
			Population: pop,
		}

		scored = append(scored, LabelCandidate{
			City:       city,
			GenericID:  p.WikidataID,
			Category:   cat,
			FinalScore: m.scorer.CalculateFinalScore(&city, acLat, acLon, heading),
			Importance: m.scorer.CalculateImportance(city.Population, city.Name),
			Direction:  m.scorer.CalculateDirectionalWeight(city.Lat, city.Lon, acLat, acLon, heading),
		})
	}
	return scored
}

func (m *Manager) calculateMsrX(name string, msrDeg float64) float64 {
	// We want a subtle stretch to prevent thin vertical stacking.
	// 0.8 base + 0.04 per char.
	// "Paris" (5)    -> 1.0x (circular)
	// "London" (6)   -> 1.04x
	// "CenterCity" (10) -> 1.2x
	// "Sankt Margarethen" (17) -> 1.48x
	return msrDeg * (0.8 + float64(len(name))*0.04)
}

func (m *Manager) isValid(cand *LabelCandidate, selected []*LabelCandidate, existingLabels []geo.Point, msrDegSq float64) bool {
	msrY := math.Sqrt(msrDegSq)
	horizCand := m.calculateMsrX(cand.City.Name, msrY)

	for _, s := range selected {
		horizS := m.calculateMsrX(s.City.Name, msrY)
		avgMsrX := (horizCand + horizS) / 2.0

		dx := cand.City.Lon - s.City.Lon
		dy := cand.City.Lat - s.City.Lat

		// Elliptical distance check: (dx/rx)^2 + (dy/ry)^2 < 1
		if math.Pow(dx/avgMsrX, 2)+math.Pow(dy/msrY, 2) < 1.0 {
			return false
		}
	}

	for _, ex := range existingLabels {
		// For existing labels where we don't have the name, assume a medium-length default factor
		// Or if we have the name (we don't in geo.Point), we would use it.
		// Let's use 1.5x as a safe default for horizontal buffer of existing labels.
		avgMsrX := (horizCand + msrY*1.5) / 2.0

		dx := cand.City.Lon - ex.Lon
		dy := cand.City.Lat - ex.Lat

		if math.Pow(dx/avgMsrX, 2)+math.Pow(dy/msrY, 2) < 1.0 {
			return false
		}
	}
	return true
}

type selectStats struct {
	existing, msr, limit, visible, shadow int
}

// greedySelect runs the greedy label selection loop, adding candidates to activeSettlements.
func (m *Manager) greedySelect(scored []LabelCandidate, vp bbox, existingLabels []geo.Point, limit int, msrDegSq float64) selectStats {
	activeSlice := make([]*LabelCandidate, 0, len(m.activeSettlements))
	for _, s := range m.activeSettlements {
		activeSlice = append(activeSlice, s)
	}

	var s selectStats
	for i := range scored {
		cand := &scored[i]
		if _, exists := m.activeSettlements[cand.GenericID]; exists {
			s.existing++
			continue
		}
		if !m.isValid(cand, activeSlice, existingLabels, msrDegSq) {
			s.msr++

			continue
		}

		if vp.contains(cand.City.Lat, cand.City.Lon) {
			nc := m.normalCount()
			if limit >= 0 && nc >= limit {
				s.limit++

				continue
			}
			cand.IsShadow = false
			s.visible++
		} else {
			cand.IsShadow = true
			s.shadow++
		}

		stored := *cand
		m.activeSettlements[cand.GenericID] = &stored
		activeSlice = append(activeSlice, &stored)
	}
	return s
}

func (m *Manager) getCitiesInBbox(minLat, minLon, maxLat, maxLon float64) []geo.City {
	return m.geoSvc.GetCitiesInBbox(minLat, minLon, maxLat, maxLon)
}
