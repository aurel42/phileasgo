package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/model"
	"phileasgo/pkg/request"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/wikidata"
	"sync"
	"testing"
	"time"
)

type mockLLM struct {
	responses map[string]string // profile -> json
	calls     []string
}

func (m *mockLLM) GenerateText(ctx context.Context, profile, prompt string) (string, error) {
	m.calls = append(m.calls, profile)
	return m.responses[profile], nil
}

func (m *mockLLM) GenerateJSON(ctx context.Context, profile, prompt string, out any) error {
	m.calls = append(m.calls, profile)
	resp, ok := m.responses[profile]
	if !ok {
		return nil
	}
	return json.Unmarshal([]byte(resp), out)
}

func (m *mockLLM) GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error) {
	return "", nil
}

func (m *mockLLM) HasProfile(profile string) bool {
	return profile == "regional_categories_ontological" || profile == "regional_categories_topographical"
}

func (m *mockLLM) ValidateModels(ctx context.Context) error { return nil }

type mockTransport struct {
	responses map[string]string // search term -> full json response
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query().Get("search")
	if q == "" {
		q = req.URL.Query().Get("ids")
	}
	resp, ok := m.responses[q]
	if !ok {
		// Fallback to a default or empty if not found
		if req.URL.Query().Get("action") == "wbgetentities" {
			resp = `{"entities": {}}`
		} else {
			resp = `{"search": []}`
		}
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(resp))),
		Header:     make(http.Header),
	}, nil
}

type mockHierarchyStore struct {
	store.Store
}

func (m *mockHierarchyStore) GetClassification(ctx context.Context, qid string) (string, bool, error) {
	return "", false, nil
}
func (m *mockHierarchyStore) GetHierarchy(ctx context.Context, qid string) (*model.WikidataHierarchy, error) {
	return nil, nil
}
func (m *mockHierarchyStore) GetRegionalCategories(ctx context.Context, latGrid, lonGrid int) (map[string]string, map[string]string, error) {
	return nil, nil, nil
}
func (m *mockHierarchyStore) SaveClassification(ctx context.Context, qid, category string, parents []string, label string) error {
	return nil
}
func (m *mockHierarchyStore) SaveRegionalCategories(ctx context.Context, latGrid, lonGrid int, categories map[string]string, labels map[string]string) error {
	return nil
}

func setupJob(t *testing.T, llmResp map[string]string, transport http.RoundTripper) (*RegionalCategoriesJob, *classifier.Classifier, *mockSpatialStore) {
	st := &mockSpatialStore{
		cats:   make(map[string]map[string]string),
		labels: make(map[string]map[string]string),
	}
	tr := tracker.New()
	catCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Sights":   {Weight: 50},
			"Shopping": {Weight: 30},
			"Mountain": {Weight: 40},
		},
	}
	clf := classifier.NewClassifier(st, nil, catCfg, tr)
	dummyCfg := config.NewProvider(&config.Config{
		Wikidata: config.WikidataConfig{
			Area: config.AreaConfig{MaxDist: 80000},
		},
	}, nil)

	mLLM := &mockLLM{responses: llmResp}
	promptDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(promptDir, "context"), 0755)
	_ = os.WriteFile(filepath.Join(promptDir, "context", "ontological.tmpl"), []byte("onto"), 0644)
	_ = os.WriteFile(filepath.Join(promptDir, "context", "topographical.tmpl"), []byte("topo"), 0644)
	pm, _ := prompts.NewManager(promptDir)

	cityFile := filepath.Join(t.TempDir(), "cities.txt")
	adminFile := filepath.Join(t.TempDir(), "admin.txt")
	// Need at least one valid-ish line to avoid NewService returning nil
	_ = os.WriteFile(cityFile, []byte("1\tCity\tCity\t\t50\t10\t\t\tUS\t\t01\t\t\t\t1000\t\t\t\tEurope/London\t\n"), 0644)
	_ = os.WriteFile(adminFile, []byte("US.01\tAlabama\tAlabama\t1234\n"), 0644)

	geoSvc, err := geo.NewService(cityFile, adminFile)
	if err != nil {
		t.Fatalf("Failed to create geo service: %v", err)
	}

	reqClient := request.New(nil, tr, request.ClientConfig{})
	if transport != nil {
		reqClient.SetTransport(transport)
	}
	wikiCl := wikidata.NewClient(reqClient, nil)
	validator := wikidata.NewValidator(wikiCl)
	wikiSvc := &wikidata.Service{}

	job := NewRegionalCategoriesJob(dummyCfg, mLLM, pm, validator, clf, geoSvc, wikiSvc, st)
	return job, clf, st
}

