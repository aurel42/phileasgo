package classifier_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/wikidata"
)

// MockClient implements classifier.WikidataClient
type MockClient struct {
	Claims        map[string]map[string][]string // qid -> property -> [targets]
	BatchCalls    int
	SingleCalls   int
	BatchEntities int
	ErrorOn       map[string]bool // qid -> true returns error
	FailOnce      map[string]bool // qid -> true returns error once then succeeds
	ErrorOnBatch  bool            // if true, GetEntitiesBatch returns error
}

func (m *MockClient) GetEntityClaims(ctx context.Context, id, property string) (targets []string, label string, err error) {
	if m.ErrorOn[id] {
		return nil, "", fmt.Errorf("simulated error for %s", id)
	}
	if m.FailOnce[id] {
		delete(m.FailOnce, id)
		return nil, "", fmt.Errorf("simulated one-time failure for %s", id)
	}
	m.SingleCalls++
	if props, ok := m.Claims[id]; ok {
		if t, ok := props[property]; ok {
			return t, "Label: " + id, nil
		}
	}
	return nil, "Label: " + id, nil
}

func (m *MockClient) GetEntityClaimsBatch(ctx context.Context, ids []string, property string) (claims map[string][]string, labels map[string]string, err error) {
	for _, id := range ids {
		if m.ErrorOn[id] {
			return nil, nil, fmt.Errorf("simulated batch error for %s", id)
		}
	}
	m.BatchCalls++
	claims = make(map[string][]string)
	labels = make(map[string]string)

	for _, id := range ids {
		labels[id] = "Label: " + id
		if props, ok := m.Claims[id]; ok {
			if t, ok := props[property]; ok {
				claims[id] = t
			}
		}
	}
	return claims, labels, nil
}

func (m *MockClient) GetEntitiesBatch(ctx context.Context, ids []string) (map[string]wikidata.EntityMetadata, error) {
	if m.ErrorOnBatch {
		return nil, fmt.Errorf("simulated batch error")
	}
	m.BatchEntities++
	res := make(map[string]wikidata.EntityMetadata)
	for _, id := range ids {
		meta := wikidata.EntityMetadata{
			Labels: map[string]string{"en": "Label: " + id},
			Claims: make(map[string][]string),
		}
		if props, ok := m.Claims[id]; ok {
			for prop, targets := range props {
				meta.Claims[prop] = targets
			}
		}
		res[id] = meta
	}
	return res, nil
}

type MockStore struct {
	Classifications      map[string]string
	Hierarchies          map[string]*model.WikidataHierarchy
	SeenEntities         map[string]bool
	GetClassCalls        int
	GetHierCalls         int
	GetSeenCalls         int
	ErrorOnSave          bool
	SavedClassifications []ClassificationEntry
}

type ClassificationEntry struct {
	QID      string
	Category string
	Parents  []string
	Label    string
}

func (m *MockStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	m.GetClassCalls++
	if val, ok := m.Classifications[qid]; ok {
		return val, true, nil
	}
	return "", false, nil
}

func (m *MockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	if m.ErrorOnSave {
		return fmt.Errorf("simulated save error")
	}
	m.Classifications[qid] = category
	m.SavedClassifications = append(m.SavedClassifications, ClassificationEntry{
		QID:      qid,
		Category: category,
		Parents:  parents,
		Label:    label,
	})
	return nil
}

func (m *MockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	m.GetHierCalls++
	if h, ok := m.Hierarchies[qid]; ok {
		return h, nil
	}
	return nil, nil
}

func (m *MockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	m.GetSeenCalls++
	res := make(map[string][]string)
	for _, qid := range qids {
		if m.SeenEntities[qid] {
			res[qid] = []string{}
		}
	}
	return res, nil
}

func (m *MockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	for qid := range entities {
		m.SeenEntities[qid] = true
	}
	return nil
}

func (m *MockStore) DeleteSeenEntities(ctx context.Context, qids []string) error {
	return nil
}

