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

// Mock Geo Service or just use the real one with a small file?
// For integration test, using the real Manager with a real (but small) Geo service is best.
// However, creating a GeoService requires a file.
// We can construct a Manager with a mock GeoService if we extracted an interface, but we didn't.
// We can assume the test environment has access to data, or we can make a dummy file.
// Better: Refactor Manager to take an Interface.
// B1.3 implementation of Manager used `*geo.Service`.
// We should probably rely on the fact that `geo.NewService` can digest a small string if we write it to a temp file.

func TestHandleSync(t *testing.T) {

	// Create temp file
	// Actually, for this test, we can just rely on the existing integration test patterns or mocks.
	// But let's try to just spin up the handler without data and see if it returns empty list (valid test).
	// If we want data, we need to mock GetCitiesInBbox.

	// Since we can't easily mock *geo.Service (struct), we'll skip data-dependent validation
	// and just validat input/output structure and error handling.
	// Real data validation was done in B1.4 (Manager Logic).

	manager := labels.NewManager(&geo.Service{}, nil) // Empty service
	handler := NewMapLabelsHandler(manager)

	// 2. Create Request
	reqPayload := SyncRequest{
		BBox:    [4]float64{40, 0, 60, 20},
		ACLat:   50,
		ACLon:   10,
		Heading: 90,
	}
	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest("POST", "/api/map/labels/sync", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	// 3. Execute
	handler.HandleSync(w, req)

	// 4. Verify
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

func TestHandleCheckShadow(t *testing.T) {
	manager := labels.NewManager(&geo.Service{}, nil)
	handler := NewMapLabelsHandler(manager)

	reqPayload := CheckShadowRequest{
		ACLat:   50,
		ACLon:   10,
		Heading: 90,
	}
	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest("POST", "/api/map/labels/check-shadow", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.HandleCheckShadow(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var respData CheckShadowResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respData.Shadow != false {
		t.Errorf("Expected Shadow=false from empty service, got true")
	}
}
