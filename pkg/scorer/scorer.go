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
	BoostFactor     float64                `json:"boost_factor"` // Multiplier for visibility range (1.0 - 1.5)
}

// Session represents a single scoring cycle context.
type Session interface {
	Calculate(poi *model.POI)
	LowestElevation() float64
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
	boost := input.BoostFactor
	if boost < 1.0 {
		boost = 1.0
	}
	radiusNM := s.visCalc.GetMaxVisibleDistance(input.Telemetry.AltitudeMSL, visibility.SizeXL, boost)
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

	// Pre-calculate future positions for deferral logic: +1, +3, +5, +10, +15 minutes
	var futurePositions []geo.Point
	if s.config.DeferralEnabled && input.Telemetry.GroundSpeed > 10 {
		horizons := []float64{1, 3, 5, 10, 15} // minutes
		futurePositions = make([]geo.Point, len(horizons))
		tel := input.Telemetry
		speedMetersPerMin := (tel.GroundSpeed * 1852.0) / 60.0 // knots → m/min

		for i, mins := range horizons {
			distMeters := speedMetersPerMin * mins
			futurePositions[i] = geo.DestinationPoint(
				geo.Point{Lat: tel.Latitude, Lon: tel.Longitude},
				distMeters,
				tel.Heading,
			)
		}
	}

	return &DefaultSession{
		scorer:          s,
		input:           input,
		lowestElev:      float64(lowestElev),
		futurePositions: futurePositions,
	}
}