func (m *MockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error {
	m.Hierarchies[h.QID] = h
	return nil
}

func (m *MockStore) GetPOI(ctx context.Context, wikidataID string) (*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	return make(map[string]*model.POI), nil
}
func (m *MockStore) SavePOI(ctx context.Context, poi *model.POI) error { return nil }
func (m *MockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *MockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *MockStore) SaveMSFSPOI(ctx context.Context, poi *model.MSFSPOI) error { return nil }
func (m *MockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *MockStore) GetState(ctx context.Context, key string) (string, bool)     { return "", false }
func (m *MockStore) SetState(ctx context.Context, key, value string) error       { return nil }
func (m *MockStore) SetCache(ctx context.Context, key string, data []byte) error { return nil }
func (m *MockStore) GetCache(ctx context.Context, key string) ([]byte, bool)     { return nil, false }
func (m *MockStore) HasCache(ctx context.Context, key string) (bool, error)      { return false, nil }
func (m *MockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *MockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	return nil
}
func (m *MockStore) SaveLastPlayed(ctx context.Context, poiID string, t time.Time) error { return nil }
func (m *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *MockStore) SaveArticle(ctx context.Context, a *model.Article) error { return nil }
func (m *MockStore) Close() error                                            { return nil }

func TestClassifier_Classify_Detailed(t *testing.T) {
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Match": {QIDs: map[string]string{"Q_MATCH_ROOT": "Match"}, Size: "M", Weight: 100},
		},
		IgnoredCategories: map[string]string{
			"Q_IGNORE_ROOT": "Ignored",
		},
	}

	tests := []classifierTestCase{
		{
			name: "Diamond Hierarchy (Match wins)",
			qid:  "Q_DIAMOND",
			setupClient: func(c *MockClient) {
				// Q_DIAMOND -> [Q_B, Q_C]
				// Q_B -> Q_IGNORE_ROOT
				// Q_C -> Q_MATCH_ROOT
				c.Claims["Q_DIAMOND"] = map[string][]string{"P31": {"Q_B", "Q_C"}}
				c.Claims["Q_B"] = map[string][]string{"P279": {"Q_IGNORE_ROOT"}}
				c.Claims["Q_C"] = map[string][]string{"P279": {"Q_MATCH_ROOT"}}
			},
			expectedCat: "Match",
		},
		{
			name: "Cycle Detection (Termination)",
			qid:  "Q_CYCLE_A",
			setupClient: func(c *MockClient) {
				// Q_A -> Q_B -> Q_A
				c.Claims["Q_CYCLE_A"] = map[string][]string{"P31": {"Q_CYCLE_B"}}
				c.Claims["Q_CYCLE_B"] = map[string][]string{"P279": {"Q_CYCLE_A"}}
			},
			expectedCat: "", // Dead end
		},
		{
			name: "Max Depth Limit (Fail at Level 10)",
			qid:  "Q_DEEP_0",
			setupClient: func(c *MockClient) {
				// Q0 -> Q1 -> ... -> Q9 -> Q_MATCH (Level 10)
				c.Claims["Q_DEEP_0"] = map[string][]string{"P31": {"Q_DEEP_1"}}
				for i := 1; i < 9; i++ {
					c.Claims[fmt.Sprintf("Q_DEEP_%d", i)] = map[string][]string{"P279": {fmt.Sprintf("Q_DEEP_%d", i+1)}}
				}
				c.Claims["Q_DEEP_9"] = map[string][]string{"P279": {"Q_MATCH_ROOT"}}
			},
			expectedCat: "", // Should definitely fail at level 10
		},
		{
			name: "DB Classification Hit",
			qid:  "Q_DB_HIT",
			setupClient: func(c *MockClient) {
				c.Claims["Q_DB_HIT"] = map[string][]string{"P31": {"Q_SOME_CLASS"}}
			},
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_SOME_CLASS"] = &model.WikidataHierarchy{
					QID:      "Q_SOME_CLASS",
					Category: "Match",
				}
			},
			expectedCat: "Match",
		},
		{
			name: "Save Failure (Graceful)",
			qid:  "Q_SAVE_FAIL",
			setupClient: func(c *MockClient) {
				c.Claims["Q_SAVE_FAIL"] = map[string][]string{"P31": {"Q_MATCH_ROOT"}}
			},
			setupStore: func(s *MockStore) {
				s.ErrorOnSave = true
			},
			expectedCat: "Match", // Should still return the result even if save fails
		},
		{
			name: "Complex Cycle (A->B->C->B)",
			qid:  "Q_COMPLEX_A",
			setupClient: func(c *MockClient) {
				c.Claims["Q_COMPLEX_A"] = map[string][]string{"P31": {"Q_COMPLEX_B"}}
				c.Claims["Q_COMPLEX_B"] = map[string][]string{"P279": {"Q_COMPLEX_C"}}
				c.Claims["Q_COMPLEX_C"] = map[string][]string{"P279": {"Q_COMPLEX_B"}}
			},
			expectedCat: "",
		},
		{
			name: "Intermediate Hierarchy Cache Hit (in scanLayerCache)",
			qid:  "Q_INTER",
			setupClient: func(c *MockClient) {
				c.Claims["Q_INTER"] = map[string][]string{"P31": {"Q_LAYER1"}}
				// Q_LAYER1 is NOT in DB, but its parent Q_LAYER2 IS in DB with a category
				c.Claims["Q_LAYER1"] = map[string][]string{"P279": {"Q_LAYER2"}}
			},
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_LAYER2"] = &model.WikidataHierarchy{
					QID:      "Q_LAYER2",
					Category: "Match",
				}
			},
			expectedCat: "Match",
		},
		{
			name: "Dead End Cache Hit",
			qid:  "Q_DEAD",
			setupClient: func(c *MockClient) {
				c.Claims["Q_DEAD"] = map[string][]string{"P31": {"Q_DEAD_CLASS"}}
			},
			setupStore: func(s *MockStore) {
				s.Classifications["Q_DEAD_CLASS"] = "__DEADEND__"
			},
			expectedCat: "",
		},
		{
			name: "Regional Category Priority",
			qid:  "Q_CONFLICT",
			setupClient: func(c *MockClient) {
				c.Claims["Q_CONFLICT"] = map[string][]string{"P31": {"Q_CONFLICT_CLASS"}}
			},
			// Q_CONFLICT_CLASS is Q_MATCH_ROOT in static, but "RegionalMatch" in regional
			setupStore: func(s *MockStore) {
				// Setup later in test body via clf.AddRegionalCategories
			},
			expectedCat: "RegionalMatch",
		},
		{
			name: "Regional Bypasses Cached Ignored Sentinel in slowPathHierarchy",
			qid:  "Q_BYPASS_SLOW_PATH",
			setupClient: func(c *MockClient) {
				c.Claims["Q_BYPASS_SLOW_PATH"] = map[string][]string{"P31": {"Q_IGN_CLASS"}}
				c.Claims["Q_IGN_CLASS"] = map[string][]string{"P279": {"Q_REGIONAL_CLASS"}}
			},
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_IGN_CLASS"] = &model.WikidataHierarchy{
					QID:      "Q_IGN_CLASS",
					Category: "__IGNORED__",
					Parents:  []string{"Q_REGIONAL_CLASS"}, // Ensure parents are returned from checkCacheOrDB
				}
				// Q_REGIONAL_CLASS will be matched regionally
			},
			expectedCat: "RegionalMatch",
		},
		{
			name: "Cached Ignored in scanLayerCache",
			qid:  "Q_CACHED_IGN",
			setupClient: func(c *MockClient) {
				c.Claims["Q_CACHED_IGN"] = map[string][]string{"P31": {"Q_IGN_NODE"}}
			},
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_IGN_NODE"] = &model.WikidataHierarchy{
					QID:      "Q_IGN_NODE",
					Category: "__IGNORED__",
				}
			},
			expectIgnored: true,
		},
		{
			name: "Dead End in scanLayerCache",
			qid:  "Q_DEAD_SCAN",
			setupClient: func(c *MockClient) {
				c.Claims["Q_DEAD_SCAN"] = map[string][]string{"P31": {"Q_DEAD_CLASS"}}
			},
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_DEAD_CLASS"] = &model.WikidataHierarchy{
					QID:      "Q_DEAD_CLASS",
					Category: "__DEADEND__",
				}
			},
			expectedCat: "",
		},
		{
			name: "Matched in scanLayerCache (Default Path)",
			qid:  "Q_MATCH_SCAN",
			setupClient: func(c *MockClient) {
				c.Claims["Q_MATCH_SCAN"] = map[string][]string{"P31": {"Q_MATCH_CLASS"}}
			},
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_MATCH_CLASS"] = &model.WikidataHierarchy{
					QID:      "Q_MATCH_CLASS",
					Category: "Match",
				}
			},
			expectedCat: "Match",
		},
		{
			name: "Slow Path API Error (P279 Failure)",
			qid:  "Q_P279_FAIL",
			setupClient: func(c *MockClient) {
				c.Claims["Q_P279_FAIL"] = map[string][]string{"P31": {"Q_BAD_CLASS"}}
				if c.ErrorOn == nil {
					c.ErrorOn = make(map[string]bool)
				}
				c.ErrorOn["Q_BAD_CLASS"] = true // P279 fetch will fail for this class
			},
			expectedCat: "",
		},
		{
			name: "Empty Parents (API Returns Empty)",
			qid:  "Q_EMPTY",
			setupClient: func(c *MockClient) {
				c.Claims["Q_EMPTY"] = map[string][]string{"P31": {}}
			},
			expectedCat: "",
		},
		{
			name: "API Error (Graceful Failure)",
			qid:  "Q_ERROR",
			setupClient: func(c *MockClient) {
				if c.ErrorOn == nil {
					c.ErrorOn = make(map[string]bool)
				}
				c.ErrorOn["Q_ERROR"] = true
			},
			expectedCat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &MockStore{
				Classifications: make(map[string]string),
				Hierarchies:     make(map[string]*model.WikidataHierarchy),
				SeenEntities:    make(map[string]bool),
			}
			cl := &MockClient{
				Claims:  make(map[string]map[string][]string),
				ErrorOn: make(map[string]bool),
			}
			tr := tracker.New()

			if tt.setupStore != nil {
				tt.setupStore(st)
			}
			if tt.setupClient != nil {
				tt.setupClient(cl)
			}

			clf := classifier.NewClassifier(st, cl, cfg, tr)

			if tt.name == "Regional Category Priority" {
				clf.AddRegionalCategories(map[string]string{"Q_CONFLICT_CLASS": "RegionalMatch"})
			} else if tt.name == "Regional Bypasses Cached Ignored Sentinel in slowPathHierarchy" {
				clf.AddRegionalCategories(map[string]string{"Q_REGIONAL_CLASS": "RegionalMatch"})
			}

			res, err := clf.Classify(context.Background(), tt.qid)
			if err != nil {
				// We expect error only if the test explicitly wants to test error bubble up,
				// but currently Classify swallows most errors with a Warn.
				// However, if we simulated ErrorOn["Q_ERROR"], Classify returns (nil, err).
				if tt.name != "API Error (Graceful Failure)" {
					t.Fatalf("Unexpected error: %v", err)
				}
			}

			actualCat := ""
			if res != nil {
				actualCat = res.Category
			}
			if actualCat != tt.expectedCat {
				t.Errorf("Expected category %q, got %q", tt.expectedCat, actualCat)
			}
		})
	}
}

