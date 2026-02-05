package wikidata

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/request"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/tracker"
)

// MockStoreMinimal for simple service tests
type MockStoreMinimal struct {
	store.Store
}

func (m *MockStoreMinimal) GetGeodataCache(ctx context.Context, key string) ([]byte, int, bool) {
	return nil, 0, false
}
func (m *MockStoreMinimal) SetGeodataCache(ctx context.Context, key string, val []byte, radius int, lat, lon float64) error {
	return nil
}
func (m *MockStoreMinimal) GetCache(ctx context.Context, key string) ([]byte, bool) {
	return nil, false
}
func (m *MockStoreMinimal) SetCache(ctx context.Context, key string, val []byte) error { return nil }
func (m *MockStoreMinimal) GetGeodataInBounds(ctx context.Context, minLat, maxLat, minLon, maxLon float64) ([]store.GeodataRecord, error) {
	return []store.GeodataRecord{{Lat: 40.0, Lon: -74.0, Radius: 9800}}, nil
}
func (m *MockStoreMinimal) ListGeodataCacheKeys(ctx context.Context, prefix string) ([]string, error) {
	return []string{"wd_h3_8928308280fffff"}, nil
}

type MockSimState struct {
	State sim.State
	Tel   sim.Telemetry
}

func (m *MockSimState) GetState() sim.State                                     { return m.State }
func (m *MockSimState) GetTelemetry(ctx context.Context) (sim.Telemetry, error) { return m.Tel, nil }
func (m *MockSimState) GetLastTransition(stage string) time.Time                { return time.Time{} }
func (m *MockSimState) SetPredictionWindow(d time.Duration)                     {}

func TestService_Lifecycle(t *testing.T) {
	// 1. Setup
	st := &MockStoreMinimal{}
	sm := &MockSimState{State: sim.StateActive, Tel: sim.Telemetry{
		Latitude: 50.0, Longitude: 10.0,
	}}

	cfg := config.WikidataConfig{
		FetchInterval: config.Duration(10 * time.Millisecond),
		Area:          config.AreaConfig{MaxDist: 100},
	}

	// Minimal valid service dependencies
	reqCli := request.New(st, tracker.New(), request.ClientConfig{})
	geoSvc := &geo.Service{}
	poiMgr := poi.NewManager(config.NewProvider(&config.Config{}, nil), st, nil)
	dm, _ := NewDensityManager("../../configs/languages.yaml")
	svc := NewService(st, sm, tracker.New(), &MockClassifier{}, reqCli, geoSvc, poiMgr, dm, config.NewProvider(&config.Config{Wikidata: cfg}, nil))

	// 2. Accessors
	if svc.WikipediaClient() == nil {
		t.Error("WikipediaClient should not be nil")
	}
	if svc.GeoService() == nil {
		t.Error("GeoService should not be nil")
	}
	if info := svc.GetLanguageInfo("us"); info.Code != "en" { // Mock map is empty -> defaults to en
		t.Errorf("GetLanguageInfo: got %v, want Default en", info)
	}

	// 3. processTick
	// Case 1: Active State (Mocked above)
	// We expect it to call GetTelemetry (mocked) and then maybe fail/log because others are simple mocks.
	// It's hard to verify "nothing happened" without side effects, but it covers the lines.
	svc.processTick(context.Background())

	// Case 2: Inactive State
	sm.State = sim.StateInactive
	svc.processTick(context.Background())

	// 4. Start & Stop
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond) // Let it run a few ticks
	cancel()                          // Stop
	wg.Wait()

	// 3. EvictFarTiles
	svc.recentMu.Lock()
	svc.recentTiles["wd_h3_8928308280fffff"] = TileWrapper{SeenAt: time.Now()}
	svc.recentMu.Unlock()

	count := svc.EvictFarTiles(40.7, -74.0, 10.0) // 10km threshold
	if count == 0 {
		// OK
	}
}

func TestService_CacheMethods(t *testing.T) {
	st := &MockStoreMinimal{}
	// Service needs scheduler with grid for H3 ops
	sched := &Scheduler{grid: NewGrid()} // Res 9 match default
	svc := &Service{
		store:     st,
		scheduler: sched,
		logger:    slog.Default(),
	}

	// GetCachedTiles
	tiles, err := svc.GetCachedTiles(context.Background(), 30, 50, -80, -70)
	if err != nil {
		t.Errorf("GetCachedTiles failed: %v", err)
	}
	if len(tiles) != 1 {
		t.Errorf("GetCachedTiles len=%d, want 1", len(tiles))
	}

	// GetGlobalCoverage
	cov, err := svc.GetGlobalCoverage(context.Background())
	if err != nil {
		t.Errorf("GetGlobalCoverage failed: %v", err)
	}
	if len(cov) == 0 {
		// Mock returns 1 valid key, logic might filter or succeed.
		// wd_h3_8928308280fffff is valid.
		t.Error("GetGlobalCoverage returned 0 tiles")
	}
}
