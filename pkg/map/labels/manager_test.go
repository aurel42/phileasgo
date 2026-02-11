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

// MockLimitProvider implements LabelLimitProvider for tests
type MockLimitProvider struct {
	Limit int
	Tier  int
}

func (m *MockLimitProvider) SettlementLabelLimit(ctx context.Context) int {
	return m.Limit
}

func (m *MockLimitProvider) SettlementTier(ctx context.Context) int {
	return m.Tier
}

func TestSelectLabels_Unified(t *testing.T) {
	geoSvc := &geo.Service{}
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()

	p1 := &model.POI{WikidataID: "Q1", NameEn: "Big City", Lat: 10.0, Lon: 10.0, Category: "City"}
	_ = poiSvc.TrackPOI(ctx, p1)

	p2 := &model.POI{WikidataID: "Q2", NameEn: "Nearby Town", Lat: 10.1, Lon: 10.1, Category: "Town"}
	_ = poiSvc.TrackPOI(ctx, p2)

	p3 := &model.POI{WikidataID: "Q3", NameEn: "Far Village", Lat: 20.0, Lon: 20.0, Category: "Village"}
	_ = poiSvc.TrackPOI(ctx, p3)

	m := NewManager(geoSvc, poiSvc, nil)

	candidates := m.SelectLabels(0, 0, 30, 30, 15, 15, 0, nil, 10)

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
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "C", NameEn: "C", Lat: 1, Lon: 1, Category: "City"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "T", NameEn: "T", Lat: 5, Lon: 5, Category: "Town"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "V", NameEn: "V", Lat: 10, Lon: 10, Category: "Village"})

	m := NewManager(&geo.Service{}, poiSvc, nil)
	results := m.SelectLabels(0, 0, 20, 20, 0, 0, 0, nil, 10)

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
	m := NewManager(&geo.Service{}, poi.NewManager(nil, &MockStore{}, nil), nil)
	ctx := context.Background()

	m.poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "P1", NameEn: "P1", Lat: 10, Lon: 10, Category: "City"})

	existing := []geo.Point{{Lat: 10.1, Lon: 10.1}}

	results := m.SelectLabels(0, 0, 30, 30, 0, 0, 0, existing, 10)

	if len(results) != 0 {
		t.Errorf("Expected 0 labels due to collision with locked label, got %d", len(results))
	}
}

func TestSelectLabels_ZoomReset(t *testing.T) {
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "A", NameEn: "A", Lat: 5, Lon: 5, Category: "City"})

	m := NewManager(&geo.Service{}, poiSvc, nil)

	// First call at zoom 10
	r1 := m.SelectLabels(0, 0, 10, 10, 5, 5, 0, nil, 10.5)
	if len(r1) != 1 {
		t.Fatalf("Expected 1 label at zoom 10, got %d", len(r1))
	}

	// Verify it's in activeSettlements
	m.mu.Lock()
	if len(m.activeSettlements) == 0 {
		t.Error("Expected activeSettlements to have entries after first call")
	}
	m.mu.Unlock()

	// Call at different zoom floor (11) should reset
	_ = m.SelectLabels(0, 0, 10, 10, 5, 5, 0, nil, 11.0)

	// The POI should be re-discovered, but the point is that state was cleared
	m.mu.Lock()
	// After the zoom change, activeSettlements was cleared and re-populated
	// Verify the zoom floor was updated
	if m.currentZoomFloor != 11 {
		t.Errorf("Expected currentZoomFloor=11 after zoom change, got %d", m.currentZoomFloor)
	}
	m.mu.Unlock()
}

