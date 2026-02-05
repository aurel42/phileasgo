package beacon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
	"unsafe"

	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/sim/simconnect"
	"phileasgo/pkg/terrain"
)

const (
	// Request IDs for internal tracking
	reqIDSpawnTarget = 100
	// reqIDSpawnForm1..3 replaced by dynamic generation

	reqIDRemove = 100 // ID for remove requests (can be shared)

	// SimConnect IDs for independent handle
	reqIDHighFreq  = 1
	DefIDTelemetry = 0
	DefIDObjectPos = 1

	// Configuration (Defaults / Fallbacks)
	UpdateInterval = 50 * time.Millisecond // ~20Hz
)

var (
	// Titles to try for spawning
	titlesToTry = []string{
		"Asobo PassiveAircraft Hot Air Balloon",
		"Generic Hot Air Balloon",
		"Airbus A320 Neo Asobo",
		"Cessna Skyhawk Asobo",
	}
)

// ObjectClient combines the needed interfaces for this service
type ObjectClient interface {
	sim.Client
	sim.ObjectClient
}

// Service manages the guiding beacons.
type Service struct {
	client ObjectClient
	logger *slog.Logger
	prov   config.Provider

	dllPath string
	handle  uintptr // Independent SimConnect handle

	mu           sync.Mutex
	active       bool
	targetLat    float64
	targetLon    float64
	targetAlt    float64
	isHoldingAlt bool

	spawnedBeacons  []SpawnedBeacon
	formationActive bool

	elev terrain.ElevationGetter
}

type SpawnedBeacon struct {
	ID        uint32
	IsTarget  bool
	AltOffset float64 // Offset relative to its specific targetAlt
	Lat       float64 // POI Latitude
	Lon       float64 // POI Longitude
	BaseAlt   float64 // POI Base Altitude
}

// NewService creates a new Beacon Service.
func NewService(client ObjectClient, logger *slog.Logger, prov config.Provider) *Service {
	return &Service{
		client: client,
		logger: logger,
		prov:   prov,
	}
}

// SetElevationProvider injects a terrain elevation provider.
func (s *Service) SetElevationProvider(e terrain.ElevationGetter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.elev = e
}

func computeFormationOffsets(count int) []float64 {
	if count < 1 {
		count = 1
	}
	if count > 5 {
		count = 5
	}

	const step = 200.0
	offsets := make([]float64, count)
	mid := float64(count-1) / 2.0
	for i := 0; i < count; i++ {
		offsets[i] = step * (float64(i) - mid)
	}
	return offsets
}