func TestClassifier_BFS_Caching(t *testing.T) {
	cfg := &config.CategoriesConfig{
		IgnoredCategories: map[string]string{"Q_IGNORE": "Ignored"},
	}
	st := &MockStore{
		Classifications: make(map[string]string),
		Hierarchies:     make(map[string]*model.WikidataHierarchy),
	}
	cl := &MockClient{Claims: make(map[string]map[string][]string)}
	tr := tracker.New()
	clf := classifier.NewClassifier(st, cl, cfg, tr)

	// Setup: Q1 -> Q2 -> Q3 -> Q_IGNORE
	cl.Claims["Q1"] = map[string][]string{"P31": {"Q2"}}
	cl.Claims["Q2"] = map[string][]string{"P279": {"Q3"}}
	cl.Claims["Q3"] = map[string][]string{"P279": {"Q_IGNORE"}}

	// First run: Full discovery
	res, _ := clf.Classify(context.Background(), "Q1")
	if res == nil || !res.Ignored {
		t.Fatalf("Expected ignored, got %v", res)
	}

	// Verify propagation: Q2 and Q3 should be marked __IGNORED__ in DB
	if st.Classifications["Q2"] != "__IGNORED__" {
		t.Errorf("Q2 not propagated as ignored, got %q", st.Classifications["Q2"])
	}
	if st.Classifications["Q3"] != "__IGNORED__" {
		t.Errorf("Q3 not propagated as ignored, got %q", st.Classifications["Q3"])
	}

	// Second run: Should hit cache for Q2 immediately
	cl.SingleCalls = 0 // Reset counter
	st.GetClassCalls = 0
	cl.Claims["Q_ANOTHER"] = map[string][]string{"P31": {"Q2"}}
	res2, _ := clf.Classify(context.Background(), "Q_ANOTHER")
	if res2 == nil || !res2.Ignored {
		t.Errorf("Expected ignored on second run, got %v", res2)
	}
	if cl.SingleCalls != 1 { // Only P31 for Q_ANOTHER
		t.Errorf("Expected 1 API call (P31), got %d", cl.SingleCalls)
	}
	if st.GetClassCalls < 1 {
		t.Errorf("Expected DB classification check for Q2")
	}
}