func TestSelectLabels_Shadow(t *testing.T) {
	// Test that a shadow (high-score city outside viewport) blocks a nearby small town inside viewport
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()

	// Small town inside the viewport
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "SmallTown", NameEn: "ST", Lat: 5, Lon: 8, Category: "Village"})

	// Big city slightly outside viewport (ahead, heading=90 means east)
	// Will be inside the expanded bbox (~30% extension along heading)
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "BigCity", NameEn: "BC", Lat: 5, Lon: 8.5, Category: "City"})

	m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 10, Tier: 3})

	// Viewport: lat 0-10, lon 0-8.2 — SmallTown is inside, BigCity is just outside
	// Heading 90 (east) → expanded bbox extends lon by ~30% of 8.2 ≈ +2.5
	// BigCity at lon=8.5 should be in the expanded bbox as a shadow
	results := m.SelectLabels(0, 0, 10, 8.2, 5, 4, 90, nil, 10)

	// BigCity (outside viewport) should be a shadow, blocking SmallTown via MSR
	// MSR = (8.2) * 0.3 = 2.46, msrDegSq = 6.05
	// Distance SmallTown-BigCity: sqrt((0)^2 + (0.5)^2) = 0.5, distSq = 0.25 < 6.05 → blocked
	foundSmall := false
	foundBig := false
	for _, c := range results {
		if c.GenericID == "SmallTown" {
			foundSmall = true
		}
		if c.GenericID == "BigCity" {
			foundBig = true
		}
	}

	// BigCity should NOT be in results (it's a shadow, outside viewport)
	if foundBig {
		t.Error("BigCity should not be in results (it's a shadow)")
	}

	// SmallTown should also NOT be in results (blocked by BigCity's MSR)
	if foundSmall {
		t.Error("SmallTown should be blocked by BigCity shadow via MSR")
	}
}

func TestSelectLabels_LimitRespected(t *testing.T) {
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()

	// Add 5 well-separated cities
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "A", NameEn: "A", Lat: 2, Lon: 2, Category: "City"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "B", NameEn: "B", Lat: 20, Lon: 20, Category: "City"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "C", NameEn: "C", Lat: 40, Lon: 40, Category: "City"})

	m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 2, Tier: 3})

	results := m.SelectLabels(0, 0, 50, 50, 25, 25, 0, nil, 10)

	if len(results) > 2 {
		t.Errorf("Expected at most 2 labels (limit=2), got %d", len(results))
	}
}

func TestSelectLabels_TierFiltering(t *testing.T) {
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()

	// Well-separated: city, town, village
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "C", NameEn: "BigCity", Lat: 2, Lon: 2, Category: "City"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "T", NameEn: "MidTown", Lat: 15, Lon: 15, Category: "Town"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "V", NameEn: "SmallVillage", Lat: 28, Lon: 28, Category: "Village"})

	t.Run("Tier0_None", func(t *testing.T) {
		m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 100, Tier: 0})
		results := m.SelectLabels(0, 0, 30, 30, 15, 15, 0, nil, 10)
		if len(results) != 0 {
			t.Errorf("Tier 0: expected 0 labels, got %d", len(results))
		}
	})

	t.Run("Tier1_CityOnly", func(t *testing.T) {
		m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 100, Tier: 1})
		results := m.SelectLabels(0, 0, 30, 30, 15, 15, 0, nil, 10)
		if len(results) != 1 {
			t.Errorf("Tier 1: expected 1 label (city only), got %d", len(results))
		}
		if len(results) == 1 && results[0].Category != "city" {
			t.Errorf("Tier 1: expected city, got %s", results[0].Category)
		}
	})

	t.Run("Tier2_CityAndTown", func(t *testing.T) {
		m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 100, Tier: 2})
		results := m.SelectLabels(0, 0, 30, 30, 15, 15, 0, nil, 10)
		if len(results) != 2 {
			t.Errorf("Tier 2: expected 2 labels (city+town), got %d", len(results))
		}
		for _, r := range results {
			if r.Category == "village" {
				t.Errorf("Tier 2: village should be filtered out")
			}
		}
	})

	t.Run("Tier3_All", func(t *testing.T) {
		m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 100, Tier: 3})
		results := m.SelectLabels(0, 0, 30, 30, 15, 15, 0, nil, 10)
		if len(results) != 3 {
			t.Errorf("Tier 3: expected 3 labels, got %d", len(results))
		}
	})
}
func TestSelectLabels_LimitChangeReset(t *testing.T) {
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "A", NameEn: "A", Lat: 5, Lon: 5, Category: "City"})

	provider := &MockLimitProvider{Limit: 10, Tier: 3}
	m := NewManager(&geo.Service{}, poiSvc, provider)

	// First call at limit 10
	r1 := m.SelectLabels(0, 0, 10, 10, 5, 5, 0, nil, 10.0)
	if len(r1) != 1 {
		t.Fatalf("Expected 1 label, got %d", len(r1))
	}

	// Change limit to 5
	provider.Limit = 5
	_ = m.SelectLabels(0, 0, 10, 10, 5, 5, 0, nil, 10.0)

	// Verify limit was updated and state was (briefly) cleared
	m.mu.Lock()
	if m.lastLimit != 5 {
		t.Errorf("Expected lastLimit=5, got %d", m.lastLimit)
	}
	m.mu.Unlock()
}

