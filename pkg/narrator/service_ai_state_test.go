package narrator

import (
	"testing"

	"phileasgo/pkg/generation"
	"phileasgo/pkg/model"
)

func TestAIService_StateChecks(t *testing.T) {
	svc := &AIService{
		generating: true,
		genQ:       generation.NewManager(),
	}

	if !svc.IsGenerating() {
		t.Error("expected generating")
	}
	if !svc.IsActive() {
		t.Error("expected active (when generating)")
	}

	svc.mu.Lock()
	svc.generating = false
	svc.mu.Unlock()
	if svc.IsGenerating() {
		t.Error("expected not generating")
	}
}

func TestAIService_POIBusy(t *testing.T) {
	svc := &AIService{
		genQ: generation.NewManager(),
	}

	poiID := "Q123"

	if svc.IsPOIBusy(poiID) {
		t.Error("expected not busy")
	}

	svc.mu.Lock()
	svc.generatingPOI = &model.POI{WikidataID: poiID}
	svc.mu.Unlock()

	if !svc.IsPOIBusy(poiID) {
		t.Error("expected busy (generating)")
	}

	svc.mu.Lock()
	svc.generatingPOI = nil
	svc.mu.Unlock()

	svc.genQ.Enqueue(&generation.Job{POIID: poiID})
	if !svc.IsPOIBusy(poiID) {
		t.Error("expected busy (queued)")
	}
}