func waitJob(job *RegionalCategoriesJob) {
	for i := 0; i < 100; i++ {
		if job.TryLock() {
			job.Unlock()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRegionalCategoriesJob_Merging(t *testing.T) {
	job, clf, _ := setupJob(t, nil, nil)

	// Pre-seed with one category
	clf.AddRegionalCategories(map[string]string{"Q1": "Cat1"}, nil)

	// Simulate discovery of another one
	job.classifier.AddRegionalCategories(map[string]string{"Q2": "Cat2"}, nil)

	res := clf.GetRegionalCategories()
	if len(res) != 2 {
		t.Errorf("Expected 2 categories after merging, got %d", len(res))
	}
	if res["Q1"] != "Cat1" || res["Q2"] != "Cat2" {
		t.Errorf("Merging failed: %v", res)
	}

	// Reset
	job.ResetSession(context.Background())
	if len(clf.GetRegionalCategories()) != 0 {
		t.Error("ResetSession should clear regional categories in classifier")
	}
}

func TestRegionalCategoriesJob_Pipeline(t *testing.T) {
	tests := []struct {
		name              string
		ontologicalResp   string
		topographicalResp string
		wikidataResps     map[string]string // Search Term -> JSON
		expectedCats      map[string]string // QID -> CategoryName
	}{
		{
			name:              "Standard Success",
			ontologicalResp:   `{"subclasses": [{"name": "Shinto Shrine", "category": "Sights", "size": "M"}]}`,
			topographicalResp: `{"subclasses": [{"name": "Night Market", "category": "Shopping", "size": "L"}]}`,
			wikidataResps: map[string]string{
				"Shinto Shrine": `{"search": [{"id": "Q123", "label": "Shinto Shrine"}]}`,
				"Night Market":  `{"search": [{"id": "Q456", "label": "Night Market"}]}`,
				"Q123":          `{"entities": {"Q123": {"labels": {"en": {"value": "Shinto Shrine"}}} }}`,
				"Q456":          `{"entities": {"Q456": {"labels": {"en": {"value": "Night Market"}}} }}`,
			},
			expectedCats: map[string]string{"Q123": "Sights", "Q456": "Shopping"},
		},
		{
			name:              "Generic Category Resolution",
			ontologicalResp:   `{"subclasses": [{"name": "Local Shrine", "category": "Generic", "specific_category": "Sights", "size": "M"}]}`,
			topographicalResp: `{"subclasses": []}`,
			wikidataResps: map[string]string{
				"Local Shrine": `{"search": [{"id": "Q789", "label": "Local Shrine"}]}`,
				"Q789":         `{"entities": {"Q789": {"labels": {"en": {"value": "Local Shrine"}}} }}`,
			},
			expectedCats: map[string]string{"Q789": "Sights"},
		},
		{
			name:              "Duplicate and Static Filtering",
			ontologicalResp:   `{"subclasses": [{"name": "Castle", "category": "Sights", "size": "L"}]}`,
			topographicalResp: `{"subclasses": [{"name": "Castle", "category": "Sights", "size": "L"}, {"name": "Aerodrome", "category": "Aerodrome", "size": "L"}]}`,
			wikidataResps: map[string]string{
				"Castle":    `{"search": [{"id": "Q1", "label": "Castle"}]}`,
				"Aerodrome": `{"search": [{"id": "Q2", "label": "Aerodrome"}]}`,
				"Q1":        `{"entities": {"Q1": {"labels": {"en": {"value": "Castle"}}} }}`,
				"Q2":        `{"entities": {"Q2": {"labels": {"en": {"value": "Aerodrome"}}} }}`,
			},
			// Castle Q1 added once. Aerodrome Q2 is already in static config.
			expectedCats: map[string]string{"Q1": "Sights"},
		},
		{
			name:              "Deep Redundancy Filtering (Subclass)",
			ontologicalResp:   `{"subclasses": [{"name": "Alcázar", "category": "Sights", "size": "M"}]}`,
			topographicalResp: `{"subclasses": []}`,
			wikidataResps: map[string]string{
				"Alcázar":   `{"search": [{"id": "Q_ALCAZAR", "label": "Alcázar"}]}`,
				"Q_ALCAZAR": `{"entities": {"Q_ALCAZAR": {"labels": {"en": {"value": "Alcázar"}}, "claims": {"P279": [{"mainsnak": {"datavalue": {"value": {"id": "Q_CASTLE"}}}}]} }}}`,
			},
			// Q_CASTLE is in static config
			expectedCats: map[string]string{}, // Should be skipped as redundant
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mLLM := &mockLLM{
				responses: map[string]string{
					"regional_categories_ontological":   tt.ontologicalResp,
					"regional_categories_topographical": tt.topographicalResp,
				},
			}

			catCfg := &config.CategoriesConfig{
				Categories: map[string]config.Category{
					"Aerodrome": {Weight: 100, QIDs: map[string]string{"Q2": ""}},
					"Sights":    {Weight: 50, QIDs: map[string]string{"Q_CASTLE": ""}},
					"Shopping":  {Weight: 30},
				},
			}

			st := &mockSpatialStore{
				cats:   make(map[string]map[string]string),
				labels: make(map[string]map[string]string),
			}
			tr := tracker.New()

			promptDir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(promptDir, "context"), 0755)
			_ = os.WriteFile(filepath.Join(promptDir, "context", "ontological.tmpl"), []byte("onto"), 0644)
			_ = os.WriteFile(filepath.Join(promptDir, "context", "topographical.tmpl"), []byte("topo"), 0644)
			pm, _ := prompts.NewManager(promptDir)

			// Mock transport for Wikidata Search
			transport := &mockTransport{responses: tt.wikidataResps}
			reqClient := request.New(nil, tr, request.ClientConfig{})
			reqClient.SetTransport(transport)

			wikiCl := wikidata.NewClient(reqClient, nil)
			validator := wikidata.NewValidator(wikiCl)
			clf := classifier.NewClassifier(st, wikiCl, catCfg, tr)
			cityFile := filepath.Join(t.TempDir(), "c.txt")
			adminFile := filepath.Join(t.TempDir(), "a.txt")
			_ = os.WriteFile(cityFile, []byte(""), 0644)
			_ = os.WriteFile(adminFile, []byte(""), 0644)
			geoSvc, _ := geo.NewService(cityFile, adminFile)
			dummyCfg := config.NewProvider(&config.Config{
				Wikidata: config.WikidataConfig{
					Area: config.AreaConfig{MaxDist: 80000},
				},
			}, nil)
			job := NewRegionalCategoriesJob(dummyCfg, mLLM, pm, validator, clf, geoSvc, &wikidata.Service{}, st)

			// Trigger Run (starts goroutine)
			job.Run(context.Background(), &sim.Telemetry{Latitude: 50, Longitude: 10})

			// Wait for completion (poll result)
			var res map[string]string
			for i := 0; i < 50; i++ {
				res = clf.GetRegionalCategories()
				if len(res) == len(tt.expectedCats) {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}

			if len(res) != len(tt.expectedCats) {
				t.Errorf("Expected %d categories, got %d: %v", len(tt.expectedCats), len(res), res)
			}
			for k, v := range tt.expectedCats {
				if res[k] != v {
					t.Errorf("For QID %s, expected category %q, got %q", k, v, res[k])
				}
			}
		})
	}
}

func TestRegionalCategoriesJob_SequentialLogic(t *testing.T) {
	mLLM := &mockLLM{
		responses: map[string]string{
			"regional_categories_ontological":   `{"subclasses": [{"name": "OntoClass", "category": "Aerodrome", "size": "L"}]}`,
			"regional_categories_topographical": `{"subclasses": [{"name": "TopoClass", "category": "Sights", "size": "M"}]}`,
		},
	}

	appCfg := &config.Config{}
	cfgProv := config.NewProvider(appCfg, nil)

	catCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Aerodrome": {Weight: 100},
			"Sights":    {Weight: 50},
		},
	}

	st := &mockHierarchyStore{}
	tr := tracker.New()

	clf := classifier.NewClassifier(st, nil, catCfg, tr)

	// Create a dummy prompt dir
	promptDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(promptDir, "context"), 0755)
	_ = os.WriteFile(filepath.Join(promptDir, "context", "ontological.tmpl"), []byte("onto"), 0644)
	_ = os.WriteFile(filepath.Join(promptDir, "context", "topographical.tmpl"), []byte("topo"), 0644)

	pm, _ := prompts.NewManager(promptDir)

	// Create a mock transport for Wikidata API
	reqClient := request.New(nil, tr, request.ClientConfig{})
	wikiCl := wikidata.NewClient(reqClient, nil)
	validator := wikidata.NewValidator(wikiCl)

	// Create a dummy geo service
	cityFile := filepath.Join(t.TempDir(), "cities.txt")
	adminFile := filepath.Join(t.TempDir(), "admin.txt")
	_ = os.WriteFile(cityFile, []byte(""), 0644)
	_ = os.WriteFile(adminFile, []byte(""), 0644)
	geoSvc, _ := geo.NewService(cityFile, adminFile)

	wikiSvc := &wikidata.Service{}

	job := NewRegionalCategoriesJob(cfgProv, mLLM, pm, validator, clf, geoSvc, wikiSvc, st)

	if job.Name() != "Regional Categories" {
		t.Errorf("Expected name 'Regional Categories', got %q", job.Name())
	}
	tel := &sim.Telemetry{Latitude: 50.0, Longitude: 10.0}

	if !job.ShouldFire(tel) {
		t.Error("ShouldFire should be true on first run")
	}

	job.ResetSession(context.Background())
	if !job.firstRun {
		t.Error("ResetSession should set firstRun to true")
	}
}

