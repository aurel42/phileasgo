package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/config"
	"phileasgo/pkg/store"
)

type mockStore struct {
	store.Store
	state map[string]string
}

func (m *mockStore) GetState(ctx context.Context, key string) (string, bool) {
	val, ok := m.state[key]
	return val, ok
}

func (m *mockStore) SetState(ctx context.Context, key, val string) error {
	if m.state == nil {
		m.state = make(map[string]string)
	}
	m.state[key] = val
	return nil
}

func (m *mockStore) DeleteState(ctx context.Context, key string) error {
	delete(m.state, key)
	return nil
}

func TestHandleGetConfig(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *config.Config
		storeState      map[string]string
		wantTTSEngine   string
		wantSimSource   string
		wantCacheLayer  bool
		wantFilterMode  string
		wantTargetCount int
		wantTextLength  int
		wantDeferral    float64
		wantBoostPower  float64
	}{
		{
			name: "Default Config",
			cfg: &config.Config{
				TTS:    config.TTSConfig{Engine: "edge-tts"},
				Scorer: config.ScorerConfig{DeferralThreshold: 1.05, DeferralProximityBoostPower: 1.0},
			},
			storeState:      map[string]string{},
			wantTTSEngine:   "edge-tts",
			wantSimSource:   "simconnect", // default fallback
			wantCacheLayer:  false,
			wantFilterMode:  "fixed", // restored default
			wantTargetCount: 5,       // new default
			wantTextLength:  3,
			wantDeferral:    1.05,
			wantBoostPower:  1.0,
		},
		{
			name: "Azure Config with Store Overrides",
			cfg: &config.Config{
				TTS: config.TTSConfig{Engine: "azure-speech"},
			},
			storeState: map[string]string{
				"sim_source":                            "mock",
				"show_cache_layer":                      "true",
				"filter_mode":                           "adaptive",
				"target_poi_count":                      "15",
				"text_length":                           "4",
				"scorer.deferral_threshold":             "1.1",
				"scorer.deferral_proximity_boost_power": "1.2",
			},
			wantTTSEngine:   "azure-speech",
			wantSimSource:   "mock",
			wantCacheLayer:  true,
			wantFilterMode:  "adaptive",
			wantTargetCount: 15,
			wantTextLength:  4,
			wantDeferral:    1.1,
			wantBoostPower:  1.2,
		},
		{
			name: "Custom Adaptive Config",
			cfg:  &config.Config{},
			storeState: map[string]string{
				"filter_mode":      "adaptive",
				"target_poi_count": "15",
			},
			wantFilterMode:  "adaptive",
			wantTargetCount: 15,
			wantSimSource:   "simconnect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &mockStore{state: tt.storeState}
			h := NewConfigHandler(st, config.NewProvider(tt.cfg, st))

			req := httptest.NewRequest("GET", "/api/config", nil)
			w := httptest.NewRecorder()

			h.HandleConfig(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status OK, got %v", resp.Status)
			}

			var got ConfigResponse
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if got.TTSEngine != tt.wantTTSEngine {
				t.Errorf("TTSEngine = %q, want %q", got.TTSEngine, tt.wantTTSEngine)
			}
			if got.SimSource != tt.wantSimSource {
				t.Errorf("SimSource = %q, want %q", got.SimSource, tt.wantSimSource)
			}
			if got.ShowCacheLayer != tt.wantCacheLayer {
				t.Errorf("ShowCacheLayer = %v, want %v", got.ShowCacheLayer, tt.wantCacheLayer)
			}
			if got.FilterMode != tt.wantFilterMode && tt.wantFilterMode != "" {
				t.Errorf("FilterMode = %q, want %q", got.FilterMode, tt.wantFilterMode)
			}
			if got.TargetPOICount != tt.wantTargetCount && tt.wantTargetCount != 0 {
				t.Errorf("TargetPOICount = %d, want %d", got.TargetPOICount, tt.wantTargetCount)
			}
			if got.TextLength != tt.wantTextLength && tt.wantTextLength != 0 {
				t.Errorf("TextLength = %d, want %d", got.TextLength, tt.wantTextLength)
			}
			if got.DeferralThreshold != tt.wantDeferral && tt.wantDeferral != 0 {
				t.Errorf("DeferralThreshold = %f, want %f", got.DeferralThreshold, tt.wantDeferral)
			}
			if got.DeferralProximityBoostPower != tt.wantBoostPower && tt.wantBoostPower != 0 {
				t.Errorf("DeferralProximityBoostPower = %f, want %f", got.DeferralProximityBoostPower, tt.wantBoostPower)
			}
		})
	}
}

