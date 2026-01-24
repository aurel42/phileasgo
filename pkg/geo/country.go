package geo

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

//go:embed countries.geojson
var countriesGeoJSON []byte

// Maritime zone distance thresholds in meters
const (
	TerritorialWatersNM = 12   // 12 nautical miles
	EEZNM               = 200  // 200 nautical miles
	NMToMeters          = 1852 // 1 nautical mile = 1852 meters

	TerritorialWatersM = TerritorialWatersNM * NMToMeters // 22,224 meters
	EEZM               = EEZNM * NMToMeters               // 370,400 meters
)

// Zone constants
const (
	ZoneLand          = "land"
	ZoneTerritorial   = "territorial"
	ZoneEEZ           = "eez"
	ZoneInternational = "international"
)

// CountryResult represents the result of a country lookup.
type CountryResult struct {
	CountryCode string  // ISO 3166-1 Alpha-2 (e.g., "RU")
	CountryName string  // Full name (e.g., "Russia")
	Zone        string  // "land", "territorial", "eez", "international"
	DistanceM   float64 // Distance to nearest coast in meters (0 if on land)
}

// CountryService provides country boundary detection using GeoJSON polygons.
type CountryService struct {
	features *geojson.FeatureCollection

	// Cache for expensive lookups
	mu         sync.RWMutex
	lastResult CountryResult
	lastLat    float64
	lastLon    float64
	lastTime   time.Time
	cacheTTL   time.Duration
}

// NewCountryServiceEmbedded creates a CountryService using embedded GeoJSON data.
// This is the preferred constructor as it doesn't require external files.
func NewCountryServiceEmbedded() (*CountryService, error) {
	return newCountryServiceFromData(countriesGeoJSON)
}

// NewCountryService loads country boundaries from a GeoJSON file.
// Prefer NewCountryServiceEmbedded() unless you need a custom file.
func NewCountryService(geojsonPath string) (*CountryService, error) {
	data, err := os.ReadFile(geojsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read countries GeoJSON: %w", err)
	}
	return newCountryServiceFromData(data)
}

func newCountryServiceFromData(data []byte) (*CountryService, error) {
	fc, err := geojson.UnmarshalFeatureCollection(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse countries GeoJSON: %w", err)
	}

	slog.Info("CountryService: Loaded country boundaries", "features", len(fc.Features))

	return &CountryService{
		features: fc,
		cacheTTL: 15 * time.Second,
	}, nil
}

// GetCountryAtPoint returns the country at the given coordinates.
// Results are cached for 15 seconds to avoid expensive polygon lookups.
func (s *CountryService) GetCountryAtPoint(lat, lon float64) CountryResult {
	// Check cache first
	s.mu.RLock()
	if time.Since(s.lastTime) < s.cacheTTL {
		result := s.lastResult
		s.mu.RUnlock()
		return result
	}
	s.mu.RUnlock()

	// Cache miss - perform lookup
	result := s.lookupCountry(lat, lon)

	// Update cache
	s.mu.Lock()
	s.lastResult = result
	s.lastLat = lat
	s.lastLon = lon
	s.lastTime = time.Now()
	s.mu.Unlock()

	return result
}

// GetCountryName returns the full name of a country given its ISO code.
func (s *CountryService) GetCountryName(code string) string {
	for _, feature := range s.features.Features {
		if getISOCode(feature.Properties) == code {
			return getStringProp(feature.Properties, "NAME")
		}
	}
	return ""
}