type DefaultSession struct {
	scorer          *Scorer
	input           *ScoringInput
	lowestElev      float64
	futurePositions []geo.Point // Pre-calculated positions at +1, +3, +5, +10, +15 min
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

	sess.applyBadges(poi)

	// [NEW] Skip logic for recently played POIs (on cooldown)
	if !poi.LastPlayed.IsZero() && input.NarratorConfig != nil {
		if time.Since(poi.LastPlayed) < time.Duration(input.NarratorConfig.RepeatTTL) {
			// Marker is on cooldown (Blue marker).
			// We skip all "Narrator" badges and scoring logic to avoid confusing UI.
			return
		}
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
	// Ensure BoostFactor is at least 1.0
	boost := input.BoostFactor
	if boost < 1.0 {
		boost = 1.0
	}
	geoScore, geoLogs, shouldReturn := s.calculateGeographicScore(poi, &state, bearing, distNM, sess.lowestElev, boost)
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

	// [BADGE] Fresh (Novelty)
	// If variety score > 1.0, it means we got a boost (novelty), so mark it fresh.
	// Note: We check varietyScore (which returns multiplier) directly.
	if varietyScore > 1.0 {
		poi.Badges = append(poi.Badges, "fresh")
	}

	// 4. Deferral Check: Will we be 25%+ closer in the future?
	if len(sess.futurePositions) > 0 {
		deferralPenalty, deferralMsg, isDeferred := sess.checkDeferral(poi, state.Heading)
		if deferralPenalty < 1.0 {
			score *= deferralPenalty
			logs = append(logs, deferralMsg)
		}
		if isDeferred {
			poi.Badges = append(poi.Badges, "deferred")
		}
	}

	poi.Score = score
	poi.ScoreDetails = strings.Join(logs, "\n")
}

// checkDeferral checks if we'll be significantly closer to the POI in the future.
// Returns multiplier (1.0 if no deferral, 0.1 if deferred), a concise log message, and a boolean indicating state.
func (sess *DefaultSession) checkDeferral(poi *model.POI, heading float64) (multiplier float64, msg string, isDeferred bool) {
	poiPoint := geo.Point{Lat: poi.Lat, Lon: poi.Lon}
	// Horizons: 0=1m, 1=3m, 2=5m, 3=10m, 4=15m
	if len(sess.futurePositions) != 5 {
		return 1.0, "", false
	}

	bestClose, bestFar := sess.getDeferralDistances(poiPoint, heading)

	// If all "Close" positions are behind or invalid, we can't compare.
	if math.IsInf(bestClose, 1) {
		return 1.0, "", false
	}

	// If all "Far" positions are behind or invalid, we are moving away/past it.
	if math.IsInf(bestFar, 1) {
		return 1.0, "", false
	}

	// Defer if bestFar is significantly closer than bestClose
	threshold := sess.scorer.config.DeferralThreshold
	if threshold <= 0 {
		threshold = 0.75 // Default: defer if 25%+ closer
	}

	// 1. Calculate Urgency Metrics (TTB, TTCPA)
	sess.calculateUrgencyMetrics(poi, poiPoint, heading)

	// 2. Apply Penalties/Boosts
	urgencyMultiplier, urgencyMsg := sess.calculateUrgencyFactor(poi)

	// 3. Absolute Deferral (Current Logic)
	if bestFar < threshold*bestClose {
		multiplier := sess.scorer.config.DeferralMultiplier
		if multiplier <= 0 {
			multiplier = 0.1
		}
		return multiplier, fmt.Sprintf("Defer: x%.1f (%.1fnm -> %.1fnm)", multiplier, bestClose, bestFar), true
	}

	return urgencyMultiplier, urgencyMsg, false
}

// calculateUrgencyFactor applies urgency boosts and patience penalties.
func (sess *DefaultSession) calculateUrgencyFactor(poi *model.POI) (multiplier float64, msg string) {
	urgencyMultiplier := 1.0

	// Urgency Boost: If disappearing in < 2 minutes
	if poi.TimeToBehind > 0 && poi.TimeToBehind < 120 {
		urgencyMultiplier *= 1.5
		msg = "Urgency Boost: x1.5 (Disappearing soon)"
		poi.Badges = append(poi.Badges, "urgent")
	}

	// Patience Penalty: If CPA is > 10 minutes away and not disappearing soon
	if poi.TimeToCPA > 600 && (poi.TimeToBehind == -1 || poi.TimeToBehind > 900) {
		urgencyMultiplier *= 0.5
		msg = "Patience Penalty: x0.5 (Best view far away)"
		poi.Badges = append(poi.Badges, "patient")
	}

	return urgencyMultiplier, msg
}

// getDeferralDistances calculates min distances for close (1-3m) and far (5-15m) buckets.
func (sess *DefaultSession) getDeferralDistances(poiPoint geo.Point, heading float64) (bestClose, bestFar float64) {
	// Group 1: Close (1m, 3m) -> indices 0, 1
	bestClose = sess.minDistInRange(poiPoint, heading, []int{0, 1})
	// Group 2: Far (5m, 10m, 15m) -> indices 2, 3, 4
	bestFar = sess.minDistInRange(poiPoint, heading, []int{2, 3, 4})
	return
}

// minDistInRange finds the minimum valid (forward-facing) distance in nautical miles.
func (sess *DefaultSession) minDistInRange(poiPoint geo.Point, heading float64, indices []int) float64 {
	minD := math.Inf(1)
	for _, idx := range indices {
		if idx >= len(sess.futurePositions) {
			continue
		}
		pos := sess.futurePositions[idx]
		bearingToPOI := geo.Bearing(pos, poiPoint)
		if math.Abs(geo.NormalizeAngle(bearingToPOI-heading)) > 90 {
			continue
		}
		d := geo.Distance(pos, poiPoint) / 1852.0
		if d < minD {
			minD = d
		}
	}
	return minD
}

// calculateUrgencyMetrics calculates TimeToBehind (TTB) and TimeToCPA (TTCPA).
func (sess *DefaultSession) calculateUrgencyMetrics(poi *model.POI, poiPoint geo.Point, heading float64) {
	poi.TimeToBehind = -1
	poi.TimeToCPA = -1

	horizons := []float64{1, 3, 5, 10, 15}
	minD := math.Inf(1)
	bestIdx := -1

	for i, pos := range sess.futurePositions {
		bearingToPOI := geo.Bearing(pos, poiPoint)
		if math.Abs(geo.NormalizeAngle(bearingToPOI-heading)) > 90 {
			if poi.TimeToBehind == -1 {
				poi.TimeToBehind = horizons[i] * 60
			}
			continue
		}

		d := geo.Distance(pos, poiPoint)
		if d < minD {
			minD = d
			bestIdx = i
		}
	}

	if bestIdx != -1 {
		poi.TimeToCPA = horizons[bestIdx] * 60
	}
}

// LowestElevation returns the calculated lowest elevation (valley floor) in meters for this session.
func (sess *DefaultSession) LowestElevation() float64 {
	return sess.lowestElev
}

func (s *Scorer) calculateGeographicScore(poi *model.POI, state *sim.Telemetry, bearing, distNM, lowestElevMeters, boostFactor float64) (score float64, logs []string, shouldReturn bool) {
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
	// POLICY: Restrict Dynamic Visibility Boost to XL POIs only.
	// We don't want to drastically expand the search radius for small items effectively invisible at distance.
	appliedBoost := 1.0
	if poiSize == "XL" {
		appliedBoost = boostFactor
	}

	visScore, visDetails := s.visCalc.CalculatePOIScore(state.Heading, state.AltitudeAGL, effectiveAGL, bearing, distNM, visibility.SizeType(poiSize), state.IsOnGround, appliedBoost)

	if visScore <= 0 {
		poi.IsVisible = false
		poi.Score = 0.0
		poi.ScoreDetails = visDetails
		return 0, nil, true
	}

	poi.IsVisible = true
	poi.Visibility = visScore
	logs = []string{visDetails}
	if appliedBoost > 1.0 {
		logs = append(logs, fmt.Sprintf("Visibility Boost: x%.1f", appliedBoost))
	}
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
		return s.config.NoveltyBoost, []string{fmt.Sprintf("Novelty Boost (No History): x%.2f", s.config.NoveltyBoost)}
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
	boost := s.config.NoveltyBoost
	logs = append(logs, fmt.Sprintf("Novelty Boost: x%.2f", boost))
	multiplier = boost

	// Category Group Check (Last Played)
	if len(history) > 0 {
		lastCat := history[len(history)-1]
		candGroup := s.catConfig.GetGroup(poi.Category)
		lastGroup := s.catConfig.GetGroup(lastCat)

		if candGroup != "" && lastGroup != "" && candGroup == lastGroup {
			groupPenalty := s.config.GroupPenalty
			multiplier *= groupPenalty
			logs = append(logs, fmt.Sprintf("Group Penalty (%s): x%.2f", candGroup, groupPenalty))
		}
	}

	return multiplier, logs
}

// applyBadges handles the stateless logic for assigning badges based on POI properties.
func (sess *DefaultSession) applyBadges(poi *model.POI) {
	s := sess.scorer

	// Reset Ephemeral Badges
	poi.Badges = make([]string, 0)
	if poi.IsMSFSPOI {
		poi.Badges = append(poi.Badges, "msfs")
	}

	// [BADGE] Deep Dive (Stateless, keep on blue markers)
	limit := s.config.Badges.DeepDive.ArticleLenMin
	if limit == 0 {
		limit = 20000 // Safe default
	}
	if poi.WPArticleLength > limit {
		poi.Badges = append(poi.Badges, "deep_dive")
	}

	// [BADGE] Stub (Stateless, mutually exclusive with Deep Dive ideally, but logic handles it)
	stubLimit := s.config.Badges.Stub.ArticleLenMax
	if stubLimit == 0 {
		stubLimit = 2000 // Safe default
	}
	if poi.WPArticleLength > 0 && poi.WPArticleLength < stubLimit {
		poi.Badges = append(poi.Badges, "stub")
	}
}
