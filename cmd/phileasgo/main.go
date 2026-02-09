package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"phileasgo/internal/api"
	"phileasgo/pkg/announcement"
	"phileasgo/pkg/audio"
	"phileasgo/pkg/beacon"
	"phileasgo/pkg/classifier"
	"phileasgo/pkg/config"
	"phileasgo/pkg/core"
	"phileasgo/pkg/db"
	"phileasgo/pkg/db/maintenance"
	"phileasgo/pkg/geo"
	"phileasgo/pkg/llm"
	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/logging"
	"phileasgo/pkg/narrator"
	"phileasgo/pkg/playback"
	"phileasgo/pkg/poi"
	"phileasgo/pkg/poi/rivers"
	"phileasgo/pkg/probe"
	"phileasgo/pkg/request"
	"phileasgo/pkg/scorer"
	"phileasgo/pkg/session"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/sim/mocksim"
	"phileasgo/pkg/store"
	"phileasgo/pkg/terrain"
	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts"
	"phileasgo/pkg/version"
	"phileasgo/pkg/visibility"
	"phileasgo/pkg/watcher"
	"phileasgo/pkg/wikidata"
	"phileasgo/pkg/wikipedia"
)

var initConfig = flag.Bool("init-config", false, "Generate default config file and exit")

func main() {
	flag.Parse()

	// Handle --init-config flag
	if *initConfig {
		if err := config.GenerateDefault("configs/phileas.yaml"); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Config file generated: configs/phileas.yaml")
		return
	}

	if err := run(context.Background(), "configs/phileas.yaml"); err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL ERROR: Application failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	appCfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cleanupLogs, err := logging.Init(&appCfg.Log, &appCfg.History)
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}
	defer cleanupLogs()

	// Configure History Logging
	tts.SetLogPath(appCfg.History.TTS.Path)
	tts.SetEnabled(appCfg.History.TTS.Enabled)

	slog.Info("PhileasGo Started", "version", version.Version)

	dbConn, st, err := initDB(appCfg)
	if err != nil {
		return err
	}
	defer dbConn.Close()

	// Initialize Unified Config Provider
	cfgProv := config.NewProvider(appCfg, st)

	if err := maintenance.Run(ctx, st, dbConn, "data/Master.csv"); err != nil {
		slog.Error("Maintenance tasks failed", "error", err)
	}

	simClient, err := initializeSimClient(ctx, cfgProv)
	if err != nil {
		return fmt.Errorf("failed to initialize sim client: %w", err)
	}
	defer simClient.Close()

	tr := tracker.New()
	catCfg, err := config.LoadCategories("configs/categories.yaml")
	if err != nil {
		return fmt.Errorf("failed to load categories config: %w", err)
	}

	svcs, densityMgr, err := initCoreServices(st, cfgProv, tr, simClient, catCfg)
	if err != nil {
		return err
	}
	go svcs.WikiSvc.Start(ctx)

	// LOS / Elevation
	elProv, losChecker := initElevation(appCfg)
	if elProv != nil {
		defer elProv.Close()

		// If using Mock Sim, inject coordinates
		if mc, ok := simClient.(*mocksim.MockClient); ok {
			slog.Info("Injecting ETOPO1 elevation provider into Mock Sim")
			mc.SetElevationProvider(elProv)
		}
	}

	// Startup Verification
	wdValidator := wikidata.NewValidator(svcs.WikiClient)
	verifyStartup(ctx, catCfg, wdValidator)

	// Narrator & TTS
	comps, err := initNarrator(ctx, cfgProv, svcs, tr, simClient, st, catCfg, elProv, densityMgr)
	if err != nil {
		return err
	}
	narratorSvc := comps.Orchestrator
	annMgr := comps.AnnManager
	promptMgr := comps.PromptManager
	sessionMgr := comps.SessionManager
	narratorSvc.Start()
	defer narratorSvc.Stop()

	// Connect Session Logic to Sim Logic (Event Recording)
	simClient.SetEventRecorder(sessionMgr)

	// Telemetry Handler (must be created before scheduler to receive updates)
	telH := api.NewTelemetryHandler()

	// Visibility
	visCalc := initVisibility(st)

	// Scheduler
	sched := setupScheduler(cfgProv, simClient, st, narratorSvc, annMgr, promptMgr, wdValidator, svcs, telH, losChecker, visCalc, sessionMgr)
	go sched.Start(ctx)

	// Session Persistence
	persistenceJob := core.NewSessionPersistenceJob(st, sessionMgr, simClient)
	persistenceJob.Start(ctx)

	// Scorer
	// Use elProv, or if nil (missing file), use a nil interface (Scorer must handle or we wrap)
	// Ideally Scorer should handle nil, but for now we pass it.
	// We need to cast *ElevationProvider to ElevationGetter.
	var elevGetter terrain.ElevationGetter
	if elProv != nil {
		elevGetter = elProv
	}
	// If elevGetter is nil, NewSession might crash.
	// Let's rely on Scorer handling nil optionally or just let it be nil for now.
	// The previous code verified startup files.
	poiScorer := scorer.NewScorer(&appCfg.Scorer, catCfg, visCalc, elevGetter, densityMgr, narratorSvc.LLMProvider().HasProfile("pregrounding"))

	// [NEW] Scoring Job
	scoringJob := poi.NewScoringJob("POIScoring", svcs.PoiMgr, simClient, poiScorer, cfgProv, narratorSvc.IsPOIBusy, slog.Default())
	sched.AddJob(scoringJob)

	// Startup Probes
	probes := []probe.Probe{
		{
			Name:     "LLM Models (Availability)",
			Check:    narratorSvc.LLMProvider().ValidateModels,
			Critical: true,
		},
	}
	// Optional: Add LOS probe if we want to surface it clearly
	// (LOS is already initialized at this point)
	if losChecker == nil {
		probes = append(probes, probe.Probe{
			Name:     "Terrain Data (ETOPO1)",
			Check:    func(context.Context) error { return fmt.Errorf("file not found or invalid") },
			Critical: false, // It's optional, app runs without it
		})
	}

	results := probe.Run(ctx, probes)
	if err := probe.AnalyzeResults(results); err != nil {
		return fmt.Errorf("startup checks failed: %w", err)
	}

	// Reset stats to ignore startup/validation calls
	tr.Reset()

	// Server
	return runServer(ctx, cfgProv, svcs, narratorSvc, simClient, visCalc, tr, st, telH, elevGetter, promptMgr, sessionMgr)
}

