package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
	"testing"
)

func TestNewFeaturesHandler(t *testing.T) {
	tests := []struct {
		name       string
		featureSvc *geo.FeatureService
		tel        *TelemetryHandler
		wantNil    bool
	}{
		{
			name:       "nil featureSvc",
			featureSvc: nil,
			tel:        &TelemetryHandler{},
			wantNil:    true,
		},
		{
			name:       "nil telemetry",
			featureSvc: &geo.FeatureService{},
			tel:        nil,
			wantNil:    true,
		},
		{
			name:       "success",
			featureSvc: &geo.FeatureService{},
			tel:        &TelemetryHandler{},
			wantNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewFeaturesHandler(tt.featureSvc, tt.tel)
			if (h == nil) != tt.wantNil {
				t.Errorf("NewFeaturesHandler() got nil = %v, wantNil %v", h == nil, tt.wantNil)
			}
		})
	}
}

func TestFeaturesHandler_HandleGet(t *testing.T) {
	// We can't easily mock geo.FeatureService without GeoJSON files,
	// but we can test the nil guard and the telemetry fallback logic.

	tel := NewTelemetryHandler()
	tel.Update(&sim.Telemetry{
		Latitude:  45.0,
		Longitude: 5.0,
	})

	h := &FeaturesHandler{
		featureSvc: nil, // Should be caught by the nil guard in HandleGet
		telemetry:  tel,
	}

	req := httptest.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()

	h.HandleGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []FeatureResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 0 {
		t.Errorf("expected empty list for nil service, got %d items", len(resp))
	}
}

func TestFeaturesHandler_QueryParams(t *testing.T) {
	tel := NewTelemetryHandler()
	// telemetry is NOT updated, but we provide query params

	h := &FeaturesHandler{
		featureSvc: nil, // Still testing nil guard but with query params
		telemetry:  tel,
	}

	req := httptest.NewRequest("GET", "/api/features?lat=10.0&lon=20.0", nil)
	rr := httptest.NewRecorder()

	h.HandleGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}
