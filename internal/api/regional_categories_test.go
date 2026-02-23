package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
)

// mockHierarchyStore implements store.HierarchyStore for testing
type mockHierarchyStore struct {
	hierarchies map[string]*model.WikidataHierarchy
}

func (m *mockHierarchyStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	if h, ok := m.hierarchies[qid]; ok {
		return h, nil
	}
	// Store actually returns nil, nil if not found
	return nil, nil
}
func (m *mockHierarchyStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error {
	return nil
}
func (m *mockHierarchyStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	return "", false, nil
}
func (m *mockHierarchyStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}

func TestRegionalCategoriesHandler_HandleGet(t *testing.T) {
	c := classifier.NewClassifier(nil, nil, &config.CategoriesConfig{}, nil)
	// Inject some test regional categories
	c.AddRegionalCategories(map[string]string{
		"Q123": "Sights",
		"Q456": "Shopping",
		"Q789": "Nature",
	})

	mockStore := &mockHierarchyStore{
		hierarchies: map[string]*model.WikidataHierarchy{
			"Q123": {QID: "Q123", Name: "Shinto Shrine"},
			"Q456": {QID: "Q456", Name: "Night Market"}, // Q789 missing from DB on purpose
		},
	}

	handler := NewRegionalCategoriesHandler(c, mockStore)

	req, _ := http.NewRequest("GET", "/api/regional", nil)
	rr := httptest.NewRecorder()

	handler.HandleGet(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response []RegionalCategoryResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 3 {
		t.Errorf("expected 3 results, got %d", len(response))
	}

	// Verify mappings (order is not guaranteed due to map iteration)
	foundShrine := false
	foundMarket := false
	foundFallback := false

	for _, item := range response {
		switch item.QID {
		case "Q123":
			foundShrine = true
			if item.Name != "Shinto Shrine" || item.Category != "Sights" {
				t.Errorf("mismatch Q123: %+v", item)
			}
		case "Q456":
			foundMarket = true
			if item.Name != "Night Market" || item.Category != "Shopping" {
				t.Errorf("mismatch Q456: %+v", item)
			}
		case "Q789":
			foundFallback = true
			if item.Name != "Q789" || item.Category != "Nature" {
				t.Errorf("mismatch Q789: %+v", item)
			}
		default:
			t.Errorf("unexpected QID: %s", item.QID)
		}
	}

	if !foundShrine || !foundMarket || !foundFallback {
		t.Errorf("missing expected items in response")
	}
}

func TestRegionalCategoriesHandler_HandleGet_Empty(t *testing.T) {
	c := classifier.NewClassifier(nil, nil, &config.CategoriesConfig{}, nil)
	mockStore := &mockHierarchyStore{}
	handler := NewRegionalCategoriesHandler(c, mockStore)

	req, _ := http.NewRequest("GET", "/api/regional", nil)
	rr := httptest.NewRecorder()

	handler.HandleGet(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if rr.Body.String() != "[]\n" {
		t.Errorf("expected empty JSON array, got %q", rr.Body.String())
	}
}