func TestClassifier_Explain(t *testing.T) {
	// Case 1: Match
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Hike":  {QIDs: map[string]string{"Q_HIKE_ROOT": "Hike"}, Weight: 100},
			"Match": {QIDs: map[string]string{"Q_MATCH_ROOT": "Match"}, Weight: 100},
		},
		IgnoredCategories: map[string]string{"Q_IGNORE_ROOT": "Ignored"},
	}
	st := &MockStore{Classifications: make(map[string]string)}
	cl := &MockClient{
		Claims:   make(map[string]map[string][]string),
		ErrorOn:  make(map[string]bool),
		FailOnce: make(map[string]bool),
	}
	clf := classifier.NewClassifier(st, cl, cfg, tracker.New())

	clf.AddRegionalCategories(map[string]string{"Q_HIKE": "Hike"})
	cl.Claims["Q_INST"] = map[string][]string{"P31": {"Q_HIKE"}}

	exp, err := clf.Explain(context.Background(), "Q_INST")
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}
	if exp.Category != "Hike" {
		t.Errorf("Expected category Hike, got %q", exp.Category)
	}
	if exp.MatchedQID != "Q_HIKE" {
		t.Errorf("Expected matched QID Q_HIKE, got %q", exp.MatchedQID)
	}

	// Case 2: Ignored
	cl.Claims["Q_IGN_INST"] = map[string][]string{"P31": {"Q_IGN_NODE"}}
	cl.Claims["Q_IGN_NODE"] = map[string][]string{"P279": {"Q_IGNORE_ROOT"}}
	exp2, err := clf.Explain(context.Background(), "Q_IGN_INST")
	if err != nil {
		t.Fatalf("Explain (Ignored) failed: %v", err)
	}
	if !exp2.Ignored {
		t.Errorf("Expected Ignored=true for exp2")
	}

	// Case 3: No Match
	cl.Claims["Q_NONE"] = map[string][]string{"P31": {"Q_NOTHING"}}
	exp3, _ := clf.Explain(context.Background(), "Q_NONE")
	if exp3.Category != "" || exp3.Ignored {
		t.Errorf("Expected empty and not ignored for no match, got %q (ign=%v)", exp3.Category, exp3.Ignored)
	}

	// Case 4: Deep Match reason
	cl.Claims["Q_DEEP_INST"] = map[string][]string{"P31": {"Q_L1"}}
	cl.Claims["Q_L1"] = map[string][]string{"P279": {"Q_L2"}}
	cl.Claims["Q_L2"] = map[string][]string{"P279": {"Q_MATCH_ROOT"}}
	exp4, _ := clf.Explain(context.Background(), "Q_DEEP_INST")
	if exp4.Category != "Match" {
		t.Errorf("Deep match failed, got %q", exp4.Category)
	}
	if exp4.Reason == "" {
		t.Error("Expected non-empty reason for deep match")
	}
}

