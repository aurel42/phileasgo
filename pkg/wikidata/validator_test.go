package wikidata

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

func TestValidator_ValidateBatch(t *testing.T) {
	// Mock Server handling both wbgetentities and wbsearchentities
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")

		if action == "wbgetentities" {
			// Mock Direct Lookup
			ids := r.URL.Query().Get("ids")
			if strings.Contains(ids, "Q1") {
				fmt.Fprint(w, `{"entities": {"Q1": {"labels": {"en": {"value": "Castle"}} } }}`)
				return
			}
			if strings.Contains(ids, "Q2") {
				// Mismatch simulation
				fmt.Fprint(w, `{"entities": {"Q2": {"labels": {"en": {"value": "River"}} } }}`)
				return
			}
			fmt.Fprint(w, `{"entities": {}}`)
			return
		}

		if action == "wbsearchentities" {
			search := r.URL.Query().Get("search")
			if search == "Tower" {
				fmt.Fprint(w, `{"search": [{"id": "Q3", "label": "The Tower"}]}`)
				return
			}
			fmt.Fprint(w, `{"search": []}`)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	// Setup Validator
	trk := tracker.New()
	reqClient := request.New(&mockCacher{}, trk) // Reuse mockCacher from mapper_test (if exported) or redefine?
	// mockCacher in mapper_test.go is not exported. I need to redefine it here or make it common.
	// Redefining for speed.

	client := NewClient(reqClient, slog.Default())
	client.APIEndpoint = server.URL

	v := NewValidator(client)

	// Test Cases
	suggestions := map[string]string{
		"Castle": "Q1",  // Exact match (Label "Castle")
		"Lake":   "Q2",  // Mismatch (Label "River") -> Fallback (fail)
		"Tower":  "Q99", // Bad QID -> Fallback search "Tower" -> Finds "Q3"
	}

	ctx := context.Background()
	results := v.ValidateBatch(ctx, suggestions)

	// Verify "Castle" -> Q1
	if res, ok := results["Castle"]; !ok || res.QID != "Q1" {
		t.Errorf("Expected Castle -> Q1, got %v", res)
	}

	// Verify "Lake" -> Should fail or be validated?
	// ValidateBatch:
	// 1. fetchLabels(Q1, Q2, Q99). Q2->River.
	// 2. tryDirectMatch("Lake", "Q2", ...) -> "river" != "lake" -> false.
	// 3. trySearchFallback("Lake") -> empty -> false.
	// So "Lake" should be missing.
	if _, ok := results["Lake"]; ok {
		t.Errorf("Expected Lake to be invalid, but got %v", results["Lake"])
	}

	// Verify "Tower" -> Search fallback -> Q3
	if res, ok := results["Tower"]; !ok || res.QID != "Q3" {
		t.Errorf("Expected Tower -> Q3 (rescued), got %v", res)
	}
}

// Redefine mockCacher here to avoid import cycles or undefined refs if different package (same package but separate files)
type mockCacherV struct{}

func (m *mockCacherV) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *mockCacherV) SetCache(ctx context.Context, key string, val []byte) error { return nil }