func TestRegionalCategoriesJob_ShouldFire_Thresholds(t *testing.T) {
	job, _, _ := setupJob(t, map[string]string{"regional_categories_ontological": "{}"}, nil)

	tel := &sim.Telemetry{Latitude: 50, Longitude: 10}

	// First run
	if !job.ShouldFire(tel) {
		t.Error("ShouldFire should be true on first run")
	}
	job.Run(context.Background(), tel)
	waitJob(job) // Wait for unlock

	// Immediate second call
	if job.ShouldFire(tel) {
		t.Error("ShouldFire should be false immediately after run")
	}

	// Move slightly (under 50nm)
	telSmallMove := &sim.Telemetry{Latitude: 50.1, Longitude: 10.1}
	if job.ShouldFire(telSmallMove) {
		t.Error("ShouldFire should be false for small move")
	}

	// Move far (>50nm) but no time passed
	telFarMove := &sim.Telemetry{Latitude: 52, Longitude: 12}
	if job.ShouldFire(telFarMove) {
		t.Error("ShouldFire should be false if enough distance but not enough time")
	}

	// Enough distance AND time (manually sets lastRunTime)
	job.lastRunTime = time.Now().Add(-31 * time.Minute)
	if !job.ShouldFire(telFarMove) {
		t.Error("ShouldFire should be true after 50nm AND 30min")
	}
}

