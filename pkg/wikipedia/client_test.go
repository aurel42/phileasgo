package wikipedia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

// mockCacher implements cache.Cacher for testing
type mockCacher struct{}

func (m *mockCacher) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *mockCacher) SetCache(ctx context.Context, key string, val []byte) error { return nil }

func TestGetArticleLengths(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Request
		if r.URL.Query().Get("action") != "query" {
			t.Errorf("Expected action=query, got %s", r.URL.Query().Get("action"))
		}
		if r.URL.Query().Get("prop") != "info" {
			t.Errorf("Expected prop=info, got %s", r.URL.Query().Get("prop"))
		}

		// Mock Response
		resp := response{}
		resp.Query.Pages = make(map[string]struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Length  int    `json:"length"`
			Missing string `json:"missing,omitempty"`
		})

		// Respond with lengths
		resp.Query.Pages["1"] = struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Length  int    `json:"length"`
			Missing string `json:"missing,omitempty"`
		}{
			PageID: 1, Title: "Paris", Length: 1000,
		}
		resp.Query.Pages["2"] = struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Length  int    `json:"length"`
			Missing string `json:"missing,omitempty"`
		}{
			PageID: 2, Title: "Berlin", Length: 2000,
		}

		// Mock Redirect
		resp.Query.Redirects = []struct {
			From string `json:"from"`
			To   string `json:"to"`
		}{
			{From: "Paname", To: "Paris"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// 2. Setup Client
	tr := tracker.New()
	// Need a request client
	reqClient := request.New(&mockCacher{}, tr)

	client := NewClient(reqClient)
	client.APIEndpoint = ts.URL

	// 3. Execute
	ctx := context.Background()
	lengths, err := client.GetArticleLengths(ctx, []string{"Paris", "Berlin", "Paname"}, "en")
	if err != nil {
		t.Fatalf("GetArticleLengths failed: %v", err)
	}

	// 4. Verify
	if len(lengths) != 3 {
		t.Errorf("Expected 3 lengths, got %d", len(lengths))
	}
	if lengths["Paris"] != 1000 {
		t.Errorf("Expected Paris length 1000, got %d", lengths["Paris"])
	}
	if lengths["Berlin"] != 2000 {
		t.Errorf("Expected Berlin length 2000, got %d", lengths["Berlin"])
	}
	// Verify Redirect handling
	if lengths["Paname"] != 1000 {
		t.Errorf("Expected Paname (redirect) length 1000, got %d", lengths["Paname"])
	}
}