// lookupCountry performs the actual point-in-polygon and distance calculations.
func (s *CountryService) lookupCountry(lat, lon float64) CountryResult {
	point := orb.Point{lon, lat} // orb uses [lon, lat] order

	// 1. Check if point is inside any country polygon
	for _, feature := range s.features.Features {
		if containsPoint(feature.Geometry, point) {
			code := getISOCode(feature.Properties)
			name := getStringProp(feature.Properties, "NAME")
			return CountryResult{
				CountryCode: code,
				CountryName: name,
				Zone:        ZoneLand,
				DistanceM:   0,
			}
		}
	}

	// 2. Point is over water - find nearest country
	minDist := math.MaxFloat64
	var nearestCode, nearestName string

	for _, feature := range s.features.Features {
		dist := distanceToGeometry(point, feature.Geometry)
		if dist < minDist {
			minDist = dist
			nearestCode = getISOCode(feature.Properties)
			nearestName = getStringProp(feature.Properties, "NAME")
		}
	}

	// Convert planar distance to approximate meters (at this latitude)
	// For more accuracy, we'd use Haversine, but this is good enough for zones
	distMeters := degreesToMeters(minDist, lat)

	// 3. Determine maritime zone
	var zone string
	switch {
	case distMeters <= TerritorialWatersM:
		zone = ZoneTerritorial
	case distMeters <= EEZM:
		zone = ZoneEEZ
	default:
		zone = ZoneInternational
		nearestCode = ""
		nearestName = ""
	}

	return CountryResult{
		CountryCode: nearestCode,
		CountryName: nearestName,
		Zone:        zone,
		DistanceM:   distMeters,
	}
}

// containsPoint checks if a geometry contains a point.
func containsPoint(geom orb.Geometry, point orb.Point) bool {
	switch g := geom.(type) {
	case orb.Polygon:
		return planar.PolygonContains(g, point)
	case orb.MultiPolygon:
		for _, poly := range g {
			if planar.PolygonContains(poly, point) {
				return true
			}
		}
	}
	return false
}

// distanceToGeometry calculates the minimum distance from a point to any part of a geometry.
func distanceToGeometry(point orb.Point, geom orb.Geometry) float64 {
	switch g := geom.(type) {
	case orb.Polygon:
		return distanceToPolygon(point, g)
	case orb.MultiPolygon:
		minDist := math.MaxFloat64
		for _, poly := range g {
			d := distanceToPolygon(point, poly)
			if d < minDist {
				minDist = d
			}
		}
		return minDist
	}
	return math.MaxFloat64
}

// distanceToPolygon calculates minimum distance from point to polygon boundary.
func distanceToPolygon(point orb.Point, poly orb.Polygon) float64 {
	minDist := math.MaxFloat64

	for _, ring := range poly {
		for i := 0; i < len(ring)-1; i++ {
			d := distanceToSegment(point, ring[i], ring[i+1])
			if d < minDist {
				minDist = d
			}
		}
	}

	return minDist
}

// distanceToSegment calculates the minimum distance from a point to a line segment.
func distanceToSegment(p, a, b orb.Point) float64 {
	// Vector from a to b
	dx := b[0] - a[0]
	dy := b[1] - a[1]

	if dx == 0 && dy == 0 {
		// Segment is a point
		return planar.Distance(p, a)
	}

	// Parameter t for the projection of p onto the line
	t := ((p[0]-a[0])*dx + (p[1]-a[1])*dy) / (dx*dx + dy*dy)

	if t < 0 {
		return planar.Distance(p, a)
	} else if t > 1 {
		return planar.Distance(p, b)
	}

	// Closest point on segment
	closest := orb.Point{a[0] + t*dx, a[1] + t*dy}
	return planar.Distance(p, closest)
}

// degreesToMeters converts a distance in degrees to approximate meters at a given latitude.
func degreesToMeters(degrees, lat float64) float64 {
	// At the equator, 1 degree ≈ 111,320 meters
	// This varies with latitude due to Earth's shape
	latRad := lat * math.Pi / 180
	metersPerDegree := 111320 * math.Cos(latRad)

	// For longitude, we need to account for latitude
	// But since we're using Euclidean distance on degrees, we approximate
	// by using the average of lat and lon scaling
	return degrees * metersPerDegree
}

// getStringProp safely extracts a string property from GeoJSON properties.
func getStringProp(props geojson.Properties, key string) string {
	if val, ok := props[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
		// Handle JSON numbers that might be parsed as float64
		if f, ok := val.(json.Number); ok {
			return string(f)
		}
	}
	return ""
}

// getISOCode extracts the ISO country code, falling back to ISO_A2_EH if ISO_A2 is -99.
// Natural Earth data has -99 for some territories (e.g., France, Kosovo).
func getISOCode(props geojson.Properties) string {
	code := getStringProp(props, "ISO_A2")
	if code == "" || code == "-99" {
		// Fall back to ISO_A2_EH (includes extended/historical codes)
		code = getStringProp(props, "ISO_A2_EH")
	}
	return code
}
