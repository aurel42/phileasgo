package wikipedia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

// mockCacher implements cache.Cacher for testing
type mockCacher struct{}

func (m *mockCacher) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *mockCacher) SetCache(ctx context.Context, key string, val []byte) error { return nil }
func (m *mockCacher) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *mockCacher) SetGeodataCache(ctx context.Context, key string, val []byte, radiusM int) error {
	return nil
}

func TestGetArticleLengths(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Request
		// Verify Request
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		if r.Form.Get("action") != "query" {
			t.Errorf("Expected action=query, got %s", r.Form.Get("action"))
		}
		if r.Form.Get("prop") != "info" {
			t.Errorf("Expected prop=info, got %s", r.Form.Get("prop"))
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
	reqClient := request.New(&mockCacher{}, tr, request.ClientConfig{
		Retries:   2,
		BaseDelay: 1 * time.Millisecond,
		MaxDelay:  5 * time.Millisecond,
	})

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
