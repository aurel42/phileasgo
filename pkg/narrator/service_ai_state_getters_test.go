package narrator

import (
	"phileasgo/pkg/model"
	"testing"
)

func TestOrchestrator_StateGetters(t *testing.T) {
	o := &Orchestrator{
		currentPOI:       &model.POI{NameEn: "Current"},
		currentImagePath: "test.jpg",
	}

	if o.CurrentPOI() == nil || o.CurrentPOI().NameEn != "Current" {
		t.Errorf("expected Current POI, got %v", o.CurrentPOI())
	}

	if o.CurrentImagePath() != "test.jpg" {
		t.Errorf("expected test.jpg, got %s", o.CurrentImagePath())
	}
}