// SetTarget initializes the guidance system towards a target coordinate.
// It spawns the target beacon and formation beacons.
func (s *Service) SetTarget(ctx context.Context, lat, lon float64) error {
	if s == nil {
		return fmt.Errorf("beacon service is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Check for redundant calls
	if s.active {
		const threshold = 0.0001
		if math.Abs(s.targetLat-lat) < threshold && math.Abs(s.targetLon-lon) < threshold {
			return nil
		}
	}

	// 2. Clear formation and duplicates
	s.clearFormationAndDuplicates(lat, lon)

	s.targetLat = lat
	s.targetLon = lon

	tel, err := s.client.GetTelemetry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get telemetry for spawn: %w", err)
	}

	if tel.IsOnGround {
		s.active = true
		return nil
	}

	// 3. Setup altitude and spawn
	title := titlesToTry[0]
	spawnFormation := s.setupTargetAltitude(ctx, &tel)

	// 4. Spawn Beacons
	s.spawnTargetBalloon(title, lat, lon)
	s.enforceTargetQuota(ctx)

	if spawnFormation {
		s.spawnFormationBalloons(ctx, title, &tel)
	} else {
		s.formationActive = false
	}

	s.active = true
	s.logger.Info("Beacon system SetTarget complete", "active_beacons", len(s.spawnedBeacons))
	return nil
}

func (s *Service) clearFormationAndDuplicates(lat, lon float64) {
	const threshold = 0.0001
	newSpawned := []SpawnedBeacon{}
	for _, b := range s.spawnedBeacons {
		isSamePOI := math.Abs(b.Lat-lat) < threshold && math.Abs(b.Lon-lon) < threshold
		if !b.IsTarget || isSamePOI {
			_ = s.client.RemoveObject(b.ID, reqIDRemove)
		} else {
			newSpawned = append(newSpawned, b)
		}
	}
	s.spawnedBeacons = newSpawned
}

func (s *Service) setupTargetAltitude(ctx context.Context, tel *sim.Telemetry) bool {
	s.targetAlt = tel.AltitudeMSL
	minSpawnAltFt := float64(s.prov.BeaconMinSpawnAltitude(ctx)) * 3.28084
	spawnFormation := s.prov.BeaconFormationEnabled(ctx)

	if tel.AltitudeAGL < minSpawnAltFt {
		s.targetAlt = tel.AltitudeMSL + minSpawnAltFt
		spawnFormation = false
		s.isHoldingAlt = true
		s.logger.Info("Low AGL, spawning target", "agl", tel.AltitudeAGL, "target_alt", s.targetAlt, "formation", false)
	} else {
		s.isHoldingAlt = false
		s.logger.Info("Spawning Target Beacon", "lat", s.targetLat, "lon", s.targetLon, "alt", s.targetAlt)
	}
	return spawnFormation
}

func (s *Service) spawnTargetBalloon(title string, lat, lon float64) {
	targetID, err := s.client.SpawnAirTraffic(reqIDSpawnTarget, title, "TGT", lat, lon, s.targetAlt, 0)
	if err == nil {
		s.spawnedBeacons = append(s.spawnedBeacons, SpawnedBeacon{
			ID:       targetID,
			IsTarget: true,
			Lat:      lat,
			Lon:      lon,
			BaseAlt:  s.targetAlt,
		})
	} else {
		s.logger.Error("Failed to spawn target beacon", "error", err)
	}
}

func (s *Service) enforceTargetQuota(ctx context.Context) {
	targetCount := 0
	for _, b := range s.spawnedBeacons {
		if b.IsTarget {
			targetCount++
		}
	}
	maxTargets := s.prov.BeaconMaxTargets(ctx)
	for targetCount > maxTargets {
		for i, b := range s.spawnedBeacons {
			if b.IsTarget {
				_ = s.client.RemoveObject(b.ID, reqIDRemove)
				s.spawnedBeacons = append(s.spawnedBeacons[:i], s.spawnedBeacons[i+1:]...)
				targetCount--
				break
			}
		}
	}
}

func (s *Service) spawnFormationBalloons(ctx context.Context, title string, tel *sim.Telemetry) {
	formationCount := s.prov.BeaconFormationCount(ctx)
	if formationCount <= 0 {
		s.formationActive = false
		return
	}

	bearingRad, _ := s.calculateBearing(tel.Latitude, tel.Longitude, s.targetLat, s.targetLon)
	bearingDeg := bearingRad * 180.0 / math.Pi
	distKm := float64(s.prov.BeaconFormationDistance(ctx)) / 1000.0
	latRad := tel.Latitude * (math.Pi / 180.0)
	fLat, fLon := calculateNewPos(tel.Latitude, tel.Longitude, bearingRad, latRad, distKm)

	offsets := computeFormationOffsets(formationCount)
	baseReqID := uint32(200)
	for i, offset := range offsets {
		absAlt := s.targetAlt + offset
		reqID := baseReqID + uint32(i)
		id, err := s.client.SpawnAirTraffic(reqID, title, "FORM", fLat, fLon, absAlt, bearingDeg)
		if err == nil {
			s.spawnedBeacons = append(s.spawnedBeacons, SpawnedBeacon{
				ID:        id,
				IsTarget:  false,
				AltOffset: offset,
				Lat:       fLat,
				Lon:       fLon,
			})
		}
	}
	s.formationActive = true
}

// Helper: Calculate bearing between two points in radians.
func (s *Service) calculateBearing(lat1, lon1, lat2, lon2 float64) (bearingRad, distKm float64) {
	lat1Rad := lat1 * math.Pi / 180.0
	lon1Rad := lon1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	lon2Rad := lon2 * math.Pi / 180.0

	dLon := lon2Rad - lon1Rad

	y := math.Sin(dLon) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(dLon)
	bearingRad = math.Atan2(y, x)

	// distance in km for convenience
	const R = 6371.0
	dLat := lat2Rad - lat1Rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distKm = R * c

	return bearingRad, distKm
}

// Clear removes all beacons.
func (s *Service) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
}

func (s *Service) clearLocked() {
	if !s.active {
		return
	}

	for _, b := range s.spawnedBeacons {
		// Best effort remove
		_ = s.client.RemoveObject(b.ID, reqIDRemove)
	}
	s.spawnedBeacons = []SpawnedBeacon{}
	s.active = false
	s.formationActive = false
	s.logger.Info("Cleared all beacons")
}

// SetDLLPath provides the path to SimConnect.dll for the independent connection.
func (s *Service) SetDLLPath(path string) {
	s.dllPath = path
}

