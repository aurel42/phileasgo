package labels

import (
	"context"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"testing"
	"time"
)

// Minimal MockStore for satisfying poi.Manager dependency
type MockStore struct{}

func (s *MockStore) SavePOI(ctx context.Context, p *model.POI) error           { return nil }
func (s *MockStore) GetPOI(ctx context.Context, id string) (*model.POI, error) { return nil, nil }
func (s *MockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return nil, nil
}
func (s *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (s *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (s *MockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}
func (s *MockStore) SaveMSFSPOI(ctx context.Context, p *model.MSFSPOI) error { return nil }
func (s *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (s *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (s *MockStore) GetState(ctx context.Context, key string) (string, bool) { return "", false }
func (s *MockStore) SetState(ctx context.Context, key, val string) error     { return nil }
func (s *MockStore) DeleteState(ctx context.Context, key string) error       { return nil }
func (s *MockStore) Close() error                                            { return nil }
func (s *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return nil, nil
}

func TestSelectLabels_Unified(t *testing.T) {
	// 1. Setup minimal Geo Service
	geoSvc := &geo.Service{}

	// 2. Setup POI Manager
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)

	// Add some tracked POIs (Local discoveries)
	ctx := context.Background()

	// A high-population City discovery
	p1 := &model.POI{
		WikidataID: "Q1",
		NameEn:     "Big City",
		Lat:        10.0,
		Lon:        10.0,
		Category:   "City",
	}
	_ = poiSvc.TrackPOI(ctx, p1)

	// A Town discovery nearby (should be pruned by MSR if too close)
	p2 := &model.POI{
		WikidataID: "Q2",
		NameEn:     "Nearby Town",
		Lat:        10.1,
		Lon:        10.1,
		Category:   "Town",
	}
	_ = poiSvc.TrackPOI(ctx, p2)

	// A Village discovery far away
	p3 := &model.POI{
		WikidataID: "Q3",
		NameEn:     "Far Village",
		Lat:        20.0,
		Lon:        20.0,
		Category:   "Village",
	}
	_ = poiSvc.TrackPOI(ctx, p3)

	m := NewManager(geoSvc, poiSvc)

	// 3. Test SelectLabels
	// Viewport covering everything
	// heading 0, ac at center
	candidates := m.SelectLabels(0, 0, 30, 30, 15, 15, 0, nil)

	// MSR is (maxLon-minLon)*0.3 = 30 * 0.3 = 9 degrees.
	// Dist(Q1, Q2) = 0.14 deg -> Pruned.
	// Q1 and Q3 should survive.

	if len(candidates) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(candidates))
	}

	foundQ1 := false
	foundQ3 := false
	for _, c := range candidates {
		if c.GenericID == "Q1" {
			foundQ1 = true
			if c.Category != "city" {
				t.Errorf("Q1 category should be city, got %s", c.Category)
			}
		}
		if c.GenericID == "Q3" {
			foundQ3 = true
			if c.Category != "village" {
				t.Errorf("Q3 category should be village, got %s", c.Category)
			}
		}
	}

	if !foundQ1 || !foundQ3 {
		t.Errorf("Did not find expected candidates Q1/Q3")
	}
}

func TestSelectLabels_Tiering(t *testing.T) {
	m := NewManager(&geo.Service{}, nil)

	// Since we can't easily inject cities into a mock geo.Service without a file,
	// we'll rely on our unified POI logic to verify the tiering constants we added.
	// We'll track 3 POIs with different "Category" strings and check their promoted "cat" in SelectLabels.

	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "C", NameEn: "C", Lat: 1, Lon: 1, Category: "City"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "T", NameEn: "T", Lat: 5, Lon: 5, Category: "Town"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "V", NameEn: "V", Lat: 10, Lon: 10, Category: "Village"})

	m.poiSvc = poiSvc
	results := m.SelectLabels(0, 0, 20, 20, 0, 0, 0, nil)

	for _, r := range results {
		switch r.GenericID {
		case "C":
			if r.Category != "city" {
				t.Errorf("C: expected city, got %s", r.Category)
			}
		case "T":
			if r.Category != "town" {
				t.Errorf("T: expected town, got %s", r.Category)
			}
		case "V":
			if r.Category != "village" {
				t.Errorf("V: expected village, got %s", r.Category)
			}
		}
	}
}

func TestSelectLabels_Locked(t *testing.T) {
	m := NewManager(&geo.Service{}, poi.NewManager(nil, &MockStore{}, nil))
	ctx := context.Background()

	// 1. Add a POI at (10, 10)
	m.poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "P1", NameEn: "P1", Lat: 10, Lon: 10, Category: "City"})

	// 2. Define an "Existing" (locked) label at (10.1, 10.1)
	existing := []geo.Point{{Lat: 10.1, Lon: 10.1}}

	// 3. Select labels with 30 deg viewport -> 9 deg MSR
	// Q1 should be pruned by the locked label
	results := m.SelectLabels(0, 0, 30, 30, 0, 0, 0, existing)

	if len(results) != 0 {
		t.Errorf("Expected 0 labels due to collision with locked label, got %d", len(results))
	}
}
