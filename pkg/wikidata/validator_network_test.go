package wikidata

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/request"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
)

// MockStoreForRequest is a minimal stub for request.New dependency
type MockStoreForRequest struct {
	store.Store // Embed interface to skip implementing everything
}

func (m *MockStoreForRequest) GetCache(ctx context.Context, key string) ([]byte, bool) {
	return nil, false
}
func (m *MockStoreForRequest) SetCache(ctx context.Context, key string, val []byte) error { return nil }

// TestVerifyStartupConfig_WithServer uses a local HTTP test server to mock Wikidata API
func TestVerifyStartupConfig_WithServer(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]string
		mockResp   map[string]interface{}
		searchResp []map[string]string
		wantErr    bool
	}{
		{
			name:   "Success and Fallback",
			config: map[string]string{"Q1": "Castle", "Q2": "Tower"},
			mockResp: map[string]interface{}{
				"Q1": map[string]interface{}{
					"labels": map[string]interface{}{
						"en": map[string]interface{}{"value": "Grand Castle"},
					},
				},
				"Q2": map[string]interface{}{
					"labels": map[string]interface{}{
						"en": map[string]interface{}{"value": "Small Hut"},
					},
				},
			},
			searchResp: []map[string]string{
				{"id": "Q99", "label": "The Tower", "description": "A big tower"},
			},
			wantErr: false,
		},
		{
			name:   "Partial Failure",
			config: map[string]string{"Q3": "Mystery"},
			mockResp: map[string]interface{}{
				"Q3": map[string]interface{}{
					"labels": map[string]interface{}{
						"en": map[string]interface{}{"value": "Something Else"},
					},
				},
			},
			searchResp: []map[string]string{}, // Empty search results
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				action := r.URL.Query().Get("action")
				if action == "wbgetentities" {
					resp := wrapperEntityResponse{Entities: make(map[string]struct {
						Labels map[string]struct {
							Value string `json:"value"`
						} `json:"labels"`
						Claims map[string][]struct {
							Mainsnak map[string]interface{} `json:"mainsnak"`
						} `json:"claims"`
					})}

					// Manually map mockResp to response struct (simplified)
					for qid, data := range tt.mockResp {
						d := data.(map[string]interface{})
						// Helper to extract label
						lbls := d["labels"].(map[string]interface{})
						en := lbls["en"].(map[string]interface{})
						val := en["value"].(string)

						resp.Entities[qid] = struct {
							Labels map[string]struct {
								Value string `json:"value"`
							} `json:"labels"`
							Claims map[string][]struct {
								Mainsnak map[string]interface{} `json:"mainsnak"`
							} `json:"claims"`
						}{
							Labels: map[string]struct {
								Value string `json:"value"`
							}{
								"en": {Value: val},
							},
						}
					}

					json.NewEncoder(w).Encode(resp)
					return
				}
				if action == "wbsearchentities" {
					wrap := map[string]interface{}{"search": tt.searchResp}
					json.NewEncoder(w).Encode(wrap)
					return
				}
				http.NotFound(w, r)
			})

			server := httptest.NewServer(handler)
			defer server.Close()

			mst := &MockStoreForRequest{}
			reqClient := request.New(mst, tracker.New(), request.ClientConfig{Retries: 0})
			client := NewClient(reqClient, slog.Default())
			client.APIEndpoint = server.URL

			validator := NewValidator(client)
			err := validator.VerifyStartupConfig(context.Background(), tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyStartupConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClient_GetEntityClaimsBatch_WithServer ensures Client methods are covered
func TestClient_GetEntityClaimsBatch_WithServer(t *testing.T) {
	tests := []struct {
		name       string
		ids        []string
		prop       string
		mockResp   map[string]interface{}
		wantClaims map[string][]string
		wantLabels map[string]string
		wantErr    bool
	}{
		{
			name: "Single Entity Success",
			ids:  []string{"Q1"},
			prop: "P31",
			mockResp: map[string]interface{}{
				"Q1": map[string]interface{}{
					"labels": map[string]interface{}{
						"en": map[string]interface{}{"value": "Test Item"},
					},
					"claims": map[string]interface{}{
						"P31": []interface{}{
							map[string]interface{}{
								"mainsnak": map[string]interface{}{
									"datavalue": map[string]interface{}{
										"value": map[string]interface{}{"id": "Q5"},
									},
								},
							},
						},
					},
				},
			},
			wantClaims: map[string][]string{"Q1": {"Q5"}},
			wantLabels: map[string]string{"Q1": "Test Item"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := wrapperEntityResponse{Entities: make(map[string]struct {
					Labels map[string]struct {
						Value string `json:"value"`
					} `json:"labels"`
					Claims map[string][]struct {
						Mainsnak map[string]interface{} `json:"mainsnak"`
					} `json:"claims"`
				})}

				for qid, data := range tt.mockResp {
					d := data.(map[string]interface{})

					lblVal := ""
					if lbls, ok := d["labels"].(map[string]interface{}); ok {
						if en, ok := lbls["en"].(map[string]interface{}); ok {
							lblVal = en["value"].(string)
						}
					}

					var claims []struct {
						Mainsnak map[string]interface{} `json:"mainsnak"`
					}
					if cls, ok := d["claims"].(map[string]interface{}); ok {
						if propClaims, ok := cls[tt.prop].([]interface{}); ok {
							for _, pc := range propClaims {
								pcMap := pc.(map[string]interface{})
								claims = append(claims, struct {
									Mainsnak map[string]interface{} `json:"mainsnak"`
								}{
									Mainsnak: pcMap["mainsnak"].(map[string]interface{}),
								})
							}
						}
					}

					resp.Entities[qid] = struct {
						Labels map[string]struct {
							Value string `json:"value"`
						} `json:"labels"`
						Claims map[string][]struct {
							Mainsnak map[string]interface{} `json:"mainsnak"`
						} `json:"claims"`
					}{
						Labels: map[string]struct {
							Value string `json:"value"`
						}{
							"en": {Value: lblVal},
						},
						Claims: map[string][]struct {
							Mainsnak map[string]interface{} `json:"mainsnak"`
						}{
							tt.prop: claims,
						},
					}
				}
				json.NewEncoder(w).Encode(resp)
			})
			server := httptest.NewServer(handler)
			defer server.Close()

			mst := &MockStoreForRequest{}
			reqClient := request.New(mst, tracker.New(), request.ClientConfig{})
			client := NewClient(reqClient, slog.Default())
			client.APIEndpoint = server.URL

			claims, labels, err := client.GetEntityClaimsBatch(context.Background(), tt.ids, tt.prop)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Failed: %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				for id, wantC := range tt.wantClaims {
					gotC := claims[id]
					if len(gotC) != len(wantC) {
						t.Errorf("Claims mismatch for %s: got %v, want %v", id, gotC, wantC)
					}
				}
				for id, wantL := range tt.wantLabels {
					if labels[id] != wantL {
						t.Errorf("Labels mismatch for %s: got %q, want %q", id, labels[id], wantL)
					}
				}
			}
		})
	}
}
