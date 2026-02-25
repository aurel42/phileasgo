package geo

import (
	"encoding/json"
	"math"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

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
		code = getStringProp(props, "iso_a2")
	}

	if code == "" || code == "-99" {
		// Fall back to ISO_A2_EH (includes extended/historical codes)
		code = getStringProp(props, "ISO_A2_EH")
		if code == "" || code == "-99" {
			code = getStringProp(props, "iso_a2_eh")
		}
	}
	return code
}
