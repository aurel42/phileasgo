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
	Name       string
	WikidataID string
	Geom       orb.MultiLineString
	Mouth      geo.Point
	Source     geo.Point
	BBox       orb.Bound
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

	groups := s.groupSegments(fc)

	for name, group := range groups {
		if group.wikidataID == "" {
			s.logger.Debug("Dropping river without Wikidata ID", "name", name)
			continue
		}
		s.rivers = append(s.rivers, s.createRiverFromGroup(name, group))
	}

	s.logger.Info("Loaded and merged rivers", "count", len(s.rivers))
	return nil
}

type riverGroup struct {
	mls        orb.MultiLineString
	bbox       orb.Bound
	wikidataID string
}

func (s *Sentinel) groupSegments(fc *geojson.FeatureCollection) map[string]*riverGroup {
	groups := make(map[string]*riverGroup)

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

		group, ok := groups[name]
		if !ok {
			group = &riverGroup{
				mls:  make(orb.MultiLineString, 0),
				bbox: mls.Bound(),
			}
			groups[name] = group
		} else {
			b := mls.Bound()
			group.bbox = group.bbox.Extend(b.Min)
			group.bbox = group.bbox.Extend(b.Max)
		}
		group.mls = append(group.mls, mls...)

		// Capture Wikidata ID if present in this segment
		if wid, ok := f.Properties["wikidataid"].(string); ok && wid != "" {
			group.wikidataID = wid
		}
	}
	return groups
}

func (s *Sentinel) createRiverFromGroup(name string, group *riverGroup) River {
	// Identify Global Ends
	endpoints := make(map[orb.Point]int)
	for _, line := range group.mls {
		if len(line) < 2 {
			continue
		}
		endpoints[line[0]]++
		endpoints[line[len(line)-1]]++
	}

	var extremities []orb.Point
	for p, count := range endpoints {
		if count == 1 {
			extremities = append(extremities, p)
		}
	}

	var source, mouth geo.Point
	if len(extremities) >= 2 {
		source = geo.Point{Lat: extremities[0][1], Lon: extremities[0][0]}
		mouth = geo.Point{Lat: extremities[1][1], Lon: extremities[1][0]}
	} else if len(group.mls) > 0 {
		l := group.mls
		source = geo.Point{Lat: l[0][0][1], Lon: l[0][0][0]}
		lastLine := l[len(l)-1]
		mouth = geo.Point{Lat: lastLine[len(lastLine)-1][1], Lon: lastLine[len(lastLine)-1][0]}
	}

	return River{
		Name:       name,
		WikidataID: group.wikidataID,
		Geom:       group.mls,
		Mouth:      mouth,
		Source:     source,
		BBox:       group.bbox,
	}
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
				WikidataID: r.WikidataID,
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
