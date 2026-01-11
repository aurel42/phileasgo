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
	}{
		{
			name: "Default Config",
			cfg: &config.Config{
				TTS: config.TTSConfig{Engine: "edge-tts"},
			},
			storeState:      map[string]string{},
			wantTTSEngine:   "edge-tts",
			wantSimSource:   "simconnect", // default
			wantCacheLayer:  false,
			wantFilterMode:  "fixed",
			wantTargetCount: 20,
		},
		{
			name: "Azure Config with Store Overrides",
			cfg: &config.Config{
				TTS: config.TTSConfig{Engine: "azure-speech"},
			},
			storeState: map[string]string{
				"sim_source":       "mock",
				"show_cache_layer": "true",
				"filter_mode":      "adaptive",
				"target_poi_count": "15",
			},
			wantTTSEngine:   "azure-speech",
			wantSimSource:   "mock",
			wantCacheLayer:  true,
			wantFilterMode:  "adaptive",
			wantTargetCount: 15,
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
			h := NewConfigHandler(st, tt.cfg)

			req := httptest.NewRequest("GET", "/api/config", nil)
			w := httptest.NewRecorder()

			h.HandleGetConfig(w, req)

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
		})
	}
}

func TestHandleSetConfig(t *testing.T) {
	st := &mockStore{state: make(map[string]string)}
	h := NewConfigHandler(st, &config.Config{})

	// Helper functions for pointers
	ptrInt := func(i int) *int { return &i }

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			req := httptest.NewRequest("PUT", "/api/config", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			h.HandleSetConfig(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200 OK, got %d", w.Code)
			}

			if val, ok := st.state[tt.wantKey]; !ok || val != tt.wantVal {
				t.Errorf("Store[%q] = %q, want %q", tt.wantKey, val, tt.wantVal)
			}
		})
	}
}
