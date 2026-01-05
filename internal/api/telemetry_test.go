package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"phileasgo/pkg/sim"
)

func TestTelemetryHandler_HandleTelemetry(t *testing.T) {
	defaultTel := sim.Telemetry{
		Latitude:    51.5,
		Longitude:   -0.1,
		AltitudeMSL: 1000,
		IsOnGround:  false,
	}

	tests := []struct {
		name           string
		setup          func(*TelemetryHandler)
		expectedStatus int
		validate       func(*testing.T, sim.Telemetry)
	}{
		{
			name: "Success_WithData",
			setup: func(h *TelemetryHandler) {
				h.Update(&defaultTel)
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, tel sim.Telemetry) {
				if tel.Latitude != defaultTel.Latitude {
					t.Errorf("got Lat %v, want %v", tel.Latitude, defaultTel.Latitude)
				}
			},
		},
		{
			name: "Success_EmptyInitial",
			setup: func(h *TelemetryHandler) {
				// No update, should return zero value struct
			},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, tel sim.Telemetry) {
				if tel.Latitude != 0 {
					t.Errorf("got Lat %v, want 0", tel.Latitude)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewTelemetryHandler()
			if tt.setup != nil {
				tt.setup(handler)
			}

			req := httptest.NewRequest("GET", "/api/telemetry", http.NoBody)
			w := httptest.NewRecorder()

			handler.handleTelemetry(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("StatusCode: got %v, want %v", resp.StatusCode, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusOK && tt.validate != nil {
				var gotTel sim.Telemetry
				if err := json.NewDecoder(resp.Body).Decode(&gotTel); err != nil {
					t.Fatalf("failed to decode JSON: %v", err)
				}
				tt.validate(t, gotTel)
			}
		})
	}
}