type mockSpatialStore struct {
	mockHierarchyStore
	cats   map[string]map[string]string // gridKey -> qid -> cat
	labels map[string]map[string]string // gridKey -> qid -> label
	mu     sync.Mutex
}

func (m *mockSpatialStore) GetRegionalCategories(ctx context.Context, latGrid, lonGrid int) (map[string]string, map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d_%d", latGrid, lonGrid)
	return m.cats[key], m.labels[key], nil
}

func TestRegionalCategoriesJob_SpatialCacheLoading(t *testing.T) {
	job, clf, st := setupJob(t, nil, nil)
	st.cats["50_10"] = map[string]string{"Q_LOCAL": "LocalCat"}
	st.labels["50_10"] = map[string]string{"Q_LOCAL": "LocalLabel"}

	// Run with telemetry in 50, 10
	job.Run(context.Background(), &sim.Telemetry{Latitude: 50.1, Longitude: 10.1})
	waitJob(job) // Wait for unlock

	// Wait for background routine (above waitJob might be enough, but let's keep polling for the classifier update)
	var res map[string]string
	for i := 0; i < 20; i++ {
		res = clf.GetRegionalCategories()
		if len(res) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if res["Q_LOCAL"] != "LocalCat" {
		t.Errorf("Expected Q_LOCAL from cache, got %v", res)
	}
	if clf.GetRegionalLabels()["Q_LOCAL"] != "LocalLabel" {
		t.Errorf("Expected LocalLabel from cache, got %v", clf.GetRegionalLabels())
	}
}
func TestRegionalCategoriesJob_LabelHydration(t *testing.T) {
	// Mock Wikidata response for Q_MISSING -> "New Hydrated Label"
	transport := &mockTransport{
		responses: map[string]string{
			"Q_MISSING": `{"entities": {"Q_MISSING": {"labels": {"en": {"value": "New Hydrated Label"}}}} }`,
		},
	}

	job, clf, st := setupJob(t, nil, transport)

	// Pre-seed cache with a category but NO label
	st.cats["50_10"] = map[string]string{"Q_MISSING": "LocalCat"}
	st.labels["50_10"] = map[string]string{"Q_MISSING": ""} // Empty label

	// Run job
	job.Run(context.Background(), &sim.Telemetry{Latitude: 50.1, Longitude: 10.1})
	waitJob(job)

	// Verify classifier has the hydrated label
	resLabels := clf.GetRegionalLabels()
	if resLabels["Q_MISSING"] != "New Hydrated Label" {
		t.Errorf("Expected hydrated label 'New Hydrated Label', got %q", resLabels["Q_MISSING"])
	}

	// Verify spatial cache was updated
	updatedLabels := st.labels["50_10"]
	if updatedLabels["Q_MISSING"] != "New Hydrated Label" {
		t.Errorf("Expected spatial cache to be updated with 'New Hydrated Label', got %v", updatedLabels)
	}
}

func (m *mockSpatialStore) SaveRegionalCategories(ctx context.Context, latGrid, lonGrid int, categories map[string]string, labels map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d_%d", latGrid, lonGrid)
	m.cats[key] = categories
	m.labels[key] = labels
	return nil
}

func TestRegionalCategoriesJob_Pruning(t *testing.T) {
	// Setup with a cached category that is now redundant (e.g. Q_SUB_CASTLE)
	// We'll mock the hierarchy so Q_SUB_CASTLE is a subclass of Q_STATIC_CASTLE
	transport := &mockTransport{
		responses: map[string]string{
			"Q_SUB_CASTLE": `{"entities": {"Q_SUB_CASTLE": {"labels": {"en": {"value": "Alcázar"}}, "claims": {"P279": [{"mainsnak": {"datavalue": {"value": {"id": "Q_STATIC_CASTLE"}}}}]} }}}`,
		},
	}

	st := &mockSpatialStore{
		cats:   make(map[string]map[string]string),
		labels: make(map[string]map[string]string),
	}
	tr := tracker.New()
	catCfg := &config.CategoriesConfig{
		Categories: map[string]config.Category{
			"Sights": {Weight: 50, QIDs: map[string]string{"Q_STATIC_CASTLE": ""}},
		},
	}

	reqClient := request.New(nil, tr, request.ClientConfig{})
	reqClient.SetTransport(transport)
	wikiCl := wikidata.NewClient(reqClient, nil)

	clf := classifier.NewClassifier(st, wikiCl, catCfg, tr)
	dummyCfg := config.NewProvider(&config.Config{
		Wikidata: config.WikidataConfig{
			Area: config.AreaConfig{MaxDist: 80000},
		},
	}, nil)

	// Pre-seed cache
	st.cats["50_10"] = map[string]string{"Q_SUB_CASTLE": "LocalCat"}
	st.labels["50_10"] = map[string]string{"Q_SUB_CASTLE": "Alcázar"}

	job := NewRegionalCategoriesJob(dummyCfg, &mockLLM{}, nil, nil, clf, nil, &wikidata.Service{}, st)

	// Run job
	job.Run(context.Background(), &sim.Telemetry{Latitude: 50.1, Longitude: 10.1})
	waitJob(job)

	// Verify classifier does NOT have Q_SUB_CASTLE
	res := clf.GetRegionalCategories()
	if _, ok := res["Q_SUB_CASTLE"]; ok {
		t.Error("Expected Q_SUB_CASTLE to be pruned from regional categories")
	}

	// Verify spatial cache was updated and pruned
	updatedCats := st.cats["50_10"]
	if _, ok := updatedCats["Q_SUB_CASTLE"]; ok {
		t.Error("Expected Q_SUB_CASTLE to be pruned from spatial cache store")
	}
}
