package classifier_test

import (
	"context"
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
}

func (m *MockClient) GetEntityClaims(ctx context.Context, id, property string) (targets []string, label string, err error) {
	m.SingleCalls++
	if props, ok := m.Claims[id]; ok {
		if t, ok := props[property]; ok {
			return t, "Label: " + id, nil
		}
	}
	return nil, "Label: " + id, nil
}

func (m *MockClient) GetEntityClaimsBatch(ctx context.Context, ids []string, property string) (claims map[string][]string, labels map[string]string, err error) {
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
	Classifications map[string]string
	Hierarchies     map[string]*model.WikidataHierarchy
	SeenEntities    map[string]bool
	GetClassCalls   int
	GetHierCalls    int
	GetSeenCalls    int
}

func (m *MockStore) GetClassification(ctx context.Context, qid string) (category string, found bool, err error) {
	m.GetClassCalls++
	if val, ok := m.Classifications[qid]; ok {
		return val, true, nil
	}
	return "", false, nil
}

func (m *MockStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	m.Classifications[qid] = category
	return nil
}

func (m *MockStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	m.GetHierCalls++
	if h, ok := m.Hierarchies[qid]; ok {
		return h, nil
	}
	return nil, nil // Return nil on miss to match real store behavior for hierarchy
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
func (m *MockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int) error {
	return nil
}
func (m *MockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *MockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *MockStore) SaveArticle(ctx context.Context, a *model.Article) error { return nil }
func (m *MockStore) Close() error                                            { return nil }

func TestClassifier_CachingLevels(t *testing.T) {
	cfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Aerodrome": {
				QIDs: map[string]string{"Q62447": "Aerodrome"},
				Size: "L", Weight: 100,
			},
		},
		IgnoredCategories: map[string]string{
			"Q56061": "Administrative Territorial Entity",
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
			expectedCat:   "Aerodrome",
			expectSingle:  2, // P31 (Single) + P279 for Q_CLASS (Single)
			expectHBatch:  0, // Matched immediately in subclasses
			expectDBClass: 1, // Miss Q_CLASS
			expectDBHier:  1, // slowPathHierarchy now checks DB first (miss expected)
		},
		{
			name: "Deep Hierarchy (Verifies BFS Batching)",
			qid:  "Q_DEEP",
			setupClient: func(c *MockClient) {
				c.Claims["Q_DEEP"] = map[string][]string{"P31": {"Q_C1"}}
				c.Claims["Q_C1"] = map[string][]string{"P279": {"Q_C2"}}
				c.Claims["Q_C2"] = map[string][]string{"P279": {"Q62447"}}
			},
			expectedCat:   "Aerodrome",
			expectSingle:  2, // P31 for Q_DEEP, P279 for Q_C1
			expectHBatch:  1, // BFS fetch for Q_C2
			expectDBClass: 1, // Miss Q_C1
			expectDBHier:  2, // 1 for Q_C1 in slowPathHierarchy, 1 for Q_C2 in BFS checkCacheOrDB
		},
		{
			name: "Fast Path Cache (Classified Class Hit)",
			qid:  "Q_FAST",
			setupStore: func(s *MockStore) {
				s.Classifications["Q_CLASS_KNOWN"] = "Aerodrome"
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_FAST"] = map[string][]string{"P31": {"Q_CLASS_KNOWN"}}
			},
			expectedCat:   "Aerodrome",
			expectSingle:  1, // P31
			expectHBatch:  0, // Handled by Fast Path
			expectDBClass: 1, // Hit Q_CLASS_KNOWN
			expectDBHier:  0,
		},
		{
			name: "Slow Path Cache (Intermediate Hierarchy Hit)",
			qid:  "Q_SLOW",
			setupStore: func(s *MockStore) {
				s.Hierarchies["Q_INTER"] = &model.WikidataHierarchy{
					QID:     "Q_INTER",
					Parents: []string{"Q62447"},
				}
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_SLOW"] = map[string][]string{"P31": {"Q_INTER"}}
			},
			expectedCat:   "Aerodrome",
			expectSingle:  1, // P31
			expectHBatch:  0, // Found in Hierarchy DB
			expectDBClass: 1, // Miss Q_INTER class
			expectDBHier:  1, // Hit Q_INTER hierarchy
		},
		{
			name: "Negative Cache (Unclassified Class Hit - Falls Through)",
			qid:  "Q_NEG",
			setupStore: func(s *MockStore) {
				s.Classifications["Q_USELESS"] = "" // Explicitly unclassified
				// We must provide parents to support the fall-through traversal
				s.Hierarchies["Q_USELESS"] = &model.WikidataHierarchy{
					QID:     "Q_USELESS",
					Parents: []string{}, // No parents, so it will eventually fail matching
				}
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_NEG"] = map[string][]string{"P31": {"Q_USELESS"}}
			},
			expectedCat:   "",
			expectSingle:  1, // P31
			expectHBatch:  0,
			expectDBClass: 1, // Hit Q_USELESS -> ""
			expectDBHier:  1, // Now falls through to check hierarchy of Q_USELESS
		},
		{
			name: "ClassifyBatch (Bulk Efficiency)",
			qids: []string{"B1", "B2"},
			setupClient: func(c *MockClient) {
				c.Claims["B1"] = map[string][]string{"P31": {"Q62447"}}
				c.Claims["B2"] = map[string][]string{"P31": {"Q62447"}}
			},
			expectedCats:  map[string]string{"B1": "Aerodrome", "B2": "Aerodrome"},
			expectBatch:   0, // Metadata is provided, not fetched
			expectSingle:  0, // Handled by batcher metadata
			expectHBatch:  0, // Direct match via P31
			expectDBClass: 0,
			expectDBHier:  0,
		},
		{
			name: "Ignored Category (Direct Subclass)",
			qid:  "Q_IGNORED",
			setupClient: func(c *MockClient) {
				c.Claims["Q_IGNORED"] = map[string][]string{"P31": {"Q_ADMIN"}}
				c.Claims["Q_ADMIN"] = map[string][]string{"P279": {"Q56061"}} // Q56061 is in ignored
			},
			expectIgnored: true,
			expectSingle:  2, // P31 + P279
			expectDBClass: 1, // Miss Q_ADMIN
			expectDBHier:  1, // slowPathHierarchy DB check
		},
		{
			name: "Ignored Category (Cached Explicit)",
			qid:  "Q_CACHED_IGNORED",
			setupStore: func(s *MockStore) {
				s.Classifications["Q_CACHED_CLASS"] = "__IGNORED__" // NOW must be explicit
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_CACHED_IGNORED"] = map[string][]string{"P31": {"Q_CACHED_CLASS"}}
			},
			expectIgnored: true,
			expectSingle:  1, // P31 only
			expectDBClass: 1, // Hit Q_CACHED_CLASS -> "__IGNORED__"
			expectDBHier:  0,
		},
		{
			name: "Ignored Category (Legacy Stale String)",
			qid:  "Q_STALE",
			setupStore: func(s *MockStore) {
				s.Classifications["Q_STALE_CLASS"] = "__IGNORED__"
			},
			setupClient: func(c *MockClient) {
				c.Claims["Q_STALE"] = map[string][]string{"P31": {"Q_STALE_CLASS"}}
			},
			expectIgnored: true,
			expectSingle:  1, // P31 only
			expectDBClass: 1, // Hit Q_STALE_CLASS -> "__IGNORED__"
			expectDBHier:  0,
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
	qids          []string
	setupStore    func(*MockStore)
	setupClient   func(*MockClient)
	expectedCat   string
	expectedCats  map[string]string
	expectIgnored bool
	expectBatch   int
	expectSingle  int
	expectHBatch  int
	expectDBClass int
	expectDBHier  int
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

	if tt.qids != nil {
		runBatchTest(t, tt, cl, clf)
	} else {
		runSingleTest(t, tt, clf)
	}

	verifyCalls(t, tt, cl, st)
}