func TestClassifier_ClassifyBatch(t *testing.T) {
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"City":  {QIDs: map[string]string{"Q_CITY_CLASS": "City"}, Weight: 100},
			"Match": {QIDs: map[string]string{"Q_MATCH_ROOT": "Match"}, Weight: 100},
		},
		IgnoredCategories: map[string]string{"Q_IGNORE_ROOT": "Ignored"},
	}

	runBatchCase := func(name string, entities map[string]wikidata.EntityMetadata, setup func(*MockStore, *MockClient)) map[string]*model.ClassificationResult {
		st := &MockStore{
			Classifications: make(map[string]string),
			Hierarchies:     make(map[string]*model.WikidataHierarchy),
		}
		cl := &MockClient{
			Claims:   make(map[string]map[string][]string),
			ErrorOn:  make(map[string]bool),
			FailOnce: make(map[string]bool),
		}
		if setup != nil {
			setup(st, cl)
		}
		clf := classifier.NewClassifier(st, cl, cfg, tracker.New())
		return clf.ClassifyBatch(context.Background(), entities)
	}

	// Case 1: Simple match
	t.Run("Simple Match", func(t *testing.T) {
		entities := map[string]wikidata.EntityMetadata{
			"Q_1": {Claims: map[string][]string{"P31": {"Q_CITY_CLASS"}}},
			"Q_2": {Claims: map[string][]string{"P31": {"Q_UNKNOWN"}}},
		}
		results := runBatchCase("Case1", entities, nil)
		if results["Q_1"] == nil || results["Q_1"].Category != "City" {
			t.Errorf("Expected City for Q_1, got %v", results["Q_1"])
		}
		if results["Q_2"] != nil {
			t.Errorf("Expected nil for Q_2, got %v", results["Q_2"])
		}
	})

	// Case 2: Partial API failure
	t.Run("Partial API Failure", func(t *testing.T) {
		entities := map[string]wikidata.EntityMetadata{
			"Q_ERR": {Claims: map[string][]string{"P31": {"Q_ANY"}}},
		}
		results := runBatchCase("Case2", entities, func(st *MockStore, cl *MockClient) {
			cl.ErrorOn["Q_ANY"] = true
		})
		if results["Q_ERR"] != nil {
			t.Error("Expected nil for error")
		}
	})

	// Case 3: Finalize failure (save error) - Use a node NOT in lookup
	t.Run("Save Failure", func(t *testing.T) {
		entities := map[string]wikidata.EntityMetadata{
			"Q_SAVE_ERR_ART": {Claims: map[string][]string{"P31": {"Q_SAVE_ERR_CLASS"}}},
		}
		results := runBatchCase("Case3", entities, func(st *MockStore, cl *MockClient) {
			st.ErrorOnSave = true
			// Q_SAVE_ERR_CLASS -> P279 -> Q_MATCH_ROOT (Match)
			cl.Claims["Q_SAVE_ERR_CLASS"] = map[string][]string{"P279": {"Q_MATCH_ROOT"}}
		})
		// results["Q_SAVE_ERR_ART"] should be nil because finalizeMatch returned error
		if results["Q_SAVE_ERR_ART"] != nil {
			t.Errorf("Expected nil for Q_SAVE_ERR_ART when save fails, got %v", results["Q_SAVE_ERR_ART"])
		}
	})

	// Case 4: Finalize Ignored failure
	t.Run("Ignored Save Failure", func(t *testing.T) {
		entities := map[string]wikidata.EntityMetadata{
			"Q_SAVE_IGN": {Claims: map[string][]string{"P31": {"Q_IGN_NODE"}}},
		}
		results := runBatchCase("Case4", entities, func(st *MockStore, cl *MockClient) {
			st.ErrorOnSave = true
			cl.Claims["Q_IGN_NODE"] = map[string][]string{"P279": {"Q_IGNORE_ROOT"}}
		})
		// finalizeIgnored returns error on save failure, which causes classifyHierarchyNode
		// to return error, which is ignored in ClassifyBatch loop, resulting in nil.
		res := results["Q_SAVE_IGN"]
		if res != nil {
			t.Errorf("Expected nil result for Q_SAVE_IGN due to save error, got %v", res)
		}
	})

	// Case 5: Batch level API failure
	t.Run("Batch Level Error", func(t *testing.T) {
		entities := map[string]wikidata.EntityMetadata{"Q_ANY": {}}
		results := runBatchCase("Case5", entities, func(st *MockStore, cl *MockClient) {
			cl.ErrorOnBatch = true
		})
		if val, _ := results["Q_ANY"]; val != nil {
			t.Errorf("Expected nil entry for Q_ANY, got %v", val)
		}
	})
}

