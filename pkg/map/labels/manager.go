package labels

import (
	"math"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/poi"
	"sort"
	"strings"
)

// LabelCandidate represents a city that is a potential label on the map.
type LabelCandidate struct {
	City       geo.City
	GenericID  string // e.g. "lat-lon" or Name
	Category   string // e.g. "city", "town", "village"
	FinalScore float64
	Importance float64
	Direction  float64
}

// Manager coordinates the selection of map labels.
type Manager struct {
	geoSvc *geo.Service
	poiSvc *poi.Manager
	scorer *Scorer
}

// NewManager creates a new Label Manager.
func NewManager(g *geo.Service, p *poi.Manager) *Manager {
	return &Manager{
		geoSvc: g,
		poiSvc: p,
		scorer: NewScorer(),
	}
}

// SelectLabels picks the best labels for a given viewport and aircraft state.
// It respects the Minimum Separation Radius (MSR) to avoid clutter.
func (m *Manager) SelectLabels(
	minLat, minLon, maxLat, maxLon float64, // Viewport BBox
	acLat, acLon, heading float64, // Aircraft State
	existingLabels []geo.Point, // Locations of labels we MUST respect (already on screen)
) []*LabelCandidate {

	// 1. Get Candidates
	var scored []LabelCandidate
	scored = append(scored, m.collectGlobalCandidates(minLat, minLon, maxLat, maxLon, acLat, acLon, heading)...)
	scored = append(scored, m.collectLocalCandidates(minLat, minLon, maxLat, maxLon, acLat, acLon, heading)...)

	// 2. Sort by Score Descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].FinalScore > scored[j].FinalScore
	})

	// 3. MSR Selection (Greedy)
	msrDegSq := math.Pow((maxLon-minLon)*MinSeparationRatio, 2)
	var selected []*LabelCandidate

	for i := range scored {
		cand := &scored[i]
		if len(selected) >= TargetLabelCount {
			break
		}

		if m.isValid(cand, selected, existingLabels, msrDegSq) {
			selected = append(selected, cand)
		}
	}

	return selected
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

func (m *Manager) isValid(cand *LabelCandidate, selected []*LabelCandidate, existingLabels []geo.Point, msrDegSq float64) bool {
	// Check collision with accepted
	for _, s := range selected {
		distSq := (cand.City.Lat-s.City.Lat)*(cand.City.Lat-s.City.Lat) +
			(cand.City.Lon-s.City.Lon)*(cand.City.Lon-s.City.Lon)
		if distSq < msrDegSq {
			return false
		}
	}

	// Check collision with existing (locked) labels
	for _, ex := range existingLabels {
		distSq := (cand.City.Lat-ex.Lat)*(cand.City.Lat-ex.Lat) +
			(cand.City.Lon-ex.Lon)*(cand.City.Lon-ex.Lon)
		if distSq < msrDegSq {
			return false
		}
	}
	return true
}

// TODO: integrating this into geo package properly would be better, but for now we access the grid if exported or
// we add a method to geo. Since geo.grid is private, we MUST add a method to geo.go.
// For compilation to work now, I'll add a placeholder that returns empty or uses a public accessor.
// WAIT: geo.grid IS private. I must modify geo.go to export a Range Query method.
func (m *Manager) getCitiesInBbox(minLat, minLon, maxLat, maxLon float64) []geo.City {
	// Wrapper to call the new geo service method we are about to add.
	return m.geoSvc.GetCitiesInBbox(minLat, minLon, maxLat, maxLon)
}
