package wikidata

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/request"
	"phileasgo/pkg/tracker"
)

type mockCacher struct{}

func (m *mockCacher) GetCache(ctx context.Context, key string) ([]byte, bool)    { return nil, false }
func (m *mockCacher) SetCache(ctx context.Context, key string, val []byte) error { return nil }

func TestGetEntitiesBatch(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ids := q.Get("ids")
		if ids == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Mock response using the actual struct but without anonymous types to avoid tag issues
		type Ent struct {
			Labels map[string]struct {
				Value string `json:"value"`
			} `json:"labels"`
			Claims map[string][]struct {
				Mainsnak map[string]interface{} `json:"mainsnak"`
			} `json:"claims"`
		}

		resp := struct {
			Entities map[string]Ent `json:"entities"`
		}{
			Entities: make(map[string]Ent),
		}

		// Add Q1
		resp.Entities["Q1"] = Ent{
			Labels: map[string]struct {
				Value string `json:"value"`
			}{
				"en": {Value: "Label 1"},
			},
		}

		// Add Q2 with P31
		resp.Entities["Q2"] = Ent{
			Labels: map[string]struct {
				Value string `json:"value"`
			}{
				"en": {Value: "Label 2"},
			},
			Claims: map[string][]struct {
				Mainsnak map[string]interface{} `json:"mainsnak"`
			}{
				"P31": {
					{
						Mainsnak: map[string]interface{}{
							"datavalue": map[string]interface{}{
								"type": "wikibase-entityid",
								"value": map[string]interface{}{
									"entity-type": "item",
									"id":          "Q5",
								},
							},
						},
					},
				},
				"P18": { // Image - string value
					{
						Mainsnak: map[string]interface{}{
							"datavalue": map[string]interface{}{
								"type":  "string",
								"value": "Image.jpg",
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// 2. Setup Client
	tr := tracker.New()
	reqClient := request.New(&mockCacher{}, tr)

	client := NewClient(reqClient, slog.Default())
	client.APIEndpoint = ts.URL

	// 3. Test
	ctx := context.Background()
	results, err := client.GetEntitiesBatch(ctx, []string{"Q1", "Q2"})
	if err != nil {
		t.Fatalf("GetEntitiesBatch failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results["Q1"].Labels["en"] != "Label 1" {
		t.Errorf("Q1 label mismatch: %s", results["Q1"].Labels["en"])
	}

	if results["Q2"].Labels["en"] != "Label 2" {
		t.Errorf("Q2 label mismatch: %s", results["Q2"].Labels["en"])
	}

	p31 := results["Q2"].Claims["P31"]
	if len(p31) != 1 || p31[0] != "Q5" {
		t.Errorf("Q2 P31 claims mismatch: %v", p31)
	}
}