func TestSelectLabels_FadingFreesSlot(t *testing.T) {
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()

	// City A at the left edge of viewport 1, City B well-separated in the center
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "A", NameEn: "EdgeCity", Lat: 5, Lon: 1, Category: "City"})
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "B", NameEn: "CenterCity", Lat: 5, Lon: 5, Category: "City"})
	// City C is outside viewport 1 but inside viewport 2
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "C", NameEn: "NewCity", Lat: 5, Lon: 9, Category: "City"})

	m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 2, Tier: 3})

	// Viewport 1: lon 0-10, all three cities inside
	r1 := m.SelectLabels(0, 0, 10, 10, 5, 5, 0, nil, 10)
	if len(r1) != 2 {
		t.Fatalf("Call 1: expected 2 labels (limit), got %d", len(r1))
	}

	// Viewport 2: shift east — lon 4-14. City A (lon=1) is now outside the viewport.
	// With fading, A should be marked fading, freeing a slot for C.
	r2 := m.SelectLabels(0, 4, 10, 14, 5, 9, 0, nil, 10)

	// A should be fading (outside inset), so not returned.
	// B (lon=5) is inside the viewport (lon 4-14).
	// C (lon=9) is well inside.
	foundA := false
	foundC := false
	for _, r := range r2 {
		if r.GenericID == "A" {
			foundA = true
		}
		if r.GenericID == "C" {
			foundC = true
		}
	}

	if foundA {
		t.Error("City A should be fading (outside shifted viewport), not returned")
	}
	if !foundC {
		t.Error("City C should fill the slot freed by fading City A")
	}
	if len(r2) != 2 {
		t.Errorf("Call 2: expected 2 labels, got %d", len(r2))
	}
}

func TestSelectLabels_EllipticalExclusion(t *testing.T) {
	poiSvc := poi.NewManager(nil, &MockStore{}, nil)
	ctx := context.Background()

	// Two very long named cities vertically close
	// City A at (5, 5). Name 29 chars -> msrX = msrDeg * (0.8 + 29*0.04) = 1.96 * msrDeg.
	// Viewport span 10 -> msrDeg = 3. msrX = 5.88.
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "Long1", NameEn: "Sankt Margarethen an der Raab", Lat: 5, Lon: 5, Category: "City"})

	// City B at (5.1, 9). dx = 4, dy = 0.1.
	// Circular: dist = sqrt(4^2 + 0.1^2) = 4.001. 4.001 > 3 -> PASS (would show both).
	// Elliptical: (4/5.88)^2 + (0.1/3)^2 = 0.46 + 0.001 = 0.46 < 1 -> REJECT (should block).
	poiSvc.TrackPOI(ctx, &model.POI{WikidataID: "Long2", NameEn: "Petrogradskoye Shosse", Lat: 5.1, Lon: 9, Category: "City"})

	m := NewManager(&geo.Service{}, poiSvc, &MockLimitProvider{Limit: 2, Tier: 3})

	// Viewport 0-10
	results := m.SelectLabels(0, 0, 10, 10, 5, 5, 0, nil, 10)

	// Should only find the first one because they collide elliptically
	if len(results) != 1 {
		t.Errorf("Expected 1 label due to elliptical collision, got %d", len(results))
	}
}
