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
