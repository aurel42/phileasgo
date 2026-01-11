package scorer

import (
	"fmt"
	"math"
	"strings"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/terrain"
	"phileasgo/pkg/visibility"
)

// ScoringInput holds usage-time data for scoring.
type ScoringInput struct {
	Telemetry       sim.Telemetry          `json:"telemetry"`
	CategoryHistory []string               `json:"category_history"`
	NarratorConfig  *config.NarratorConfig `json:"narrator_config"`
}

// Session represents a single scoring cycle context.
type Session interface {
	Calculate(poi *model.POI)
}

// Scorer calculates dynamic scores for POIs.
type Scorer struct {
	config    *config.ScorerConfig
	catConfig *config.CategoriesConfig
	visCalc   *visibility.Calculator
	elevation terrain.ElevationGetter
}

// NewScorer creates a new Scorer.
func NewScorer(cfg *config.ScorerConfig, catCfg *config.CategoriesConfig, visCalc *visibility.Calculator, elev terrain.ElevationGetter) *Scorer {
	return &Scorer{
		config:    cfg,
		catConfig: catCfg,
		visCalc:   visCalc,
		elevation: elev,
	}
}

// NewSession initiates a new scoring cycle, pre-calculating expensive terrain data.
func (s *Scorer) NewSession(input *ScoringInput) Session {
	// Pre-calculate lowest elevation in dynamic radius based on XL visibility at MSL
	// This ensures we scan far enough to see big mountains if we are high up.
	radiusNM := s.visCalc.GetMaxVisibleDistance(input.Telemetry.AltitudeMSL, visibility.SizeXL)
	if radiusNM < 10.0 {
		radiusNM = 10.0 // Minimum scan radius safety
	}

	// This is the O(1) op performed once per cycle.
	lowestElev, err := s.elevation.GetLowestElevation(input.Telemetry.Latitude, input.Telemetry.Longitude, radiusNM)
	if err != nil {
		// Log error? For now, fallback to 0 (MSL) or current elevation?
		// Since we cap at 0 in implementation, 0 is safe default.
		lowestElev = 0
	}

	return &DefaultSession{
		scorer:     s,
		input:      input,
		lowestElev: float64(lowestElev),
	}
}

type DefaultSession struct {
	scorer     *Scorer
	input      *ScoringInput
	lowestElev float64
}

// Calculate updates the Score, ScoreDetails, and IsVisible fields of the POI.
func (sess *DefaultSession) Calculate(poi *model.POI) {
	s := sess.scorer
	input := sess.input
	state := input.Telemetry
	predLat := state.PredictedLatitude
	predLon := state.PredictedLongitude
	if predLat == 0 && predLon == 0 {
		predLat = state.Latitude
		predLon = state.Longitude
	}

	poiPoint := geo.Point{Lat: poi.Lat, Lon: poi.Lon}
	predPoint := geo.Point{Lat: predLat, Lon: predLon}
	distMeters := geo.Distance(predPoint, poiPoint)
	distNM := distMeters / 1852.0
	bearing := geo.Bearing(predPoint, poiPoint)

	score := 1.0
	var logs []string
	logs = append(logs, "Base Score: 1.0")

	// 1. Geographic Scoring (Visibility & Ground)
	// Pass lowestElev to geographic score for effective AGL calculation
	geoScore, geoLogs, shouldReturn := s.calculateGeographicScore(poi, &state, bearing, distNM, sess.lowestElev)
	if shouldReturn {
		return
	}
	score *= geoScore
	logs = append(logs, geoLogs...)

	// 2. Content Multipliers
	contentScore, contentLogs := s.calculateContentScore(poi)
	score *= contentScore
	logs = append(logs, contentLogs...)

	// 3. Variety & Novelty
	varietyScore, varietyLogs := s.calculateVarietyScore(poi, input.CategoryHistory)
	score *= varietyScore
	logs = append(logs, varietyLogs...)

	// 4. Repeat Penalty (Specific POI)
	if input.NarratorConfig != nil && !poi.LastPlayed.IsZero() {
		// Use Real Time for "Time Since Played" to avoid SimTime skew issues
		ts := time.Now()
		playedAgo := math.Abs(float64(ts.Sub(poi.LastPlayed).Seconds()))
		ttl := time.Duration(input.NarratorConfig.RepeatTTL).Seconds() // Convert to seconds
		if playedAgo < ttl {
			score = 0
			logs = append(logs, fmt.Sprintf("Repeat Penalty (played %.0fs ago, TTL %.0fs)", playedAgo, ttl))
		}
	}

	poi.Score = score
	poi.ScoreDetails = strings.Join(logs, "\n")
}

// OLD Calculate - kept for compatibility if needed, but should be removed or deprecated.
// For now, removing it to force update in Manager.

