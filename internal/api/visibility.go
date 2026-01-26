package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/terrain"
	"phileasgo/pkg/visibility"
	"phileasgo/pkg/wikidata"
)

// CoverageProvider interface for fetching tile coverage
type CoverageProvider interface {
	GetGlobalCoverage(ctx context.Context) ([]wikidata.CachedTile, error)
}

// VisibilityHandler handles map visibility requests
type VisibilityHandler struct {
	calculator *visibility.Calculator
	simClient  sim.Client
	elevation  terrain.ElevationGetter
	store      store.Store
	coverage   CoverageProvider
}

// NewVisibilityHandler creates a new handler
func NewVisibilityHandler(calc *visibility.Calculator, sim sim.Client, elev terrain.ElevationGetter, st store.Store, cov CoverageProvider) *VisibilityHandler {
	return &VisibilityHandler{
		calculator: calc,
		simClient:  sim,
		elevation:  elev,
		store:      st,
		coverage:   cov,
	}
}

// HandleGetCoverage handles GET /api/map/coverage
func (h *VisibilityHandler) HandleGetCoverage(w http.ResponseWriter, r *http.Request) {
	if h.coverage == nil {
		http.Error(w, "Coverage provider not configured", http.StatusNotImplemented)
		return
	}

	tiles, err := h.coverage.GetGlobalCoverage(r.Context())
	if err != nil {
		http.Error(w, "Failed to get coverage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tiles); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Handler handles GET /api/map/visibility
func (h *VisibilityHandler) Handler(w http.ResponseWriter, r *http.Request) {
	// 1. Get Aircraft State
	telemetry, err := h.simClient.GetTelemetry(r.Context())
	if err != nil {
		if err == sim.ErrWaitingForTelemetry {
			http.Error(w, "Waiting for telemetry", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Sim not connected", http.StatusServiceUnavailable)
		return
	}

	// 2. Parse Query Params
	params, err := h.parseQueryParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Calculate Effective AGL (Valley)
	effectiveAGL := h.calculateEffectiveAGL(r.Context(), &telemetry)

	// 3a. Get Visibility Boost
	boostFactor := h.getBoostFactor(r.Context())

	// 4. Generate Grid
	gridM, gridL, gridXL := h.computeGrids(&telemetry, effectiveAGL, params.North, params.East, params.South, params.West, params.Resolution, boostFactor)

	// 5. Response
	resp := map[string]interface{}{
		"gridM":  gridM,
		"gridL":  gridL,
		"gridXL": gridXL,
		"rows":   params.Resolution,
		"cols":   params.Resolution,
		"bounds": map[string]float64{
			"north": params.North, "east": params.East, "south": params.South, "west": params.West,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type VisibilityParams struct {
	North, East, South, West float64
	Resolution               int
}

func (h *VisibilityHandler) parseQueryParams(r *http.Request) (VisibilityParams, error) {
	boundsStr := r.URL.Query().Get("bounds")
	resolutionStr := r.URL.Query().Get("resolution")

	if boundsStr == "" {
		return VisibilityParams{}, strconv.ErrSyntax // Or simplified error
	}

	parts := strings.Split(boundsStr, ",")
	if len(parts) != 4 {
		return VisibilityParams{}, strconv.ErrSyntax
	}

	n, _ := strconv.ParseFloat(parts[0], 64)
	e, _ := strconv.ParseFloat(parts[1], 64)
	s, _ := strconv.ParseFloat(parts[2], 64)
	w, _ := strconv.ParseFloat(parts[3], 64)

	res := 20 // Default
	if resolutionStr != "" {
		if v, err := strconv.Atoi(resolutionStr); err == nil && v > 0 && v <= 100 {
			res = v
		}
	}
	return VisibilityParams{
		North:      n,
		East:       e,
		South:      s,
		West:       w,
		Resolution: res,
	}, nil
}

func (h *VisibilityHandler) getBoostFactor(ctx context.Context) float64 {
	boostFactor := 1.0
	if h.store != nil {
		val, ok := h.store.GetState(ctx, "visibility_boost")
		if ok && val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				boostFactor = f
			}
		}
	}
	return boostFactor
}

func (h *VisibilityHandler) calculateEffectiveAGL(ctx context.Context, telemetry *sim.Telemetry) float64 {
	effectiveAGL := telemetry.AltitudeAGL

	if h.elevation != nil {
		// Quick scan similar to Scorer Session (dynamic radius)
		boostFactor := h.getBoostFactor(ctx)
		radiusNM := h.calculator.GetMaxVisibleDistance(telemetry.AltitudeMSL, visibility.SizeXL, boostFactor)
		if radiusNM < 10.0 {
			radiusNM = 10.0
		}
		lowestMeters, err := h.elevation.GetLowestElevation(telemetry.Latitude, telemetry.Longitude, radiusNM)
		if err == nil {
			lowestFeet := float64(lowestMeters) * 3.28084
			effectiveAGL = telemetry.AltitudeMSL - lowestFeet
		}
	}
	return effectiveAGL
}

func (h *VisibilityHandler) computeGrids(telemetry *sim.Telemetry, effectiveAGL, north, east, south, west float64, res int, boostFactor float64) (gridM, gridL, gridXL []float64) {
	latStep := (north - south) / float64(res)

	// Ensure positive steps
	if latStep < 0 {
		latStep = -latStep
	}

	// Handle dateline crossing
	width := east - west
	if width < 0 {
		width += 360
	}
	lonStep := width / float64(res)

	gridM = make([]float64, 0, res*res)
	gridL = make([]float64, 0, res*res)
	gridXL = make([]float64, 0, res*res)

	// Rows: North -> South
	for row := 0; row < res; row++ {
		// Center of the cell
		lat := north - (float64(row)+0.5)*latStep

		// Cols: West -> East
		for col := 0; col < res; col++ {
			lon := west + (float64(col)+0.5)*lonStep
			// Normalize lon
			if lon > 180 {
				lon -= 360
			}
			if lon < -180 {
				lon += 360
			}

			// Calculate Geometry
			dist, bearing := calculateGeom(telemetry.Latitude, telemetry.Longitude, lat, lon)

			// Calculate Score for each size
			scoreM := h.calculator.CalculateVisibilityForSize(
				telemetry.Heading,
				telemetry.AltitudeAGL,
				effectiveAGL,
				bearing,
				dist,
				visibility.SizeM,
				telemetry.IsOnGround,
				boostFactor,
			)
			scoreL := h.calculator.CalculateVisibilityForSize(
				telemetry.Heading,
				telemetry.AltitudeAGL,
				effectiveAGL,
				bearing,
				dist,
				visibility.SizeL,
				telemetry.IsOnGround,
				boostFactor,
			)
			scoreXL := h.calculator.CalculateVisibilityForSize(
				telemetry.Heading,
				telemetry.AltitudeAGL,
				effectiveAGL,
				bearing,
				dist,
				visibility.SizeXL,
				telemetry.IsOnGround,
				boostFactor,
			)

			gridM = append(gridM, scoreM)
			gridL = append(gridL, scoreL)
			gridXL = append(gridXL, scoreXL)
		}
	}
	return gridM, gridL, gridXL
}

// Simple Haversine/Bearing helper
func calculateGeom(lat1, lon1, lat2, lon2 float64) (distNM, bearing float64) {
	// ... implementation ...
	// Quick implementation for the tool
	const R = 3440.065 // nm

	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distNM = R * c

	y := math.Sin(dLon) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) -
		math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(dLon)
	brng := math.Atan2(y, x) * 180 / math.Pi

	return distNM, brng
}