func TestClassifier_CachingLevels(t *testing.T) {
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Aerodrome": {
				QIDs: map[string]string{"Q62447": "Aerodrome"},
				Size: "L", Weight: 100,
			},
		},
		IgnoredCategories: map[string]string{
			"Q56061":    "Administrative Territorial Entity",
			"Q51041800": "Religious administrative entity",
		},
	}

	tests := []classifierTestCase{
		{
			name: "Cold Cache (Full Discovery)",
			qid:  "Q_COLD",
			setupClient: func(c *MockClient) {
				c.Claims["Q_COLD"] = map[string][]string{"P31": {"Q_CLASS"}}
				c.Claims["Q_CLASS"] = map[string][]string{"P279": {"Q62447"}}
			},
			expectedCat:  "Aerodrome",
			expectSingle: 2, // P31 (Q_COLD) + P279 (Q_CLASS)
			expectHier:   1, // Q_CLASS check (Structural check)
		},
		{
			name: "Fast Path Cache (Classified Class Hit)",
			qid:  "Q_FAST",
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_CLASS_KNOWN"] = &model.WikidataHierarchy{
					QID:      "Q_CLASS_KNOWN",
					Category: "Aerodrome",
				}
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_FAST"] = map[string][]string{"P31": {"Q_CLASS_KNOWN"}}
			},
			expectedCat:  "Aerodrome",
			expectSingle: 1, // P31
			expectHier:   1, // Hit Q_CLASS_KNOWN in DB
		},
		{
			name: "Negative Cache (DEADEND Hit)",
			qid:  "Q_NEG",
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_USELESS"] = &model.WikidataHierarchy{
					QID:      "Q_USELESS",
					Category: "__DEADEND__",
				}
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_NEG"] = map[string][]string{"P31": {"Q_USELESS"}}
			},
			expectedCat:  "",
			expectSingle: 1, // P31
			expectHier:   1, // Hit Q_USELESS -> "__DEADEND__"
		},
		{
			name: "Match over Ignore (Sibling Parents)",
			qid:  "Q_SIBLING",
			setupClient: func(c *MockClient) {
				// Q_SIBLING has two parents: Q_IGNORE and Q_MATCH
				c.Claims["Q_SIBLING"] = map[string][]string{"P31": {"Q_NODE"}}
				c.Claims["Q_NODE"] = map[string][]string{"P279": {"Q_IGNORE", "Q_MATCH"}}
				c.Claims["Q_IGNORE"] = map[string][]string{"P279": {"Q56061"}} // Leads to IGNORED
				c.Claims["Q_MATCH"] = map[string][]string{"P279": {"Q62447"}}  // Leads to Aerodrome
			},
			expectedCat:  "Aerodrome",
			expectSingle: 2,
			expectHier:   3,
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			runClassifierTest(t, &tests[i], cfg)
		})
	}
}

