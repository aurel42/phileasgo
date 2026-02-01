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

	if err := maintenance.Run(ctx, st, dbConn, "data/Master.csv"); err != nil {
		slog.Error("Maintenance tasks failed", "error", err)
	}

	simClient, err := initializeSimClient(appCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize sim client: %w", err)
	}
	defer simClient.Close()

	tr := tracker.New()
	catCfg, err := config.LoadCategories("configs/categories.yaml")
	if err != nil {
		return fmt.Errorf("failed to load categories config: %w", err)
	}

	svcs, err := initCoreServices(st, appCfg, tr, simClient, catCfg)
	if err != nil {
		return err
	}
	go svcs.WikiSvc.Start(ctx)

	// Startup Verification
	wdValidator := wikidata.NewValidator(svcs.WikiClient)
	verifyStartup(ctx, catCfg, wdValidator)

	// Narrator & TTS
	comps, err := initNarrator(ctx, appCfg, svcs, tr, simClient, st, catCfg)
	if err != nil {
		return err
	}
	narratorSvc := comps.AIService
	annMgr := comps.AnnManager
	promptMgr := comps.PromptManager
	sessionMgr := comps.SessionManager
	narratorSvc.Start()
	defer narratorSvc.Stop()

	// Connect Session Logic to Sim Logic (Event Recording)
	simClient.SetEventRecorder(sessionMgr)

	// Telemetry Handler (must be created before scheduler to receive updates)
	telH := api.NewTelemetryHandler()

	// LOS
	elProv, losChecker := initLOS(appCfg)
	if elProv != nil {
		defer elProv.Close()

		// If using Mock Sim, inject coordinates
		if mc, ok := simClient.(*mocksim.MockClient); ok {
			slog.Info("Injecting ETOPO1 elevation provider into Mock Sim")
			mc.SetElevationProvider(elProv)
		}
	}

	// Visibility
	visCalc := initVisibility(st)

	// Scheduler
	sched := setupScheduler(appCfg, simClient, st, narratorSvc, annMgr, promptMgr, wdValidator, svcs, telH, losChecker, visCalc, sessionMgr)
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
	poiScorer := scorer.NewScorer(&appCfg.Scorer, catCfg, visCalc, elevGetter, narratorSvc.LLMProvider().HasProfile("pregrounding"))

	// [NEW] Scoring Job
	scoringJob := poi.NewScoringJob("POIScoring", svcs.PoiMgr, simClient, poiScorer, &appCfg.Narrator, narratorSvc.IsPOIBusy, slog.Default())
	sched.AddJob(scoringJob)

	// Startup Probes
	probes := []probe.Probe{
		{
			Name:     "LLM Providers",
			Check:    narratorSvc.LLMProvider().HealthCheck,
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

	// Server
	return runServer(ctx, appCfg, svcs, narratorSvc, simClient, visCalc, tr, st, telH, elevGetter, promptMgr)
}

func initDB(appCfg *config.Config) (*db.DB, store.Store, error) {
	dbConn, err := db.Init(appCfg.DB.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return dbConn, store.NewSQLiteStore(dbConn), nil
}

func initCoreServices(st store.Store, cfg *config.Config, tr *tracker.Tracker, simClient sim.Client, catCfg *config.CategoriesConfig) (*CoreServices, error) {
	geoSvc, err := geo.NewService("data/cities1000.txt", "data/admin1CodesASCII.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize geo service: %w", err)
	}

	// Initialize CountryService for accurate country boundary detection (embedded data)
	countrySvc, err := geo.NewCountryServiceEmbedded()
	if err != nil {
		slog.Warn("CountryService not available", "error", err)
	} else {
		geoSvc.SetCountryService(countrySvc)
	}
	reqClient := request.New(st, tr, request.ClientConfig{
		Retries:   cfg.Request.Retries,
		Timeout:   time.Duration(cfg.Request.Timeout),
		BaseDelay: time.Duration(cfg.Request.Backoff.BaseDelay),
		MaxDelay:  time.Duration(cfg.Request.Backoff.MaxDelay),
	})
	poiMgr := poi.NewManager(cfg, st, catCfg)
	wikiClient := wikidata.NewClient(reqClient, slog.With("component", "wikidata_client"))
	smartClassifier := classifier.NewClassifier(st, wikiClient, catCfg, tr)
	wpClient := wikipedia.NewClient(reqClient)

	tr.SetFreeTier("wikidata", true)
	tr.SetFreeTier("wikipedia", true)

	wikiSvc := wikidata.NewService(st, simClient, tr, smartClassifier, reqClient, geoSvc, poiMgr, cfg.Wikidata, cfg.Narrator.TargetLanguage)

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
	}, nil
}

type NarratorComponents struct {
	AIService      *narrator.AIService
	AnnManager     *announcement.Manager
	PromptManager  *prompts.Manager
	SessionManager *session.Manager
}

