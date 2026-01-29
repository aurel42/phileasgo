package rivers

import (
	"log/slog"
	"os"
	"sync"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
)

// River represents a loaded river feature
type River struct {
	Name   string
	Geom   orb.MultiLineString
	Mouth  geo.Point
	Source geo.Point
	BBox   orb.Bound
}

// Sentinel monitors aircraft position relative to rivers.
type Sentinel struct {
	logger *slog.Logger
	rivers []River
	mu     sync.RWMutex
}

// NewSentinel loads river data and initializes the sentinel.
func NewSentinel(logger *slog.Logger, geojsonPath string) *Sentinel {
	s := &Sentinel{
		logger: logger,
		rivers: make([]River, 0),
	}
	if err := s.loadData(geojsonPath); err != nil {
		logger.Error("Failed to load river data", "path", geojsonPath, "err", err)
	}
	return s
}

func (s *Sentinel) loadData(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fc, err := geojson.UnmarshalFeatureCollection(data)
	if err != nil {
		return err
	}

	for _, f := range fc.Features {
		name := ""
		if n, ok := f.Properties["name_en"].(string); ok && n != "" {
			name = n
		} else if n, ok := f.Properties["name"].(string); ok && n != "" {
			name = n
		}

		if name == "" {
			continue
		}

		// Convert Geometry
		var mls orb.MultiLineString
		switch g := f.Geometry.(type) {
		case orb.MultiLineString:
			mls = g
		case orb.LineString:
			mls = orb.MultiLineString{g}
		default:
			continue
		}

		if len(mls) == 0 || len(mls[0]) == 0 {
			continue
		}

		// Determine Ends
		// Assuming last point of last segment is mouth (standard for NE rivers but validation helps)
		// For now we trust the order or just take extremities.
		// Let's create a bbox for fast filter
		bbox := mls.Bound()

		// Find "End" and "Start" based on simplified assumption or just endpoints of the longest line?
		// Usually MLS is segmented. The bounding box extremities might work?
		// Or simpler: The *endpoints* of the geometry.
		// Since it's a MultiLineString, it might be disjoint.
		// Ideally we treat each feature as a single river entity.
		// Lets take the first point of first segment and last point of last segment.
		start := geo.Point{Lat: mls[0][0][1], Lon: mls[0][0][0]}
		lastSeg := mls[len(mls)-1]
		end := geo.Point{Lat: lastSeg[len(lastSeg)-1][1], Lon: lastSeg[len(lastSeg)-1][0]}

		s.rivers = append(s.rivers, River{
			Name:   name,
			Geom:   mls,
			Mouth:  end, // Assume downstream
			Source: start,
			BBox:   bbox,
		})
	}
	s.logger.Info("Loaded rivers", "count", len(s.rivers))
	return nil
}

// Update checks for nearby rivers relative to aircraft.
func (s *Sentinel) Update(lat, lon, heading float64) *model.RiverCandidate {
	const DetectionRadius = 25000.0 // 25km (broad search, narrow down later)

	p := geo.Point{Lat: lat, Lon: lon}

	// Thread-safe if we edit state, but here we read-only mostly
	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *model.RiverCandidate
	minDist := 1000000.0 // large init

	for _, r := range s.rivers {
		// 1. BBox Filter (Rough lat/lon check first)
		// 1 deg lat ~ 111km. 0.5 deg ~ 55km.
		if lat < r.BBox.Min[1]-0.5 || lat > r.BBox.Max[1]+0.5 ||
			lon < r.BBox.Min[0]-1.0 || lon > r.BBox.Max[0]+1.0 { // Lon scale varies but this is loose filter
			continue
		}

		// 2. Accurate Distance
		// Find closest point on ALL segments
		riverClosest := 1000000.0
		var rPoint geo.Point

		for _, line := range r.Geom {
			for i := 0; i < len(line)-1; i++ {
				// Convert orb.Point to geo.Point
				a := geo.Point{Lat: line[i][1], Lon: line[i][0]}
				b := geo.Point{Lat: line[i+1][1], Lon: line[i+1][0]}

				dist, closest := geo.DistancePointSegment(p, a, b)
				if dist < riverClosest {
					riverClosest = dist
					rPoint = closest
				}
			}
		}

		if riverClosest > DetectionRadius {
			continue
		}

		// 3. Ahead Check
		// Must be within +/- 90 degrees relative bearing
		if !geo.IsAhead(p, rPoint, heading) {
			continue
		}

		// 4. Competition
		if riverClosest < minDist {
			minDist = riverClosest
			best = &model.RiverCandidate{
				Name:       r.Name,
				ClosestLat: rPoint.Lat,
				ClosestLon: rPoint.Lon,
				Distance:   riverClosest,
				IsAhead:    true,
				MouthLat:   r.Mouth.Lat,
				MouthLon:   r.Mouth.Lon,
				SourceLat:  r.Source.Lat,
				SourceLon:  r.Source.Lon,
			}
		}
	}

	return best
}
