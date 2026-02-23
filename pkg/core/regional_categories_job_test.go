package core

import (
	"bytes"
	"context"
	"encoding/json"
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
	resp, ok := m.responses[q]
	if !ok {
		// Fallback to a default or empty if not found
		resp = `{"search": []}`
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
func (m *mockHierarchyStore) GetRegionalCategories(ctx context.Context, latGrid, lonGrid int) (map[string]string, error) {
	return nil, nil
}
func (m *mockHierarchyStore) SaveRegionalCategories(ctx context.Context, latGrid, lonGrid int, categories map[string]string) error {
	return nil
}

func TestRegionalCategoriesJob_Merging(t *testing.T) {
	st := &mockHierarchyStore{}
	tr := tracker.New()
	clf := classifier.NewClassifier(nil, nil, &config.CategoriesConfig{}, tr)
	job := NewRegionalCategoriesJob(nil, nil, nil, nil, clf, nil, nil, st)

	// Pre-seed with one category
	clf.AddRegionalCategories(map[string]string{"Q1": "Cat1"})

	// Simulate discovery of another one
	job.classifier.AddRegionalCategories(map[string]string{"Q2": "Cat2"})

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
			},
			expectedCats: map[string]string{"Q123": "Sights", "Q456": "Shopping"},
		},
		{
			name:              "Generic Category Resolution",
			ontologicalResp:   `{"subclasses": [{"name": "Local Shrine", "category": "Generic", "specific_category": "Sights", "size": "M"}]}`,
			topographicalResp: `{"subclasses": []}`,
			wikidataResps: map[string]string{
				"Local Shrine": `{"search": [{"id": "Q789", "label": "Local Shrine"}]}`,
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
			},
			// Castle Q1 added once. Aerodrome Q2 is already in static config.
			expectedCats: map[string]string{"Q1": "Sights"},
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
					"Sights":    {Weight: 50},
					"Shopping":  {Weight: 30},
				},
			}

			st := &mockHierarchyStore{}
			tr := tracker.New()
			clf := classifier.NewClassifier(st, nil, catCfg, tr)

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
