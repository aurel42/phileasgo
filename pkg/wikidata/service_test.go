package wikidata

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/model"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/request"
	"phileasgo/pkg/rescue"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
)

// Add GeodataCacheMap to mockStore to support caching tests
type mockStore struct {
	pois          map[string]*model.POI
	geodataCache  map[string][]byte // New
	geodataCacheR map[string]int    // New
}

func (m *mockStore) GetPOI(ctx context.Context, id string) (*model.POI, error) {
	return m.pois[id], nil
}
func (m *mockStore) GetPOIsBatch(ctx context.Context, ids []string) (map[string]*model.POI, error) {
	res := make(map[string]*model.POI)
	for _, id := range ids {
		if p, ok := m.pois[id]; ok {
			res[id] = p
		}
	}
	return res, nil
}
func (m *mockStore) SavePOI(ctx context.Context, p *model.POI) error { return nil }
func (m *mockStore) GetRecentlyPlayedPOIs(ctx context.Context, since time.Time) ([]*model.POI, error) {
	return nil, nil
}
func (m *mockStore) ResetLastPlayed(ctx context.Context, lat, lon, radius float64) error { return nil }
func (m *mockStore) GetCache(ctx context.Context, key string) ([]byte, bool)             { return nil, false }
func (m *mockStore) HasCache(ctx context.Context, key string) (bool, error)              { return false, nil }
func (m *mockStore) SetCache(ctx context.Context, key string, val []byte) error          { return nil }
func (m *mockStore) ListCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

// Updated GetGeodataCache implementation
func (m *mockStore) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	if val, ok := m.geodataCache[key]; ok {
		return val, m.geodataCacheR[key], true
	}
	return nil, 0, false
}
func (m *mockStore) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	if m.geodataCache == nil {
		m.geodataCache = make(map[string][]byte)
		m.geodataCacheR = make(map[string]int)
	}
	m.geodataCache[key] = val
	m.geodataCacheR[key] = radius
	return nil
}
func (m *mockStore) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]store.GeodataRecord, error) {
	return nil, nil
}
func (m *mockStore) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}
func (m *mockStore) GetState(ctx context.Context, key string) (string, bool) { return "", false }
func (m *mockStore) SetState(ctx context.Context, key, val string) error     { return nil }
func (m *mockStore) SaveMSFSPOI(ctx context.Context, p *model.MSFSPOI) error { return nil }
func (m *mockStore) GetMSFSPOI(ctx context.Context, id int64) (*model.MSFSPOI, error) {
	return nil, nil
}
func (m *mockStore) GetClassification(ctx context.Context, id string) (category string, found bool, err error) {
	return "", false, nil
}
func (m *mockStore) SaveClassification(ctx context.Context, id, cat string, parents []string, name string) error {
	return nil
}
func (m *mockStore) GetHierarchy(ctx context.Context, id string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *mockStore) SaveHierarchy(ctx context.Context, h *model.WikidataHierarchy) error { return nil }
func (m *mockStore) SaveArticle(ctx context.Context, a *model.Article) error             { return nil }
func (m *mockStore) GetArticle(ctx context.Context, uuid string) (*model.Article, error) {
	return nil, nil
}
func (m *mockStore) GetSeenEntitiesBatch(ctx context.Context, qids []string) (map[string][]string, error) {
	return make(map[string][]string), nil
}
func (m *mockStore) CheckMSFSPOI(ctx context.Context, lat, lon, radius float64) (bool, error) {
	return false, nil
}
func (m *mockStore) MarkEntitiesSeen(ctx context.Context, entities map[string][]string) error {
	return nil
}
func (m *mockStore) Close() error { return nil }

type MockClassifier struct {
	ClassifyFunc      func(ctx context.Context, qid string) (*model.ClassificationResult, error)
	ClassifyBatchFunc func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult
}

