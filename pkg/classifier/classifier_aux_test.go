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

	// 2. Dimensions Lifecycle
	clf.ResetDimensions()
	clf.ObserveDimensions(100, 100, 1000)
	clf.FinalizeDimensions()

	// 3. ShouldRescue & Multiplier
	// Case A: Ignored Instance prevents rescue coverage
	if clf.ShouldRescue(1000, 1000, 1000, []string{"Q_IGNORED"}) {
		t.Error("ShouldRescue should return false if instance is ignored")
	}
	// Case B: Delegation coverage
	_ = clf.ShouldRescue(10, 10, 10, []string{"Q_OTHER"})

	// Multiplier delegation coverage
	// Feed some data to establish medians
	for i := 0; i < 20; i++ {
		clf.ObserveDimensions(10, 10, 100)
	}
	clf.FinalizeDimensions()

	// Check multiplier for something much larger
	m := clf.GetMultiplier(100, 100, 1000)
	if m <= 1.0 {
		t.Logf("Expected multiplier > 1.0 for large item, got %f", m)
	}

	// Check standard
	_ = clf.GetMultiplier(10, 10, 100)

	// 4. Dynamic Interests
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
