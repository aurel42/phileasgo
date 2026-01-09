package wikidata

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

type mockCache struct{}

func (m *mockCache) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *mockCache) SetCache(ctx context.Context, key string, val []byte) error { return nil }

func TestFetchFallbackData(t *testing.T) {
	tests := []struct {
		name       string
		qids       []string
		mockResp   string
		mockStatus int
		wantErr    bool
		wantCount  int
		wantLabel  string
	}{
		{
			name: "Success with labels and sitelinks",
			qids: []string{"Q1"},
			mockResp: `{
				"entities": {
					"Q1": {
						"labels": {
							"en": {"value": "Tower"}
						},
						"sitelinks": {
							"enwiki": {"site": "enwiki", "title": "Tower"}
						}
					}
				}
			}`,
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantCount:  1,
			wantLabel:  "Tower",
		},
		{
			name:       "Empty QID list",
			qids:       []string{},
			mockResp:   "",
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantCount:  0,
		},
		{
			name:       "API Error",
			qids:       []string{"Q1"},
			mockResp:   `{"error": "bad"}`,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
			wantCount:  0,
		},
		{
			name:       "Malformed JSON",
			qids:       []string{"Q1"},
			mockResp:   `{invalid json}`,
			mockStatus: http.StatusOK,
			wantErr:    true,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/w/api.php" {
					t.Errorf("Expected path /w/api.php, got %s", r.URL.Path)
				}
				action := r.URL.Query().Get("action")
				if action != "wbgetentities" {
					t.Errorf("Expected action wbgetentities, got %s", action)
				}
				w.WriteHeader(tt.mockStatus)
				fmt.Fprint(w, tt.mockResp)
			}))
			defer server.Close()

			trk := tracker.New()
			mc := &mockCache{}
			reqClient := request.New(mc, trk)
			client := NewClient(reqClient, slog.Default())
			client.APIEndpoint = server.URL + "/w/api.php"

			got, err := client.FetchFallbackData(context.Background(), tt.qids)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchFallbackData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != tt.wantCount {
					t.Errorf("FetchFallbackData() count = %d, want %d", len(got), tt.wantCount)
				}
				if tt.wantCount > 0 {
					if l := got["Q1"].Labels["en"]; l != tt.wantLabel {
						t.Errorf("FetchFallbackData() label = %s, want %s", l, tt.wantLabel)
					}
				}
			}
		})
	}
}

func TestGetEntityClaims(t *testing.T) {
	// Simple test for existing method to boost coverage
	mockResp := `{
		"entities": {
			"Q1": {
				"labels": {"en": {"value": "MyLabel"}},
				"claims": {
					"P31": [
						{"mainsnak": {"datavalue": {"value": {"id": "Q5"}}}}
					]
				}
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, mockResp)
	}))
	defer server.Close()

	trk := tracker.New()
	mc := &mockCache{}
	reqClient := request.New(mc, trk)
	client := NewClient(reqClient, slog.Default())
	client.APIEndpoint = server.URL + "/w/api.php"

	targets, label, err := client.GetEntityClaims(context.Background(), "Q1", "P31")
	if err != nil {
		t.Fatalf("GetEntityClaims failed: %v", err)
	}
	if label != "MyLabel" {
		t.Errorf("Expected label MyLabel, got %s", label)
	}
	if len(targets) != 1 || targets[0] != "Q5" {
		t.Errorf("Expected target Q5, got %v", targets)
	}
}

func TestQuerySPARQL(t *testing.T) {
	tests := []struct {
		name       string
		mockResp   string
		mockStatus int
		mockErr    bool
		wantErr    bool
		wantCount  int
	}{
		{
			name: "Success",
			mockResp: `{
				"results": {
					"bindings": [
						{
							"item": {"value": "http://www.wikidata.org/entity/Q1"},
							"lat": {"value": "50.5"},
							"lon": {"value": "14.5"},
							"sitelinks": {"value": "10"},
							"title_local_val": {"value": "LocalTitle"},
							"itemLabel": {"value": "LabelTitle"},
							"instances": {"value": "http://www.wikidata.org/entity/Q5,http://www.wikidata.org/entity/Q6"}
						}
					]
				}
			}`,
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantCount:  1,
		},
		{
			name:       "API Error (500)",
			mockResp:   "",
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
			wantCount:  0,
		},
		{
			name:       "Malformed JSON",
			mockResp:   "{invalid json}",
			mockStatus: http.StatusOK,
			wantErr:    true,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" { // Verify POST usage
					t.Errorf("Expected POST, got %s", r.Method)
				}
				w.WriteHeader(tt.mockStatus)
				fmt.Fprint(w, tt.mockResp)
			}))
			defer server.Close()

			trk := tracker.New()
			mc := &mockCache{}
			reqClient := request.New(mc, trk)
			client := NewClient(reqClient, slog.Default())
			client.SPARQLEndpoint = server.URL + "/sparql"

			articles, _, err := client.QuerySPARQL(context.Background(), "SELECT * WHERE {}", "")

			if (err != nil) != tt.wantErr {
				t.Fatalf("QuerySPARQL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(articles) != tt.wantCount {
					t.Fatalf("Expected %d articles, got %d", tt.wantCount, len(articles))
				}
				if tt.wantCount > 0 {
					a := articles[0]
					if a.QID != "Q1" {
						t.Errorf("Expected QID Q1, got %s", a.QID)
					}
				}
			}
		})
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mockResp   string
		mockStatus int
		mockErr    bool
		wantErr    bool
		wantCount  int
		wantID     string
	}{
		{
			name:  "Success",
			query: "Eiffel Tower",
			mockResp: `{
				"search": [
					{
						"id": "Q243",
						"label": "Eiffel Tower",
						"description": "landmark in Paris"
					}
				]
			}`,
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantCount:  1,
			wantID:     "Q243",
		},
		{
			name:       "Empty Results",
			query:      "NonExistentThing12345",
			mockResp:   `{"search": []}`,
			mockStatus: http.StatusOK,
			wantErr:    false,
			wantCount:  0,
		},
		{
			name:       "API Failure",
			query:      "Error",
			mockResp:   `{"error": "fail"}`,
			mockStatus: http.StatusInternalServerError,
			wantErr:    true,
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/w/api.php" {
					t.Errorf("Expected path /w/api.php, got %s", r.URL.Path)
				}
				action := r.URL.Query().Get("action")
				if action != "wbsearchentities" {
					t.Errorf("Expected action wbsearchentities, got %s", action)
				}
				search := r.URL.Query().Get("search")
				if search != tt.query {
					t.Errorf("Expected search %s, got %s", tt.query, search)
				}
				w.WriteHeader(tt.mockStatus)
				fmt.Fprint(w, tt.mockResp)
			}))
			defer server.Close()

			trk := tracker.New()
			mc := &mockCache{}
			reqClient := request.New(mc, trk)
			client := NewClient(reqClient, slog.Default())
			client.APIEndpoint = server.URL + "/w/api.php"

			results, err := client.SearchEntities(context.Background(), tt.query)

			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(results) != tt.wantCount {
					t.Errorf("Expected %d results, got %d", tt.wantCount, len(results))
				}
				if tt.wantCount > 0 {
					if results[0].ID != tt.wantID {
						t.Errorf("Expected ID %s, got %s", tt.wantID, results[0].ID)
					}
				}
			}
		})
	}
}
