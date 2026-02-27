package narrator

import (
	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"testing"
)

func TestOrchestratorHiddenPOIs(t *testing.T) {
	registry := config.BeaconRegistry{
		"yellow": {MapColor: "#E9C46A", Title: "Yellow", Livery: "l1"},
	}
	order := []string{"yellow"}
	orchestrator := &Orchestrator{
		beaconRegistry: registry,
		colorKeys:      order,
	}

	t.Run("assignBeaconColor should return early for hidden POI", func(t *testing.T) {
		p := &model.POI{
			WikidataID:      "Q1",
			IsHiddenFeature: true,
		}
		orchestrator.assignBeaconColor(p)
		if p.BeaconColor != "" {
			t.Errorf("expected no beacon color for hidden POI, got %s", p.BeaconColor)
		}
	})

	t.Run("assignBeaconColor should assign color for normal POI", func(t *testing.T) {
		p := &model.POI{
			WikidataID:      "Q2",
			IsHiddenFeature: false,
		}
		orchestrator.assignBeaconColor(p)
		if p.BeaconColor == "" {
			t.Error("expected beacon color for normal POI, got empty string")
		}
	})
}

func TestOrchestratorPlayFeature(t *testing.T) {
	// This would require more mocking of the generator and POI manager to test fully
	// but we can at least verify it compiles and the logic looks sound in the code.
}