type classifierTestCase struct {
	name          string
	qid           string
	setupStore    func(*MockStore)
	setupClient   func(*MockClient)
	expectedCat   string
	expectIgnored bool
	expectSingle  int
	expectHier    int
}

func runClassifierTest(t *testing.T, tt *classifierTestCase, cfg *config.CategoriesConfig) {
	st := &MockStore{
		Classifications: make(map[string]string),
		Hierarchies:     make(map[string]*model.WikidataHierarchy),
		SeenEntities:    make(map[string]bool),
	}
	cl := &MockClient{Claims: make(map[string]map[string][]string)}
	tr := tracker.New()

	if tt.setupStore != nil {
		tt.setupStore(st)
	}
	if tt.setupClient != nil {
		tt.setupClient(cl)
	}

	clf := classifier.NewClassifier(st, cl, cfg, tr)

	res, err := clf.Classify(context.Background(), tt.qid)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if tt.expectIgnored {
		if res == nil || !res.Ignored {
			t.Errorf("Expected Ignored=true, got res=%v", res)
		}
		return
	}

	actualCat := ""
	if res != nil {
		actualCat = res.Category
	}
	if actualCat != tt.expectedCat {
		t.Errorf("Expected category %q, got %q", tt.expectedCat, actualCat)
	}

	if cl.SingleCalls != tt.expectSingle {
		t.Errorf("Expected %d Entity Single calls, got %d", tt.expectSingle, cl.SingleCalls)
	}
	if st.GetHierCalls != tt.expectHier {
		t.Errorf("Expected %d DB Hierarchy calls, got %d", tt.expectHier, st.GetHierCalls)
	}
}