func initNarrator(ctx context.Context, cfg *config.Config, svcs *CoreServices, tr *tracker.Tracker, simClient sim.Client, st store.Store, catCfg *config.CategoriesConfig) (*NarratorComponents, error) {
	llmProv, err := narrator.NewLLMProvider(cfg.LLM, cfg.History.LLM, svcs.ReqClient, tr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	// Configure temperature for narration prompts (bell curve distribution)
	if tc, ok := llmProv.(interface{ SetTemperature(base, jitter float32) }); ok {
		tc.SetTemperature(cfg.Narrator.TemperatureBase, cfg.Narrator.TemperatureJitter)
		slog.Debug("Configured LLM temperature", "base", cfg.Narrator.TemperatureBase, "jitter", cfg.Narrator.TemperatureJitter)
	}

	ttsProv, err := narrator.NewTTSProvider(&cfg.TTS, cfg.Narrator.TargetLanguage, tr)
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
	if cfg.Beacon.Enabled {
		if bc, ok := simClient.(beacon.ObjectClient); ok {
			beaconSvc = beacon.NewService(bc, slog.With("component", "beacon"), &cfg.Beacon)
			beaconSvc.SetDLLPath("SimConnect.dll")
			go beaconSvc.StartIndependentLoop(ctx)
		}
	}

	narratorSvc := createAIService(cfg, llmProv, ttsProv, promptMgr, audio.New(&cfg.Narrator), svcs.PoiMgr, beaconSvc, svcs.WikiSvc, simClient, st, tr, catCfg, sessionMgr)

	// Restore Volume
	volStr, _ := st.GetState(ctx, "volume")
	if volStr != "" {
		var val float64
		if _, err := fmt.Sscanf(volStr, "%f", &val); err == nil {
			narratorSvc.AudioService().SetVolume(val)
		}
	}

	// Initialize Announcement Managers (Decoupled from AIService)
	annMgr := announcement.NewManager(narratorSvc)
	annMgr.Register(announcement.NewLetsgo(cfg, narratorSvc, sessionMgr))
	annMgr.Register(announcement.NewBriefing(cfg, narratorSvc, sessionMgr))
	annMgr.Register(announcement.NewDebriefing(cfg, narratorSvc, sessionMgr))
	annMgr.Register(announcement.NewBorder(cfg, svcs.WikiSvc.GeoService(), narratorSvc, sessionMgr))

	return &NarratorComponents{
		AIService:      narratorSvc,
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

func initLOS(cfg *config.Config) (*terrain.ElevationProvider, *terrain.LOSChecker) {
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

func runServer(ctx context.Context, cfg *config.Config, svcs *CoreServices, ns *narrator.AIService, simClient sim.Client, vis *visibility.Calculator, tr *tracker.Tracker, st store.Store, telH *api.TelemetryHandler, elevGetter terrain.ElevationGetter, promptMgr *prompts.Manager) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	shutdownFunc := func() { quit <- syscall.SIGTERM }

	statsH := api.NewStatsHandler(tr, svcs.PoiMgr, cfg.LLM.Fallback)
	configH := api.NewConfigHandler(st, cfg)
	geoH := api.NewGeographyHandler(svcs.WikiSvc.GeoService())

	srv := api.NewServer(cfg.Server.Address,
		telH,
		configH,
		statsH,
		api.NewCacheHandler(svcs.WikiSvc),
		api.NewPOIHandler(svcs.PoiMgr, svcs.WikipediaClient, st, ns.LLMProvider(), promptMgr),
		api.NewVisibilityHandler(vis, simClient, elevGetter, st, svcs.WikiSvc),
		api.NewAudioHandler(ns.AudioService(), ns, st),
		api.NewNarratorHandler(ns.AudioService(), ns, st),
		api.NewImageHandler(cfg),
		geoH,
		shutdownFunc,
	)

	srv.Handler = loggingMiddleware(srv.Handler)
	return runServerLifecycle(ctx, srv, quit)
}

func setupScheduler(cfg *config.Config, simClient sim.Client, st store.Store, narratorSvc *narrator.AIService, annMgr *announcement.Manager, pm *prompts.Manager, v *wikidata.Validator, svcs *CoreServices, apiHandler *api.TelemetryHandler, los *terrain.LOSChecker, vis *visibility.Calculator, sessionMgr *session.Manager) *core.Scheduler {
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
	if cfg.Narrator.Screenshot.Enabled {
		var err error
		screenWatcher, err = watcher.NewService(cfg.Narrator.Screenshot.Paths)
		if err != nil {
			slog.Warn("Failed to initialize screenshot watcher", "error", err)
		} else {
			slog.Info("Screenshot watcher started", "paths", cfg.Narrator.Screenshot.Paths)
			// Register Screenshot Announcement
			annMgr.Register(announcement.NewScreenshot(cfg, screenWatcher, narratorSvc, sessionMgr))
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
		// 3. Auto Narrations
		if narrationJob.CanPreparePOI(t) {
			if narrationJob.PreparePOI(c, t) {
				return
			}
		}
		if narrationJob.CanPrepareEssay(t) {
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
	if cfg.Transponder.Enabled {
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

func createAIService(
	appCfg *config.Config,
	llmProv llm.Provider,
	ttsProv tts.Provider,
	promptMgr *prompts.Manager,
	audioMgr audio.Service,
	poiMgr narrator.POIProvider,
	beaconSvc *beacon.Service,
	wikiSvc *wikidata.Service,
	simClient sim.Client,
	st store.Store,
	tr *tracker.Tracker,
	catCfg *config.CategoriesConfig,
	sessionMgr *session.Manager,
) *narrator.AIService {
	var beaconProvider narrator.BeaconProvider
	if beaconSvc != nil {
		beaconProvider = beaconSvc
	}
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
		appCfg,
		llmProv,
		ttsProv,
		promptMgr,
		audioMgr,
		poiMgr,
		beaconProvider,
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
	)
}