// Connect establishes the independent SimConnect connection for high-frequency updates.
func (s *Service) Connect() error {
	if s.dllPath == "" {
		return fmt.Errorf("DLL path not set")
	}
	// Assume DLL is already loaded by the main client (simconnect.LoadDLL is global)
	h, err := simconnect.Open("PhileasBeacons")
	if err != nil {
		return err
	}
	s.handle = h

	// 1. Setup Definitions
	// User Telemetry
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "PLANE LATITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "PLANE LONGITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "PLANE ALTITUDE", "feet", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "PLANE ALT ABOVE GROUND", "feet", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "PLANE HEADING DEGREES TRUE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "GROUND VELOCITY", "knots", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDTelemetry, "SIM ON GROUND", "Bool", simconnect.DATATYPE_INT32)

	// Object Position Update
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "PLANE LATITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "PLANE LONGITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "PLANE ALTITUDE", "feet", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "PLANE PITCH DEGREES", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "PLANE BANK DEGREES", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "PLANE HEADING DEGREES TRUE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "SIM ON GROUND", "Bool", simconnect.DATATYPE_INT32)
	_ = simconnect.AddToDataDefinition(h, DefIDObjectPos, "AIRSPEED TRUE", "knots", simconnect.DATATYPE_INT32)

	// 2. Request Data
	// PERIOD_VISUAL_FRAME with interval=1 (update every frame for max smoothness)
	// TESTED: Perfect signal smoothness. DO NOT CHANGE.
	err = simconnect.RequestDataOnSimObject(h, reqIDHighFreq, DefIDTelemetry, simconnect.OBJECT_ID_USER, simconnect.PERIOD_VISUAL_FRAME, 0, 0, 1, 0)
	if err != nil {
		_ = simconnect.Close(h)
		return err
	}
	return nil
}

// StartIndependentLoop runs the frame-driven update thread.
// It handles connection and retries automatically.
func (s *Service) StartIndependentLoop(ctx context.Context) {
	retryTicker := time.NewTicker(s.retryInterval())
	defer retryTicker.Stop()

	for {
		if s.handle == 0 {
			if err := s.Connect(); err != nil {
				// Use DEBUG to avoid console spam at startup
				// s.logger is set to fmt.Printf in NewService, let's keep it simple for now
				// or maybe we should use slog here? Service struct doesn't have it yet.
				// But s.logger is what we have.
				time.Sleep(100 * time.Millisecond) // brief pause before next loop iteration
			} else {
				s.logger.Info("Starting independent frame-driven update loop")
			}
		}

		if s.handle != 0 {
			// Inner loop for dispatching
			success := s.runDispatchIteration()
			if !success {
				// Handle became invalid
				s.mu.Lock()
				_ = simconnect.Close(s.handle)
				s.handle = 0
				s.mu.Unlock()
				s.logger.Warn("SimConnect handle lost, returning to retry loop")
			}
		}

		select {
		case <-ctx.Done():
			s.mu.Lock()
			if s.handle != 0 {
				_ = simconnect.Close(s.handle)
				s.handle = 0
			}
			s.mu.Unlock()
			return
		case <-retryTicker.C:
			// Just trigger next iteration
		default:
			if s.handle == 0 {
				// Wait for ticker if not connected
				select {
				case <-ctx.Done():
					return
				case <-retryTicker.C:
					// Proceed
				}
			}
		}
	}
}

func (s *Service) retryInterval() time.Duration {
	return 60 * time.Second
}

func (s *Service) runDispatchIteration() bool {
	ppData, _, err := simconnect.GetNextDispatch(s.handle)
	if err != nil {
		s.logger.Error("Dispatch error", "error", err)
		return false
	}
	if ppData == nil {
		// Match simtest responsiveness (2ms instead of 10ms)
		// TESTED: Perfect smoothness at 2ms. DO NOT CHANGE.
		time.Sleep(2 * time.Millisecond)
		return true
	}

	recv := (*simconnect.Recv)(ppData)
	if recv.ID == simconnect.RECV_ID_SIMOBJECT_DATA {
		data := (*simconnect.RecvSimobjectData)(ppData)
		if data.RequestID == reqIDHighFreq {
			dataPtr := unsafe.Pointer(uintptr(ppData) + unsafe.Sizeof(simconnect.RecvSimobjectData{}))
			tel := (*simconnect.TelemetryData)(dataPtr)
			// Independent update has no obvious ctx, use Background
			s.updateStep(context.Background(), tel)
		}
	}
	if recv.ID == simconnect.RECV_ID_QUIT {
		s.logger.Info("Simulator quit detected in beacon loop")
		return false
	}
	return true
}