func initDB(appCfg *config.Config) (*db.DB, store.Store, error) {
	dbConn, err := db.Init(appCfg.DB.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return dbConn, store.NewSQLiteStore(dbConn), nil
}

func initCoreServices(st store.Store, cfg config.Provider, tr *tracker.Tracker, simClient sim.Client, catCfg *config.CategoriesConfig) (*CoreServices, *wikidata.DensityManager, error) {
	appCfg := cfg.AppConfig()
	geoSvc, err := geo.NewService("data/cities1000.txt", "data/admin1CodesASCII.txt")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize geo service: %w", err)
	}

	// Initialize CountryService for accurate country boundary detection (embedded data)
	countrySvc, err := geo.NewCountryServiceEmbedded()
	if err != nil {
		slog.Warn("CountryService not available", "error", err)
	} else {
		geoSvc.SetCountryService(countrySvc)
	}
	reqClient := request.New(st, tr, request.ClientConfig{
		Retries:   appCfg.Request.Retries,
		Timeout:   time.Duration(appCfg.Request.Timeout),
		BaseDelay: time.Duration(appCfg.Request.Backoff.BaseDelay),
		MaxDelay:  time.Duration(appCfg.Request.Backoff.MaxDelay),
	})

	poiMgr := poi.NewManager(cfg, st, catCfg)
	wikiClient := wikidata.NewClient(reqClient, slog.With("component", "wikidata_client"))
	smartClassifier := classifier.NewClassifier(st, wikiClient, catCfg, tr)
	wpClient := wikipedia.NewClient(reqClient)

	tr.SetFreeTier("wikidata", true)
	tr.SetFreeTier("wikipedia", true)

	densityMgr, err := wikidata.NewDensityManager("configs/languages.yaml")
	if err != nil {
		slog.Warn("Failed to initialize DensityManager, using defaults", "error", err)
	}

	wikiSvc := wikidata.NewService(st, simClient, tr, smartClassifier, reqClient, geoSvc, poiMgr, densityMgr, cfg)

	// River Sentinel Wiring (Phase 3)
	riverSentinel := rivers.NewSentinel(slog.With("component", "river_sentinel"), "data/ne_50m_rivers_lake_centerlines.geojson")
	poiMgr.SetRiverSentinel(riverSentinel)
	poiMgr.SetPOILoader(wikiSvc)

	return &CoreServices{
		WikiSvc:         wikiSvc,
		PoiMgr:          poiMgr,
		ReqClient:       reqClient,
		Classifier:      smartClassifier,
		WikiClient:      wikiClient,
		WikipediaClient: wpClient,
	}, densityMgr, nil
}