func (m *MockClassifier) Classify(ctx context.Context, qid string) (*model.ClassificationResult, error) {
	if m.ClassifyFunc != nil {
		return m.ClassifyFunc(ctx, qid)
	}
	return nil, nil
}

func (m *MockClassifier) ClassifyBatch(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
	if m.ClassifyBatchFunc != nil {
		return m.ClassifyBatchFunc(ctx, entities)
	}
	return nil
}

func (m *MockClassifier) GetConfig() *config.CategoriesConfig {
	return &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"City":   {SitelinksMin: 5},
			"Church": {SitelinksMin: 5},
		},
	}
}

// MockWikidataClient implements ClientInterface for testing
type MockWikidataClient struct {
	QuerySPARQLFunc       func(ctx context.Context, query, cacheKey string, radiusM int, lat, lon float64) ([]Article, string, error)
	QueryEntitiesFunc     func(ctx context.Context, ids []string) ([]Article, string, error)
	GetEntitiesBatchFunc  func(ctx context.Context, ids []string) (map[string]EntityMetadata, error)
	FetchFallbackDataFunc func(ctx context.Context, ids []string, allowedSites []string) (map[string]FallbackData, error)
	GetEntityClaimsFunc   func(ctx context.Context, id, property string) (targets []string, label string, err error)
}

func (m *MockWikidataClient) QuerySPARQL(ctx context.Context, query, cacheKey string, radiusM int, lat, lon float64) ([]Article, string, error) {
	if m.QuerySPARQLFunc != nil {
		return m.QuerySPARQLFunc(ctx, query, cacheKey, radiusM, lat, lon)
	}
	return nil, "[]", nil
}

func (m *MockWikidataClient) QueryEntities(ctx context.Context, ids []string) ([]Article, string, error) {
	if m.QueryEntitiesFunc != nil {
		return m.QueryEntitiesFunc(ctx, ids)
	}
	return nil, "[]", nil
}

func (m *MockWikidataClient) GetEntitiesBatch(ctx context.Context, ids []string) (map[string]EntityMetadata, error) {
	if m.GetEntitiesBatchFunc != nil {
		return m.GetEntitiesBatchFunc(ctx, ids)
	}
	return make(map[string]EntityMetadata), nil
}

func (m *MockWikidataClient) FetchFallbackData(ctx context.Context, ids []string, allowedSites []string) (map[string]FallbackData, error) {
	if m.FetchFallbackDataFunc != nil {
		return m.FetchFallbackDataFunc(ctx, ids, allowedSites)
	}
	return make(map[string]FallbackData), nil
}

func (m *MockWikidataClient) GetEntityClaims(ctx context.Context, id, property string) (targets []string, label string, err error) {
	if m.GetEntityClaimsFunc != nil {
		return m.GetEntityClaimsFunc(ctx, id, property)
	}
	return nil, "", nil
}

// MockWikipediaProvider implements WikipediaProvider for testing
type MockWikipediaProvider struct {
	GetArticleLengthsFunc func(ctx context.Context, titles []string, lang string) (map[string]int, error)
	GetArticleContentFunc func(ctx context.Context, title, lang string) (string, error)
	GetArticleHTMLFunc    func(ctx context.Context, title, lang string) (string, error)
}

func (m *MockWikipediaProvider) GetArticleLengths(ctx context.Context, titles []string, lang string) (map[string]int, error) {
	if m.GetArticleLengthsFunc != nil {
		return m.GetArticleLengthsFunc(ctx, titles, lang)
	}
	return make(map[string]int), nil
}

func (m *MockWikipediaProvider) GetArticleContent(ctx context.Context, title, lang string) (string, error) {
	if m.GetArticleContentFunc != nil {
		return m.GetArticleContentFunc(ctx, title, lang)
	}
	return "", nil
}

func (m *MockWikipediaProvider) GetArticleHTML(ctx context.Context, title, lang string) (string, error) {
	if m.GetArticleHTMLFunc != nil {
		return m.GetArticleHTMLFunc(ctx, title, lang)
	}
	return "", nil
}

