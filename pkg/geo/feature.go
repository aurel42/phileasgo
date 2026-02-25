package geo

import (
	"fmt"
	"os"
	"phileasgo/pkg/model"
	"sync"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// FeatureResult represents a matched spatial feature.
type FeatureResult struct {
	Name     string
	QID      string
	Category string
}

// FeatureService handles lookup of multiple spatial features from GeoJSON layers.
type FeatureService struct {
	mu       sync.RWMutex
	features []*geojson.Feature
}

// NewFeatureService creates a new service and loads the specified GeoJSON files.
func NewFeatureService(paths ...string) (*FeatureService, error) {
	s := &FeatureService{}
	for _, path := range paths {
		if err := s.load(path); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *FeatureService) load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read geojson %s: %w", path, err)
	}

	fc, err := geojson.UnmarshalFeatureCollection(data)
	if err != nil {
		return fmt.Errorf("failed to parse geojson %s: %w", path, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.features = append(s.features, fc.Features...)
	return nil
}

// GetFeaturesAtPoint returns all features covering the given coordinates.
func (s *FeatureService) GetFeaturesAtPoint(lat, lon float64) []FeatureResult {
	point := orb.Point{lon, lat}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []FeatureResult
	for _, f := range s.features {
		// Fast bounding box check
		if !f.Geometry.Bound().Contains(point) {
			continue
		}

		if containsPoint(f.Geometry, point) {
			results = append(results, FeatureResult{
				Name:     getStringProp(f.Properties, "name"),
				QID:      getStringProp(f.Properties, "qid"),
				Category: getStringProp(f.Properties, "category"),
			})
		}
	}

	return results
}

// ToPOI converts a FeatureResult to a model.POI for injection into the context.
func (r FeatureResult) ToPOI() *model.POI {
	return &model.POI{
		NameEn:          r.Name,
		WikidataID:      r.QID,
		Category:        r.Category,
		IsHiddenFeature: true,
		Source:          "natural_earth",
	}
}