func (s *Service) updateStep(ctx context.Context, tel *simconnect.TelemetryData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active || len(s.spawnedBeacons) == 0 {
		return
	}

	// 1. Check for formation cleanup
	bearingRad, distKm := s.calculateBearing(tel.Latitude, tel.Longitude, s.targetLat, s.targetLon)
	s.checkFormationCleanup(ctx, distKm)

	// 2. Update aircraft-relative state
	altFloorFt := float64(s.prov.BeaconAltitudeFloor(ctx)) * 3.28084
	if tel.AltitudeAGL >= altFloorFt {
		s.targetAlt = tel.AltitudeMSL
		s.isHoldingAlt = false
	} else if !s.isHoldingAlt {
		s.isHoldingAlt = true
	}

	// 3. Update all beacons
	s.updateAllBeacons(ctx, tel, bearingRad)
}

func (s *Service) checkFormationCleanup(ctx context.Context, distKm float64) {
	triggerDistKm := (float64(s.prov.BeaconFormationDistance(ctx)) / 1000.0) * 1.5
	if s.formationActive && distKm < triggerDistKm {
		s.logger.Info("Target Distance < Trigger Distance. Despawning formation.", "dist_km", distKm, "trigger_km", triggerDistKm)
		var kept []SpawnedBeacon
		for _, b := range s.spawnedBeacons {
			if !b.IsTarget {
				_ = s.client.RemoveObject(b.ID, reqIDRemove)
			} else {
				kept = append(kept, b)
			}
		}
		s.spawnedBeacons = kept
		s.formationActive = false
	}
}

func (s *Service) updateAllBeacons(ctx context.Context, tel *simconnect.TelemetryData, bearingRad float64) {
	latRad := tel.Latitude * math.Pi / 180.0
	formDistance := float64(s.prov.BeaconFormationDistance(ctx))
	formLat, formLon := calculateNewPos(tel.Latitude, tel.Longitude, bearingRad, latRad, formDistance/1000.0)

	kept := []SpawnedBeacon{}
	for _, b := range s.spawnedBeacons {
		bBearingRad, bDistKm := s.calculateBearing(tel.Latitude, tel.Longitude, b.Lat, b.Lon)
		if b.IsTarget && s.isBeaconStale(tel, bBearingRad, bDistKm) {
			s.logger.Info("Despawning stale target balloon", "id", b.ID, "dist", bDistKm)
			_ = s.client.RemoveObject(b.ID, reqIDRemove)
			continue
		}

		var absAlt float64
		var lat, lon float64
		if b.IsTarget {
			absAlt = s.calculateTargetAltitude(ctx, tel, b.Lat, b.Lon, b.BaseAlt, bDistKm*1000.0) + b.AltOffset
			lat, lon = b.Lat, b.Lon
		} else {
			absAlt = s.targetAlt + b.AltOffset
			lat, lon = formLat, formLon
		}

		if s.updateObjectOnSim(b.ID, lat, lon, absAlt) {
			kept = append(kept, b)
		}
	}
	s.spawnedBeacons = kept
}

func (s *Service) isBeaconStale(tel *simconnect.TelemetryData, bearingRad, distKm float64) bool {
	hdgRad := tel.Heading * math.Pi / 180.0
	diff := bearingRad - hdgRad
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}

	isBehind := math.Abs(diff) > math.Pi/2.0
	return distKm > 50.0 && isBehind
}

func (s *Service) updateObjectOnSim(id uint32, lat, lon, alt float64) bool {
	upd := simconnect.MarkerUpdateData{
		Latitude:    lat,
		Longitude:   lon,
		AltitudeMSL: alt,
	}

	var err error
	if s.handle != 0 {
		err = simconnect.SetDataOnSimObject(s.handle, DefIDObjectPos, id, 0, 0, uint32(unsafe.Sizeof(upd)), unsafe.Pointer(&upd))
	} else {
		err = s.client.SetObjectPosition(id, lat, lon, alt, 0, 0, 0)
	}

	if err != nil {
		s.logger.Debug("Failed to update beacon position", "id", id, "error", err)
		_ = s.client.RemoveObject(id, reqIDRemove)
		return false
	}
	return true
}

// Helper: Calculate new coord given origin, heading(rad), and dist(km)
// Note: latRad is passed for cosine calc
func calculateNewPos(lat, lon, hdgRad, latRad, distKm float64) (newLat, newLon float64) {
	dLat := (distKm / 111.0) * math.Cos(hdgRad)

	cosLat := math.Cos(latRad)
	if math.Abs(cosLat) < 0.0001 {
		cosLat = 0.0001
	}
	dLon := (distKm / (111.0 * cosLat)) * math.Sin(hdgRad)

	return lat + dLat, lon + dLon
}