type NarratorComponents struct {
	Orchestrator   *narrator.Orchestrator
	AnnManager     *announcement.Manager
	PromptManager  *prompts.Manager
	SessionManager *session.Manager
}

func initNarrator(ctx context.Context, cfg config.Provider, svcs *CoreServices, tr *tracker.Tracker, simClient sim.Client, st store.Store, catCfg *config.CategoriesConfig, elProv *terrain.ElevationProvider, densityMgr *wikidata.DensityManager) (*NarratorComponents, error) {
	appCfg := cfg.AppConfig()
	llmProv, err := narrator.NewLLMProvider(appCfg.LLM, appCfg.History.LLM, svcs.ReqClient, tr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	// Configure temperature for narration prompts (bell curve distribution)
	if tc, ok := llmProv.(interface{ SetTemperature(base, jitter float32) }); ok {
		tc.SetTemperature(appCfg.Narrator.TemperatureBase, appCfg.Narrator.TemperatureJitter)
		slog.Debug("Configured LLM temperature", "base", appCfg.Narrator.TemperatureBase, "jitter", appCfg.Narrator.TemperatureJitter)
	}

	ttsProv, err := narrator.NewTTSProvider(&appCfg.TTS, cfg, tr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize TTS provider: %w", err)
	}

	promptMgr, err := prompts.NewManager("configs/prompts")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize prompt manager: %w", err)
	}

	sessionMgr := session.NewManager(simClient)

	var beaconSvc *beacon.Service
	// Initialize Beacon Service if enabled in config
	if appCfg.Beacon.Enabled {
		if bc, ok := simClient.(beacon.ObjectClient); ok {
			beaconSvc = beacon.NewService(bc, slog.With("component", "beacon"), cfg)
			if elProv != nil {
				beaconSvc.SetElevationProvider(elProv)
			}
			beaconSvc.SetDLLPath("SimConnect.dll")
			go beaconSvc.StartIndependentLoop(ctx)
		}
	}

	var beaconProvider narrator.BeaconProvider
	if beaconSvc != nil {
		beaconProvider = beaconSvc
	}

	pbQ := playback.NewManager()
	gen := createAIService(cfg, llmProv, ttsProv, promptMgr, svcs.PoiMgr, svcs.WikiSvc, simClient, st, tr, catCfg, sessionMgr, densityMgr)

	orch := narrator.NewOrchestrator(gen, audio.New(&appCfg.Narrator), pbQ, sessionMgr, beaconProvider, simClient)
	gen.SetOnPlayback(orch.EnqueuePlayback)

	// Restore Volume
	volStr, _ := st.GetState(ctx, "volume")
	if volStr != "" {
		var val float64
		if _, err := fmt.Sscanf(volStr, "%f", &val); err == nil {
			orch.AudioService().SetVolume(val)
		}
	}

	// Initialize Announcement Managers (Decoupled from AIService)
	annMgr := announcement.NewManager(gen, orch)
	annMgr.Register(announcement.NewLetsgo(appCfg, orch, sessionMgr))
	annMgr.Register(announcement.NewBriefing(appCfg, orch, sessionMgr))
	annMgr.Register(announcement.NewDebriefing(appCfg, orch, sessionMgr))
	annMgr.Register(announcement.NewBorder(appCfg, svcs.WikiSvc.GeoService(), orch, sessionMgr))

	return &NarratorComponents{
		Orchestrator:   orch,
		AnnManager:     annMgr,
		PromptManager:  promptMgr,
		SessionManager: sessionMgr,
	}, nil
}

func verifyStartup(ctx context.Context, catCfg *config.CategoriesConfig, v *wikidata.Validator) {
	// Use a dedicated timeout for startup verification to avoid inherited deadline issues
	verifyCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	catQIDs := make(map[string]string)
	for _, data := range catCfg.Categories {
		for qid, name := range data.QIDs {
			catQIDs[qid] = name
		}
	}
	_ = v.VerifyStartupConfig(verifyCtx, catQIDs)
}