// Minimal stub for sim
type mockSim struct{}

func (m *mockSim) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	return sim.Telemetry{}, nil
}
func (m *mockSim) GetState() sim.State                      { return sim.StateActive } // Important for service.Start/Tick
func (m *mockSim) GetLastTransition(stage string) time.Time { return time.Time{} }
func (m *mockSim) SetPredictionWindow(d time.Duration)      {}

func TestFetchTile_CacheOptimization(t *testing.T) {
	// Setup Store with Cached Data
	cachedJSON := `{"results":{"bindings":[
		{"item":{"value":"http://wd.org/Q_CACHED"}, "lat":{"value":"52.0"}, "lon":{"value":"13.0"}, "sitelinks":{"value":"100"}}
	]}}`
	st := &mockStore{
		pois:          make(map[string]*model.POI),
		geodataCache:  map[string][]byte{"wd_h3_891f1a48c6bffff": []byte(cachedJSON)},
		geodataCacheR: map[string]int{"wd_h3_891f1a48c6bffff": 5000},
	}

	// Mock Client - Should fail if called!
	mockClient := &MockWikidataClient{
		QuerySPARQLFunc: func(ctx context.Context, query, cacheKey string, radiusM int, lat, lon float64) ([]Article, string, error) {
			t.Fatal("QuerySPARQL should NOT be called when cache is present")
			return nil, "", nil
		},
		GetEntitiesBatchFunc: func(ctx context.Context, ids []string) (map[string]EntityMetadata, error) {
			return make(map[string]EntityMetadata), nil
		},
		FetchFallbackDataFunc: func(ctx context.Context, ids []string, allowedSites []string) (map[string]FallbackData, error) {
			// Provide fallback data logic here if ProcessTileData needs it for Q_CACHED
			// We can return simple data so processing succeeds
			return map[string]FallbackData{
				"Q_CACHED": {Labels: map[string]string{"en": "Cached POI"}},
			}, nil
		},
	}

	geoSvc, err := geo.NewService("../../data/cities1000.txt", "../../data/admin1CodesASCII.txt")
	if err != nil {
		t.Fatalf("Failed to create geo service: %v", err)
	}

	svc := NewService(st, &mockSim{}, tracker.New(), &MockClassifier{}, request.New(st, tracker.New(), request.ClientConfig{
		Retries:   2,
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  50 * time.Millisecond,
	}), geoSvc, poi.NewManager(&config.Config{}, st, nil), config.WikidataConfig{Area: config.AreaConfig{MaxDist: 100}}, "en")
	svc.client = mockClient
	svc.classifier = &MockClassifier{
		ClassifyBatchFunc: func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
			return map[string]*model.ClassificationResult{"Q_CACHED": {Category: "City"}}
		},
	}
	// We need to inject the mock wiki client explicitly since NewService creates a real one
	svc.wiki = &MockWikipediaProvider{}

	// Create Candidate that matches the cache key (index 891f1a48c6bffff)
	// We need a valid HexTile.
	// Since NewService initializes scheduler/grid internally, we can use the scheduler to help or just construct manually.
	// But `fetchTile` is private. We can't call it directly from this test package unless we export it or put test in same package.
	// We are in `package wikidata`, so we can call private methods!

	candidate := Candidate{
		Tile: HexTile{Index: "891f1a48c6bffff"},
		Dist: 5.0,
	}

	// Execute FetchTile (Private Method)
	svc.fetchTile(context.Background(), candidate, rescue.MedianStats{})

	// Since we didn't crash and didn't call QuerySPARQL (fatal mock), test passes.
}

