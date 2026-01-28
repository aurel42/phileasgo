package classifier_test

import (
	"context"
	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/tracker"
	"testing"
)

func TestClassifier_AuxiliaryCoverage(t *testing.T) {
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Mountain": {Size: "XL", Weight: 100},
		},
		IgnoredCategories: map[string]string{
			"Q_IGNORED": "Ignored",
		},
	}

	st := &MockStore{
		Classifications: make(map[string]string),
		Hierarchies:     make(map[string]*model.WikidataHierarchy),
		SeenEntities:    make(map[string]bool),
	}
	cl := &MockClient{Claims: make(map[string]map[string][]string)}
	tr := tracker.New()
	clf := classifier.NewClassifier(st, cl, cfg, tr)

	// 1. GetConfig
	if clf.GetConfig() != cfg {
		t.Error("GetConfig returned wrong config")
	}

	// 2. Dynamic Interests
	dynamic := map[string]string{"Q_DYN": "Mountain"}
	clf.SetDynamicInterests(dynamic)

	// Verify dynamic lookup works via Classify
	// Classify first checks if the QID itself maps to a category
	res, err := clf.Classify(context.Background(), "Q_DYN")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if res == nil || res.Category != "Mountain" {
		t.Errorf("Expected dynamic match Mountain, got %v", res)
	}
}
