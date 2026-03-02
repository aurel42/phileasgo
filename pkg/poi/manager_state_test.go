package poi

import (
	"context"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
	"phileasgo/pkg/sim"
)

func TestManager_StateMethods(t *testing.T) {
	cfg := config.NewProvider(&config.Config{}, nil)
	store := NewMockStore()
	mgr := NewManager(cfg, store, nil)

	ctx := context.Background()
	p1 := &model.POI{WikidataID: "Q1", NameEn: "POI 1"}
	p2 := &model.POI{WikidataID: "Q2", NameEn: "POI 2"}

	// Test Upsert and Tracking
	mgr.UpsertPOI(ctx, p1)
	mgr.UpsertPOI(ctx, p2)

	if mgr.ActiveCount() != 2 {
		t.Errorf("Expected 2 active POIs, got %d", mgr.ActiveCount())
	}

	// Test LastScoredPosition
	mgr.UpdateScoringState(10.0, 20.0)
	lat, lon := mgr.LastScoredPosition()
	if lat != 10.0 || lon != 20.0 {
		t.Errorf("Expected lat 10.0, lon 20.0, got %f, %f", lat, lon)
	}

	// Test GetPOIsNear
	near := mgr.GetPOIsNear(10.0, 20.0, 1000)
	// Both POIs are at 0,0 by default, so they shouldn't be near 10,20
	if len(near) != 0 {
		t.Errorf("Expected 0 POIs near 10,20, got %d", len(near))
	}

	p1.Lat = 10.0
	p1.Lon = 20.0
	mgr.UpsertPOI(ctx, p1)
	near = mgr.GetPOIsNear(10.0, 20.0, 1000)
	if len(near) != 1 {
		t.Errorf("Expected 1 POI near 10,20, got %d", len(near))
	}

	// Test ResetSession
	mgr.ResetSession(ctx)
	if mgr.ActiveCount() != 0 {
		t.Errorf("Expected 0 active POIs after reset, got %d", mgr.ActiveCount())
	}
	lat, lon = mgr.LastScoredPosition()
	if lat != 0 || lon != 0 {
		t.Errorf("Expected lat 0, lon 0 after reset, got %f, %f", lat, lon)
	}

	// Test LastPlayed Persistence
	mgr.UpsertPOI(ctx, p1)
	now := time.Now()
	mgr.SaveLastPlayed(ctx, "Q1", now)
	// Check if it's in the store (MockStore updates its map)
	sp, _ := store.GetPOI(ctx, "Q1")
	if sp.LastPlayed != now {
		t.Errorf("Expected LastPlayed %v, got %v", now, sp.LastPlayed)
	}

	// Test ResetLastPlayed
	mgr.ResetLastPlayed(ctx, 10.0, 20.0, 1000)
	if !p1.LastPlayed.IsZero() {
		t.Errorf("Expected LastPlayed to be zero after reset, got %v", p1.LastPlayed)
	}

	// Test FetchHistory
	p1.LastPlayed = time.Now()
	p1.Category = "castle"
	store.SavePOI(ctx, p1)
	history, err := mgr.FetchHistory(ctx)
	if err != nil {
		t.Fatalf("FetchHistory failed: %v", err)
	}
	if len(history) != 1 || history[0] != "castle" {
		t.Errorf("Expected history ['castle'], got %v", history)
	}

	// Test GetCategoriesConfig
	if mgr.GetCategoriesConfig() != nil {
		t.Error("Expected nil CategoriesConfig initially")
	}

	// Test SetValleyAltitudeCallback and NotifyScoringComplete (multiple)
	calledScoring := false
	valleyAlt := 0.0
	mgr.SetScoringCallback(func(ctx context.Context, t *sim.Telemetry) {
		calledScoring = true
	})
	mgr.SetValleyAltitudeCallback(func(altMeters float64) {
		valleyAlt = altMeters
	})
	mgr.NotifyScoringComplete(ctx, &sim.Telemetry{}, 100.0)
	if !calledScoring {
		t.Error("Scoring callback not called")
	}
	if valleyAlt != 100.0 {
		t.Errorf("Expected valley altitude 100.0, got %f", valleyAlt)
	}

	// Test ClearBeaconColor
	p1.BeaconColor = "red"
	mgr.UpsertPOI(ctx, p1)
	mgr.ClearBeaconColor("red")
	if p1.BeaconColor != "" {
		t.Error("Beacon color not cleared")
	}

	// Test GetBoostFactor with state
	store.SetState(ctx, "visibility_boost", "1.5")
	boost := mgr.GetBoostFactor(ctx)
	if boost != 1.5 {
		t.Errorf("Expected boost 1.5, got %f", boost)
	}

	// Test GetBoostFactor with invalid state
	store.SetState(ctx, "visibility_boost", "invalid")
	boost = mgr.GetBoostFactor(ctx)
	if boost != 1.0 {
		t.Errorf("Expected fallback boost 1.0 for invalid state, got %f", boost)
	}
}