func initVisibility(st store.Store) *visibility.Calculator {
	visManager, err := visibility.NewManager("configs/visibility.yaml")
	if err != nil {
		slog.Warn("Failed to load visibility config, using defaults", "error", err)
		return visibility.NewCalculator(nil, st)
	}
	return visibility.NewCalculator(visManager, st)
}

func initElevation(cfg *config.Config) (*terrain.ElevationProvider, *terrain.LOSChecker) {
	path := cfg.Terrain.ElevationFile
	if path == "" {
		path = "data/etopo1/etopo1_ice_g_i2.bin"
	}
	provider, err := terrain.NewElevationProvider(path)
	if err != nil {
		// Log but don't fail, LOS just won't work
		slog.Info("LOS: ETOPO1 data not found or invalid", "path", path, "error", err)
		return nil, nil
	}
	slog.Info("LOS: ETOPO1 Loaded", "path", path)
	return provider, terrain.NewLOSChecker(provider)
}

func runServer(ctx context.Context, cfg config.Provider, svcs *CoreServices, ns narrator.Service, simClient sim.Client, vis *visibility.Calculator, tr *tracker.Tracker, st store.Store, telH *api.TelemetryHandler, elevGetter terrain.ElevationGetter, promptMgr *prompts.Manager, sessionMgr *session.Manager) error {
	appCfg := cfg.AppConfig()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	shutdownFunc := func() { quit <- syscall.SIGTERM }

	statsH := api.NewStatsHandler(tr, svcs.PoiMgr, appCfg.LLM.Fallback)
	configH := api.NewConfigHandler(st, cfg)
	geoH := api.NewGeographyHandler(svcs.WikiSvc.GeoService())

	srv := api.NewServer(appCfg.Server.Address,
		telH,
		configH,
		statsH,
		api.NewCacheHandler(svcs.WikiSvc),
		api.NewPOIHandler(svcs.PoiMgr, svcs.WikipediaClient, st, cfg, ns.LLMProvider(), promptMgr),
		api.NewVisibilityHandler(vis, simClient, elevGetter, st, svcs.WikiSvc),
		api.NewAudioHandler(ns.AudioService(), ns, st),
		api.NewNarratorHandler(ns.AudioService(), ns, st),
		api.NewImageHandler(appCfg),
		geoH,
		api.NewTripHandler(sessionMgr, st),
		shutdownFunc,
	)

	srv.Handler = loggingMiddleware(srv.Handler)
	return runServerLifecycle(ctx, srv, quit)
}