func runBatchTest(t *testing.T, tt *classifierTestCase, cl *MockClient, clf *classifier.Classifier) {
	meta, _ := cl.GetEntitiesBatch(context.Background(), tt.qids)
	cl.BatchEntities = 0
	results := clf.ClassifyBatch(context.Background(), meta)
	for qid, expected := range tt.expectedCats {
		actual := ""
		if results[qid] != nil {
			actual = results[qid].Category
		}
		if actual != expected {
			t.Errorf("QID %s: Expected %q, got %q", qid, expected, actual)
		}
	}
}

func runSingleTest(t *testing.T, tt *classifierTestCase, clf *classifier.Classifier) {
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
}

func verifyCalls(t *testing.T, tt *classifierTestCase, cl *MockClient, st *MockStore) {
	if cl.BatchEntities != tt.expectBatch {
		t.Errorf("Expected %d Entity Batch calls, got %d", tt.expectBatch, cl.BatchEntities)
	}
	if cl.SingleCalls != tt.expectSingle {
		t.Errorf("Expected %d Entity Single calls, got %d", tt.expectSingle, cl.SingleCalls)
	}
	if cl.BatchCalls != tt.expectHBatch {
		t.Errorf("Expected %d Hierarchy Batch calls, got %d", tt.expectHBatch, cl.BatchCalls)
	}
	if st.GetClassCalls != tt.expectDBClass {
		t.Errorf("Expected %d DB Classification calls, got %d", tt.expectDBClass, st.GetClassCalls)
	}
	if st.GetHierCalls != tt.expectDBHier {
		t.Errorf("Expected %d DB Hierarchy calls, got %d", tt.expectDBHier, st.GetHierCalls)
	}
}