func (s *Scorer) calculateGeographicScore(poi *model.POI, state *sim.Telemetry, bearing, distNM, lowestElevMeters float64) (score float64, logs []string, shouldReturn bool) {
	// 1. Determine Size
	poiSize := poi.Size
	if poiSize == "" {
		poiSize = s.catConfig.GetSize(poi.Category)
	}
	if poiSize == "" {
		poiSize = "M"
	}

	// 2. Calculate Effective AGL (Valley Logic)
	// state.AltitudeMSL is in Feet. lowestElevMeters is Meters.
	lowestElevFt := lowestElevMeters * 3.28084
	effectiveAGL := state.AltitudeMSL - lowestElevFt
	// Sanity check: effectiveAGL shouldn't be insanely high if data is bad, but logic handles max dist.

	// 3. Calculate Visibility Score
	// Passes both RealAGL and EffectiveAGL. Calculator returns the better score.
	visScore, visDetails := s.visCalc.CalculatePOIScore(state.Heading, state.AltitudeAGL, effectiveAGL, bearing, distNM, visibility.SizeType(poiSize), state.IsOnGround)

	if visScore <= 0 {
		poi.IsVisible = false
		poi.Score = 0.0
		poi.ScoreDetails = visDetails
		return 0, nil, true
	}

	poi.IsVisible = true
	logs = []string{visDetails}
	score = visScore

	// 4. Apply Size Penalty (reduces advantage of distant large POIs)
	sizePenalty := map[string]float64{"S": 1.0, "M": 1.0, "L": 0.85, "XL": 0.7}
	if penalty, ok := sizePenalty[poiSize]; ok && penalty < 1.0 {
		score *= penalty
		logs = append(logs, fmt.Sprintf("Size Penalty (%s): x%.2f", poiSize, penalty))
	}

	// 5. Apply Dimension Multiplier
	if poi.DimensionMultiplier > 1.0 {
		score *= poi.DimensionMultiplier
		logs = append(logs, fmt.Sprintf("Dimensions: x%.1f", poi.DimensionMultiplier))
	}

	return score, logs, false
}

func (s *Scorer) calculateContentScore(poi *model.POI) (score float64, logs []string) {
	score = 1.0

	// Article Length
	lengthMult := 1.0
	l := float64(poi.WPArticleLength)
	if l > 500 {
		lengthMult = math.Sqrt(l / 500.0)
	}
	if lengthMult > 1.0 {
		score *= lengthMult
		logs = append(logs, fmt.Sprintf("Length (%.0f chars): x%.2f", l, lengthMult))
	}

	// Sitelinks
	sitelinks := poi.Sitelinks

	// Cap for City/Town
	catLower := strings.ToLower(poi.Category)
	if catLower == "city" || catLower == "town" {
		if sitelinks > 4 {
			sitelinks = 4
		}
	}

	slMult := 1.0 + math.Sqrt(math.Max(0, float64(sitelinks-1)))
	if slMult > 1.0 {
		score *= slMult
		logs = append(logs, fmt.Sprintf("Sitelinks (%d): x%.2f", poi.Sitelinks, slMult))
	}

	// Category Weight
	catWeight := s.catConfig.GetWeight(poi.Category)
	if catWeight != 1.0 {
		score *= catWeight
		logs = append(logs, fmt.Sprintf("Category (%s): x%.2f", poi.Category, catWeight))
	}

	// MSFS POI
	if poi.IsMSFSPOI {
		score *= 4.0
		logs = append(logs, "MSFS POI: x4.0")
	}

	return score, logs
}

func (s *Scorer) calculateVarietyScore(poi *model.POI, history []string) (multiplier float64, logs []string) {
	if len(history) == 0 {
		return 1.3, []string{"Novelty Boost (No History): x1.30"}
	}

	foundIdx := -1
	for i := len(history) - 1; i >= 0; i-- {
		if strings.EqualFold(history[i], poi.Category) {
			foundIdx = (len(history) - 1) - i
			break
		}
	}

	if foundIdx != -1 && foundIdx < s.config.VarietyPenaltyNum {
		appliedPenalty := s.config.VarietyPenaltyFirst
		if s.config.VarietyPenaltyNum > 1 {
			fraction := float64(foundIdx) / float64(s.config.VarietyPenaltyNum-1)
			appliedPenalty = s.config.VarietyPenaltyFirst + (s.config.VarietyPenaltyLast-s.config.VarietyPenaltyFirst)*fraction
		}
		return appliedPenalty, []string{fmt.Sprintf("Variety Penalty (Pos %d): x%.2f", foundIdx+1, appliedPenalty)}
	}

	// If not found in history, OR found but outside penalty window
	boost := 1.3
	logs = append(logs, fmt.Sprintf("Novelty Boost: x%.1f", boost))
	multiplier = boost

	// Category Group Check (Last Played)
	if len(history) > 0 {
		lastCat := history[len(history)-1]
		candGroup := s.catConfig.GetGroup(poi.Category)
		lastGroup := s.catConfig.GetGroup(lastCat)

		if candGroup != "" && lastGroup != "" && candGroup == lastGroup {
			groupPenalty := s.config.VarietyPenaltyLast
			multiplier *= groupPenalty
			logs = append(logs, fmt.Sprintf("Group Penalty (%s): x%.2f", candGroup, groupPenalty))
		}
	}

	return multiplier, logs
}