func setupScheduler(cfg config.Provider, simClient sim.Client, st store.Store, narratorSvc narrator.Service, annMgr *announcement.Manager, pm *prompts.Manager, v *wikidata.Validator, svcs *CoreServices, apiHandler *api.TelemetryHandler, los *terrain.LOSChecker, vis *visibility.Calculator, sessionMgr *session.Manager) *core.Scheduler {
	appCfg := cfg.AppConfig()
	sched := core.NewScheduler(cfg, simClient, apiHandler, svcs.WikiSvc.GeoService())
	// Session Restoration (Restores session state on startup)
	sched.AddJob(core.NewSessionRestorationJob(st, sessionMgr, simClient))

	sched.AddJob(core.NewDistanceJob("DistanceSync", 5000, func(c context.Context, t sim.Telemetry) {
		_ = st.MarkEntitiesSeen(c, map[string][]string{})
	}))

	// Register Resettables for Teleport Detection
	sched.AddResettable(narratorSvc)
	sched.AddResettable(svcs.PoiMgr)
	sched.AddResettable(annMgr)
	sched.AddResettable(sessionMgr)

	// Register Cleanup Job (runs every 10s)
	sched.AddJob(core.NewTimeJob("CacheCleanup", 10*time.Second, func(c context.Context, t sim.Telemetry) {
		// Clean up old cache entries if needed
	}))

	// Register Announcement Jobs (Standard) - 1Hz
	sched.AddJob(core.NewTimeJob("Announcements", 1*time.Second, func(c context.Context, t sim.Telemetry) {
		annMgr.Tick(c, &t)
	}))

	// Register River Job (runs every 15s, detects nearby rivers)
	sched.AddJob(core.NewRiverJob(svcs.PoiMgr))

	// Register Debrief Job (implicitly added by NewScheduler via debriefer arg)

	// Watcher for Screenshots
	var screenWatcher *watcher.Service
	if appCfg.Narrator.Screenshot.Enabled {
		var err error
		screenWatcher, err = watcher.NewService(appCfg.Narrator.Screenshot.Paths)
		if err != nil {
			slog.Warn("Failed to initialize screenshot watcher", "error", err)
		} else {
			slog.Info("Screenshot watcher started", "paths", appCfg.Narrator.Screenshot.Paths)
			// Register Screenshot Announcement
			annMgr.Register(announcement.NewScreenshot(appCfg, screenWatcher, narratorSvc, sessionMgr))
		}
	}

	// Hook NarrationJob into POI Manager's scoring loop (every 5s) instead of Scheduler
	narrationJob := core.NewNarrationJob(cfg, narratorSvc, narratorSvc.POIManager(), simClient, st, los)
	svcs.PoiMgr.SetScoringCallback(func(c context.Context, t *sim.Telemetry) {
		// 1. Process Sync Priority Queue (Manual Overrides)
		if narratorSvc.HasPendingGeneration() {
			narratorSvc.ProcessGenerationQueue(c)
			return
		}

		// 3. Auto Narrations
		if narrationJob.CanPreparePOI(c, t) {
			if narrationJob.PreparePOI(c, t) {
				return
			}
		}
		if narrationJob.CanPrepareEssay(c, t) {
			narrationJob.PrepareEssay(c, t)
			return
		}
	})
	svcs.PoiMgr.SetValleyAltitudeCallback(func(altMeters float64) {
		apiHandler.SetValleyAltitude(altMeters)
	})

	dynamicJob := core.NewDynamicConfigJob(cfg, narratorSvc.LLMProvider(), pm, v, svcs.Classifier, svcs.WikiSvc.GeoService(), svcs.WikiSvc)
	sched.AddJob(dynamicJob)
	sched.AddResettable(dynamicJob)

	sched.AddJob(core.NewEvictionJob(cfg, svcs.PoiMgr, svcs.WikiSvc))

	// Transponder Control
	if appCfg.Transponder.Enabled {
		sched.AddJob(core.NewTransponderWatcherJob(cfg, narratorSvc, st, vis))
	}

	return sched
}

func runServerLifecycle(ctx context.Context, srv *http.Server, quit chan os.Signal) error {
	slog.Info("Starting server", "addr", srv.Addr)
	serverErrors := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()
	select {
	case <-quit:
		slog.Info("Shutting down server...")
	case <-ctx.Done():
		slog.Info("Context cancelled, shutting down...")
	case err := <-serverErrors:
		return fmt.Errorf("server failed: %w", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logging.RequestLogger.Info("Request Processed", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

type CoreServices struct {
	WikiSvc         *wikidata.Service
	PoiMgr          *poi.Manager
	ReqClient       *request.Client
	Classifier      *classifier.Classifier
	WikiClient      *wikidata.Client
	WikipediaClient *wikipedia.Client
}

func createAIService(cfg config.Provider, llmProv llm.Provider, ttsProv tts.Provider, promptMgr *prompts.Manager, poiMgr narrator.POIProvider, wikiSvc *wikidata.Service, simClient sim.Client, st store.Store, tr *tracker.Tracker, catCfg *config.CategoriesConfig, sessionMgr *session.Manager, densityMgr *wikidata.DensityManager) *narrator.AIService {
	essayConfig := "configs/essays.yaml"
	essayH, _ := narrator.NewEssayHandler(essayConfig, promptMgr)

	// Load interests config
	interestsCfg, err := config.LoadInterests("configs/interests.yaml")
	var interests []string
	var avoid []string
	if err != nil {
		slog.Warn("Failed to load interests config, using empty list", "error", err)
	} else {
		interests = interestsCfg.Interests
		avoid = interestsCfg.Avoid
	}

	return narrator.NewAIService(
		cfg,
		llmProv,
		ttsProv,
		promptMgr,
		poiMgr,
		wikiSvc.GeoService(),
		simClient,
		st,
		wikiSvc.WikipediaClient(),
		wikiSvc,
		catCfg,
		essayH,
		interests,
		avoid,
		tr,
		sessionMgr,
		densityMgr,
	)
}
