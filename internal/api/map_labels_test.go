package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/map/labels"
	"testing"
)

func TestHandleSync(t *testing.T) {
	manager := labels.NewManager(&geo.Service{}, nil, nil)
	handler := NewMapLabelsHandler(manager)

	reqPayload := SyncRequest{
		BBox:    [4]float64{40, 0, 60, 20},
		ACLat:   50,
		ACLon:   10,
		Heading: 90,
		Zoom:    10,
	}
	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest("POST", "/api/map/labels/sync?sid=test", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.HandleSync(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var respData SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(respData.Labels) != 0 {
		t.Errorf("Expected 0 labels from empty service, got %d", len(respData.Labels))
	}
}
