package narrator

import (
	"phileasgo/pkg/model"
	"testing"
)

func TestAIService_StateGetters(t *testing.T) {
	svc := &AIService{
		currentPOI:       &model.POI{NameEn: "Current"},
		currentImagePath: "test.jpg",
	}

	if svc.CurrentPOI() == nil || svc.CurrentPOI().NameEn != "Current" {
		t.Errorf("expected Current POI, got %v", svc.CurrentPOI())
	}

	if svc.CurrentImagePath() != "test.jpg" {
		t.Errorf("expected test.jpg, got %s", svc.CurrentImagePath())
	}
}
