package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
)

// flexibleMockStore extends apiMockStore to support dynamic state
type flexibleMockStore struct {
	apiMockStore
	state map[string]string
}

func (m *flexibleMockStore) GetState(ctx context.Context, key string) (string, bool) {
	if m.state != nil {
		if v, ok := m.state[key]; ok {
			return v, true
		}
	}
	return m.apiMockStore.GetState(ctx, key)
}

func (m *flexibleMockStore) SetState(ctx context.Context, key, val string) error {
	if m.state == nil {
		m.state = make(map[string]string)
	}
	m.state[key] = val
	return nil
}

func TestSettlementTierFiltering(t *testing.T) {
	mockStore := &flexibleMockStore{
		state: make(map[string]string),
	}

	// Create config provider backed by our mock store
	configProvider := config.NewProvider(config.DefaultConfig(), mockStore)

	// Inject Settlement group for test
	localCatCfg := &config.CategoriesConfig{
		CategoryGroups: map[string][]string{
			"Settlements": {"City", "Town", "Village"},
		},
		GroupLookup: make(map[string]string),
	}
	for group, cats := range localCatCfg.CategoryGroups {
		for _, cat := range cats {
			localCatCfg.GroupLookup[strings.ToLower(cat)] = group
		}
	}

	mgr := poi.NewManager(configProvider, mockStore, localCatCfg)
	handler := NewPOIHandler(mgr, nil, mockStore, configProvider, nil, nil)

	// Setup Tracked POIs
	city := &model.POI{WikidataID: "Q1", NameEn: "City", Category: "city", Lat: 10.0, Lon: 10.0, Score: 100}
	town := &model.POI{WikidataID: "Q2", NameEn: "Town", Category: "town", Lat: 10.0, Lon: 10.0, Score: 50}
	village := &model.POI{WikidataID: "Q3", NameEn: "Village", Category: "village", Lat: 10.0, Lon: 10.0, Score: 10}

	mgr.TrackPOI(context.Background(), city)
	mgr.TrackPOI(context.Background(), town)
	mgr.TrackPOI(context.Background(), village)

	tests := []struct {
		name           string
		tier           int
		expectCount    int
		expectCategory string // for single item checks
		expectTierIdx  int
	}{
		{
			name:          "Tier 0 (None)",
			tier:          0,
			expectCount:   0,
			expectTierIdx: -1,
		},
		{
			name:           "Tier 1 (City Only)",
			tier:           1,
			expectCount:    1,
			expectCategory: "city",
			expectTierIdx:  0, // City is index 0
		},
		{
			name:           "Tier 2 (City + Town)",
			tier:           2,
			expectCount:    1, // Tier strategy returns highest available. Since City is there, we expect City.
			expectCategory: "city",
			expectTierIdx:  0,
		},
		{
			name:           "Tier 3 (All)",
			tier:           3,
			expectCount:    1, // Still expect City as it's highest priority
			expectCategory: "city",
			expectTierIdx:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the tier in store
			mockStore.state[config.KeySettlementTier] = strconv.Itoa(tt.tier)

			// Query params to include all POIs
			queryParams := "?minLat=9&maxLat=11&minLon=9&maxLon=11"
			req := httptest.NewRequest(http.MethodGet, "/api/map/settlements"+queryParams, nil)
			w := httptest.NewRecorder()

			handler.HandleSettlements(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected 200 OK, got %d", w.Code)
			}

			var resp SettlementResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(resp.Items) != tt.expectCount {
				t.Errorf("Expected %d items, got %d", tt.expectCount, len(resp.Items))
			}

			if tt.expectCount > 0 {
				if resp.Items[0].Category != tt.expectCategory {
					t.Errorf("Expected category %s, got %s", tt.expectCategory, resp.Items[0].Category)
				}
			}

			if resp.TierIndex != tt.expectTierIdx {
				t.Errorf("Expected TierIndex %d, got %d", tt.expectTierIdx, resp.TierIndex)
			}
		})
	}
}

func TestSettlementTierFiltering_ExcludeHighTier(t *testing.T) {
	// Test case where City is present but filtered out by Tier=2?
	// Wait, Tier 2 means City+Town. So City IS included.
	// We want to test if Tier=2 (Town,Village?? No, logic is atomic levels)
	// My implementation:
	// Tier 1: City
	// Tier 2: City, Town
	// Tier 3: City, Town, Village

	// So if I have only Town and Village, and Tier=1 (City only), I should get NOTHING.

	mockStore := &flexibleMockStore{state: make(map[string]string)}
	configProvider := config.NewProvider(config.DefaultConfig(), mockStore)

	localCatCfg := &config.CategoriesConfig{
		CategoryGroups: map[string][]string{"Settlements": {"City", "Town", "Village"}},
		GroupLookup:    make(map[string]string),
	}
	localCatCfg.GroupLookup["town"] = "Settlements"
	localCatCfg.GroupLookup["village"] = "Settlements"

	mgr := poi.NewManager(configProvider, mockStore, localCatCfg)
	handler := NewPOIHandler(mgr, nil, mockStore, configProvider, nil, nil)

	// Town and Village only
	town := &model.POI{WikidataID: "Q2", NameEn: "Town", Category: "town", Lat: 10.0, Lon: 10.0, Score: 50}
	mgr.TrackPOI(context.Background(), town)

	// Set Tier to 1 (City only)
	mockStore.state[config.KeySettlementTier] = "1"

	req := httptest.NewRequest(http.MethodGet, "/api/map/settlements?minLat=9&maxLat=11&minLon=9&maxLon=11", nil)
	w := httptest.NewRecorder()
	handler.HandleSettlements(w, req)

	var resp SettlementResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Items) != 0 {
		t.Errorf("Expected 0 items (Town should be filtered out by Tier 1=City), got %d", len(resp.Items))
	}

	// Set Tier to 2 (City + Town)
	mockStore.state[config.KeySettlementTier] = "2"
	w2 := httptest.NewRecorder()
	handler.HandleSettlements(w2, req)
	json.NewDecoder(w2.Body).Decode(&resp)

	if len(resp.Items) != 1 || resp.Items[0].Category != "town" {
		t.Errorf("Expected Town with Tier 2, got %d items", len(resp.Items))
	}
}