func TestProcessTileData(t *testing.T) {
	// Base Mocks
	st := &mockStore{
		pois: map[string]*model.POI{
			"Q_KNOWN": {WikidataID: "Q_KNOWN", Category: "Castle"},
		},
	}
	cl := &MockClassifier{
		ClassifyFunc: func(ctx context.Context, qid string) (*model.ClassificationResult, error) {
			if qid == "Q2" {
				return &model.ClassificationResult{Category: "City"}, nil
			}
			return &model.ClassificationResult{Category: "Unknown"}, nil
		},
	}

	// Table Driven Test
	tests := []struct {
		name         string
		rawJSON      string
		fallbackData map[string]FallbackData
		fallbackErr  error
		forceDesc    bool
		wantArticles int
		wantRescued  int
		expectError  bool
	}{
		{
			name: "Success - Cheap Query + Hydration",
			// Raw JSON mimics Cheap Query response (No Labels, No Titles)
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q2"}, "lat":{"value":"52.5"}, "lon":{"value":"13.4"}, "sitelinks":{"value":"10"}, "instances":{"value":"http://wd.org/P31/Q515"}}
			]}}`,
			fallbackData: map[string]FallbackData{
				"Q2": {
					Labels:    map[string]string{"en": "Berlin"},
					Sitelinks: map[string]string{"enwiki": "Berlin"},
				},
			},
			wantArticles: 1,
			wantRescued:  0,
			expectError:  false,
		},
		{
			name:         "Empty Results",
			rawJSON:      `{"results":{"bindings":[]}}`,
			wantArticles: 0,
			wantRescued:  0,
			expectError:  false,
		},
		{
			name: "Hydration Failure",
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q2"}, "lat":{"value":"52.5"}, "lon":{"value":"13.4"}, "sitelinks":{"value":"10"}}
			]}}`,
			fallbackErr: context.DeadlineExceeded,
			expectError: false, // Dropped silently before hydration because no instances
		},
		{
			name: "Filtered Existing POI",
			// Q_KNOWN is in the store
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q_KNOWN"}, "lat":{"value":"50.0"}, "lon":{"value":"10.0"}, "sitelinks":{"value":"50"}}
			]}}`,
			wantArticles: 0,
			expectError:  false,
		},
		{
			name: "Reprocess Force=True",
			// Even if filtered or seen, force=true should process it (though mock store behavior for filter is static above).
			// We check if logic flows through.
			// Currently filterExisting is hardcoded in test setup.
			// Let's use a new QID for 'seen' check if we had a seen store.
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q3"}, "lat":{"value":"52.0"}, "lon":{"value":"13.0"}, "sitelinks":{"value":"5"}, "instances":{"value":"http://wd.org/P31/Q515"}}
			]}}`,
			fallbackData: map[string]FallbackData{
				"Q3": {Labels: map[string]string{"en": "Potsdam"}},
			},
			forceDesc:    true,
			wantArticles: 1,
			expectError:  false,
		},
		{
			name: "Partial Hydration (Missing Entity)",
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q2"}, "lat":{"value":"52.5"}, "lon":{"value":"13.4"}, "sitelinks":{"value":"10"}, "instances":{"value":"http://wd.org/P31/Q515"}},
				{"item":{"value":"http://wd.org/Q999"}, "lat":{"value":"52.5"}, "lon":{"value":"13.4"}, "sitelinks":{"value":"10"}, "instances":{"value":"http://wd.org/P31/Q515"}}
			]}}`,
			fallbackData: map[string]FallbackData{
				"Q2": {
					Labels:    map[string]string{"en": "Berlin"},
					Sitelinks: map[string]string{"enwiki": "Berlin"},
				},
				// Q999 is missing from fallback
			},
			wantArticles: 1, // Only Q2 survives
			wantRescued:  0,
			expectError:  false,
		},
		{
			name: "Classification Persistence - No Metadata Fetch",
			// This test ensures that if an article has "instances" in SPARQL JSON,
			// it is classified correctly even if we do NOT fetch anything from API (or if API fetch is skipped).
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q2"}, "lat":{"value":"52.5"}, "lon":{"value":"13.4"}, "sitelinks":{"value":"10"}, "instances":{"value":"http://wd.org/P31/Q515"}}
			]}}`,
			// Fallback Data IS needed for title/hydration
			fallbackData: map[string]FallbackData{
				"Q2": {Labels: map[string]string{"en": "Berlin"}},
			},
			wantArticles: 1,
			expectError:  false,
		},
		{
			name: "Drop - No Instances and No Dimensions",
			// No instances in SPARQL, No dimensions. Should be dropped.
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q4"}, "lat":{"value":"52.0"}, "lon":{"value":"13.0"}, "sitelinks":{"value":"10"}}
			]}}`,
			fallbackData: map[string]FallbackData{
				"Q4": {Labels: map[string]string{"en": "Ghost POI"}},
			},
			wantArticles: 0,
			expectError:  false,
		},
		{
			name: "Rescue - Dimension Rescue (Height)",
			// No instances, but has height. Should be rescued.
			rawJSON: `{"results":{"bindings":[
				{"item":{"value":"http://wd.org/Q5"}, "lat":{"value":"52.0"}, "lon":{"value":"13.0"}, "sitelinks":{"value":"10"}, "height":{"value":"100"}}
			]}}`,
			fallbackData: map[string]FallbackData{
				"Q5": {
					Labels:    map[string]string{"en": "Tall Tower"},
					Sitelinks: map[string]string{"enwiki": "Tall Tower"},
				},
			},
			wantArticles: 1,
			wantRescued:  1,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Mock Client
			mockClient := &MockWikidataClient{
				FetchFallbackDataFunc: func(ctx context.Context, ids []string, allowedSites []string) (map[string]FallbackData, error) {
					if tt.fallbackErr != nil {
						return nil, tt.fallbackErr
					}
					return tt.fallbackData, nil
				},
				// Needed for classification step
				GetEntitiesBatchFunc: func(ctx context.Context, ids []string) (map[string]EntityMetadata, error) {
					// Return dummy metadata so classification has something to work with
					res := make(map[string]EntityMetadata)
					for _, id := range ids {
						res[id] = EntityMetadata{
							Claims: map[string][]string{"P31": {"Q515"}}, // City
						}
					}
					return res, nil
				},
			}

			// Update ClassifyBatchFunc on the specific classifier instance for this test run needed?
			// The classifier `cl` is shared across subtests. We need to set the func per test or make it robust.
			// Let's set it globally for `cl` but it needs to match expectations.
			cl.ClassifyBatchFunc = func(ctx context.Context, entities map[string]EntityMetadata) map[string]*model.ClassificationResult {
				res := make(map[string]*model.ClassificationResult)
				for qid := range entities {
					// Simple mock logic: If strict check needed, use qid
					if qid == "Q2" || qid == "Q3" || qid == "Q999" {
						res[qid] = &model.ClassificationResult{Category: "City"}
					} else {
						res[qid] = &model.ClassificationResult{Category: "Unknown"}
					}
				}
				return res
			}

			// Construct Pipeline directly
			pl := NewPipeline(
				st,
				mockClient,
				&MockWikipediaProvider{},
				&geo.Service{}, // Geo
				poi.NewManager(&config.Config{}, st, nil), // POI
				NewGrid(), // Grid (extracted from new scheduler)
				NewLanguageMapper(st, nil, slog.Default()), // Mapper
				cl,
				config.WikidataConfig{},
				slog.Default(),
				"en",
			)

			// EXECUTE
			gotArticles, _, gotRescued, err := pl.ProcessTileData(context.Background(), []byte(tt.rawJSON), 0, 0, tt.forceDesc, rescue.MedianStats{})

			// ASSERT
			if (err != nil) != tt.expectError {
				t.Errorf("ProcessTileData() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if len(gotArticles) != tt.wantArticles {
				t.Errorf("ProcessTileData() returned %d articles, want %d", len(gotArticles), tt.wantArticles)
			}
			if gotRescued != tt.wantRescued {
				t.Errorf("ProcessTileData() rescued = %d, want %d", gotRescued, tt.wantRescued)
			}
		})
	}
}

func TestBuildCheapQuery(t *testing.T) {
	got := buildCheapQuery(52.5, 13.4, "10.0")

	// Verify core components of the Cheap Query
	if !strings.Contains(got, `?item wdt:P625 ?location`) {
		t.Errorf("Query missing location service")
	}
	if !strings.Contains(got, `wikibase:radius "10.0"`) {
		t.Errorf("Query missing correct radius")
	}

	// Verify ABSENCE of expensive fields
	if strings.Contains(got, `SERVICE wikibase:label`) {
		t.Errorf("Cheap Query should NOT contain label service")
	}
	if strings.Contains(got, `VALUES ?lang`) {
		t.Errorf("Cheap Query should NOT contain language VALUES clause")
	}

	// Verify captured fields
	if !strings.Contains(got, `?item wikibase:sitelinks ?sitelinks`) {
		t.Errorf("Query missing sitelinks selection")
	}

	// Case 2: Default Radius
	gotDefault := buildCheapQuery(52.5, 13.4, "")
	if !strings.Contains(gotDefault, `wikibase:radius "9.8"`) {
		t.Errorf("Cheap Query fallback radius expected 9.8, got %s", gotDefault)
	}
}
func TestGetNeighborhoodStats(t *testing.T) {
	svc := &Service{
		recentTiles: make(map[string]TileWrapper),
		cfg:         config.WikidataConfig{Rescue: config.RescueConfig{PromoteByDimension: config.PromoteByDimensionConfig{RadiusKM: 20}}},
	}

	// Grid setup for coordinates
	svc.scheduler = &Scheduler{grid: NewGrid()}

	// Center point for T1
	tile := HexTile{Index: "891f1a48c6bffff"}
	centerLat, centerLon := svc.gridCenter(tile)

	// T1 is the tile itself
	svc.recentTiles[tile.Index] = TileWrapper{
		Stats: rescue.TileStats{Lat: centerLat, Lon: centerLon, MaxHeight: 100},
	}
	// T2 is far
	svc.recentTiles["8928308280fffff"] = TileWrapper{
		Stats: rescue.TileStats{Lat: 40.0, Lon: -74.0, MaxHeight: 500},
	}

	medians := svc.getNeighborhoodStats(tile)

	// T2 should be ignored because it's too far (radius 20km)
	if medians.MedianHeight != 100 {
		t.Errorf("Expected MedianHeight 100, got %f", medians.MedianHeight)
	}
}

func TestUpdateTileStats(t *testing.T) {
	svc := &Service{
		recentTiles: make(map[string]TileWrapper),
	}
	h100 := 100.0
	articles := []Article{
		{QID: "Q1", Height: &h100},
	}

	svc.updateTileStats("T_NEW", 50.0, 10.0, articles)

	if stats, ok := svc.recentTiles["T_NEW"]; !ok {
		t.Error("Tile stats not saved")
	} else if stats.Stats.MaxHeight != 100 {
		t.Errorf("Expected MaxHeight 100, got %f", stats.Stats.MaxHeight)
	}

	// Test case: Ignored articles should be excluded
	articlesWithIgnored := []Article{
		{QID: "Q1", Height: &h100},
		{QID: "Q_LARGE", Height: func() *float64 { f := 5000.0; return &f }(), Ignored: true},
	}
	svc.updateTileStats("T_IGNORED", 50.0, 10.0, articlesWithIgnored)
	if stats, ok := svc.recentTiles["T_IGNORED"]; !ok {
		t.Error("Tile stats not saved for T_IGNORED")
	} else if stats.Stats.MaxHeight != 100 {
		t.Errorf("Expected MaxHeight 100 (excluding Ignored 5000), got %f", stats.Stats.MaxHeight)
	}
}