func TestHandleSetConfig(t *testing.T) {
	st := &mockStore{state: make(map[string]string)}
	h := NewConfigHandler(st, config.NewProvider(&config.Config{}, st))

	// Helper functions for pointers
	ptrInt := func(i int) *int { return &i }
	ptrBool := func(b bool) *bool { return &b }
	ptrFloat := func(f float64) *float64 { return &f }
	ptrString := func(s string) *string { return &s }

	tests := []struct {
		name    string
		req     ConfigRequest
		wantKey string
		wantVal string
	}{
		{
			name:    "Update Sim Source",
			req:     ConfigRequest{SimSource: "mock"},
			wantKey: "sim_source",
			wantVal: "mock",
		},
		{
			name:    "Update Units",
			req:     ConfigRequest{Units: "hybrid"},
			wantKey: "units",
			wantVal: "hybrid",
		},
		{
			name:    "Update Range Ring Units",
			req:     ConfigRequest{RangeRingUnits: "nm"},
			wantKey: "range_ring_units",
			wantVal: "nm",
		},
		{
			name:    "Update Active Target Language",
			req:     ConfigRequest{ActiveTargetLanguage: ptrString("de-DE")},
			wantKey: "active_target_language",
			wantVal: "de-DE",
		},
		{
			name:    "Update Filter Mode",
			req:     ConfigRequest{FilterMode: "adaptive"},
			wantKey: "filter_mode",
			wantVal: "adaptive",
		},
		{
			name:    "Update Target Count",
			req:     ConfigRequest{TargetPOICount: ptrInt(25)},
			wantKey: "target_poi_count",
			wantVal: "25",
		},
		{
			name:    "Update Text Length",
			req:     ConfigRequest{TextLength: ptrInt(5)},
			wantKey: "text_length",
			wantVal: "5",
		},
		{
			name:    "Update Boolean True",
			req:     ConfigRequest{ShowCacheLayer: ptrBool(true)},
			wantKey: "show_cache_layer",
			wantVal: "true",
		},
		{
			name:    "Update Boolean False",
			req:     ConfigRequest{ShowCacheLayer: ptrBool(false)},
			wantKey: "show_cache_layer",
			wantVal: "false",
		},
		{
			name:    "Update Float Score",
			req:     ConfigRequest{MinPOIScore: ptrFloat(0.75)},
			wantKey: "min_poi_score",
			wantVal: "0.75",
		},
		{
			name:    "Update Deferral Threshold",
			req:     ConfigRequest{DeferralThreshold: ptrFloat(1.1)},
			wantKey: "scorer.deferral_threshold",
			wantVal: "1.10",
		},
		{
			name:    "Update Deferral Boost Power",
			req:     ConfigRequest{DeferralProximityBoostPower: ptrFloat(1.5)},
			wantKey: "scorer.deferral_proximity_boost_power",
			wantVal: "1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			// Test both POST and PUT as both should be supported now
			methods := []string{"POST", "PUT"}
			for _, method := range methods {
				req := httptest.NewRequest(method, "/api/config", bytes.NewBuffer(body))
				w := httptest.NewRecorder()

				h.HandleConfig(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("method %s: expected 200 OK, got %d. Body: %s", method, w.Code, w.Body.String())
				}

				if val, ok := st.state[tt.wantKey]; !ok || val != tt.wantVal {
					t.Errorf("method %s: Store[%q] = %q, want %q", method, tt.wantKey, val, tt.wantVal)
				}

				// Verify CORS headers
				if w.Header().Get("Access-Control-Allow-Origin") != "*" {
					t.Errorf("method %s: missing CORS header Access-Control-Allow-Origin", method)
				}
			}
		})
	}

	t.Run("CORS and OPTIONS", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/config", nil)
		w := httptest.NewRecorder()
		h.HandleConfig(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("OPTIONS: expected 200 OK, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("OPTIONS: missing Access-Control-Allow-Methods")
		}
	})

	t.Run("Invalid Sim Source", func(t *testing.T) {
		body, _ := json.Marshal(ConfigRequest{SimSource: "invalid"})
		req := httptest.NewRequest("POST", "/api/config", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		h.HandleConfig(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Invalid Units", func(t *testing.T) {
		body, _ := json.Marshal(ConfigRequest{Units: "km"}) // km is now invalid for Units (must be imperial/hybrid/metric)
		req := httptest.NewRequest("POST", "/api/config", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		h.HandleConfig(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}
