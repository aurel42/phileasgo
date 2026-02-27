package poi

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"
)

func TestHiddenPOIFiltering(t *testing.T) {
	mgr := NewManager(config.NewProvider(&config.Config{}, nil), NewMockStore(), nil)

	tests := []struct {
		name           string
		poi            *model.POI
		isHidden       bool
		shouldBeInUI   bool
		shouldBeInAuto bool
	}{
		{
			name:           "Normal POI",
			poi:            &model.POI{WikidataID: "Q1", Score: 10, IsVisible: true, Visibility: 1.0},
			isHidden:       false,
			shouldBeInUI:   true,
			shouldBeInAuto: true,
		},
		{
			name:           "Hidden Feature",
			poi:            &model.POI{WikidataID: "Q2", Score: 10, IsHiddenFeature: true, IsVisible: true, Visibility: 1.0},
			isHidden:       true,
			shouldBeInUI:   false,
			shouldBeInAuto: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr.trackedPOIs = map[string]*model.POI{
				tt.poi.WikidataID: tt.poi,
			}

			// Test GetPOIsForUI
			uiPois, _ := mgr.GetPOIsForUI("fixed", 10, 0)
			foundInUI := false
			for _, p := range uiPois {
				if p.WikidataID == tt.poi.WikidataID {
					foundInUI = true
					break
				}
			}
			if foundInUI != tt.shouldBeInUI {
				t.Errorf("GetPOIsForUI: expected found=%v, got %v", tt.shouldBeInUI, foundInUI)
			}

			// Test GetNarrationCandidates
			candidates := mgr.GetNarrationCandidates(10, nil)
			foundInAuto := false
			for _, p := range candidates {
				if p.WikidataID == tt.poi.WikidataID {
					foundInAuto = true
					break
				}
			}
			if foundInAuto != tt.shouldBeInAuto {
				t.Errorf("GetNarrationCandidates: expected found=%v, got %v", tt.shouldBeInAuto, foundInAuto)
			}
		})
	}
}
