package beacon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
	"unsafe"

	"phileasgo/pkg/sim"
	"phileasgo/pkg/sim/simconnect"
)

const (
	// Request IDs for internal tracking
	reqIDSpawnTarget = 100
	reqIDSpawnForm1  = 101
	reqIDSpawnForm2  = 102
	reqIDSpawnForm3  = 103
	reqIDRemove      = 100 // ID for remove requests (can be shared)

	// SimConnect IDs for independent handle
	reqIDHighFreq  = 1
	DefIDTelemetry = 0
	DefIDObjectPos = 1

	// Configuration
	FormationDistKm = 2.0
	TriggerDistKm   = 3.0
	FormationAltTop = 200.0
	FormationAltMid = 0.0
	FormationAltBot = -200.0
	UpdateInterval  = 50 * time.Millisecond // ~20Hz
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
}

type SpawnedBeacon struct {
	ID        uint32
	IsTarget  bool
	AltOffset float64 // Offset relative to current s.targetAlt
}

// NewService creates a new Beacon Service.
func NewService(client ObjectClient, logger *slog.Logger) *Service {
	return &Service{
		client: client,
		logger: logger,
	}
}

// SetTarget initializes the guidance system towards a target coordinate.
// It spawns the target beacon and formation beacons.
func (s *Service) SetTarget(ctx context.Context, lat, lon float64) error {
	if s == nil {
		return fmt.Errorf("beacon service is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Clear existing if any
	if s.active {
		s.clearLocked()
	}

	s.targetLat = lat
	s.targetLon = lon

	// Get current user pos to calculate initial formation spawn
	tel, err := s.client.GetTelemetry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get telemetry for spawn: %w", err)
	}

	// 2. Select Title
	title := titlesToTry[0] // TODO: Try loop if fails? (Spawn blocks now so we could)

	// 3. Spawning Logic based on AGL
	// If below 1000ft AGL:
	// - Spawn target at MSL + 1000 ft
	// - Do NOT spawn formation
	// If above 1000ft AGL:
	// - Spawn target at MSL
	// - Spawn formation

	var spawnFormation bool
	s.isHoldingAlt = false // Reset holding state

	if tel.AltitudeAGL < 1000.0 {
		s.targetAlt = tel.AltitudeMSL + 1000.0
		spawnFormation = false
		s.isHoldingAlt = true // Lock immediately if spawned low
		s.logger.Info("Low AGL, spawning target at +1000ft", "agl", tel.AltitudeAGL, "target_alt", s.targetAlt, "formation", false)
	} else {
		s.targetAlt = tel.AltitudeMSL
		spawnFormation = true
		s.logger.Info("Spawning Target Beacon", "lat", lat, "lon", lon, "alt", s.targetAlt)
	}

	targetID, err := s.client.SpawnAirTraffic(reqIDSpawnTarget, title, "TGT", lat, lon, s.targetAlt, 0)
	if err != nil {
		return fmt.Errorf("failed to spawn target: %w", err)
	}
	s.spawnedBeacons = append(s.spawnedBeacons, SpawnedBeacon{ID: targetID, IsTarget: true, AltOffset: 0.0})

	// 4. Spawn Formation (if active)
	if spawnFormation {
		hdgRad := tel.Heading * (math.Pi / 180.0)
		latRad := tel.Latitude * (math.Pi / 180.0)

		// Calc initial formation pos
		formLat, formLon := calculateNewPos(tel.Latitude, tel.Longitude, hdgRad, latRad, FormationDistKm)

		formReqs := []struct {
			reqID     uint32
			altOffset float64
		}{
			{reqIDSpawnForm1, FormationAltTop},
			{reqIDSpawnForm2, FormationAltMid},
			{reqIDSpawnForm3, FormationAltBot},
		}

		for _, req := range formReqs {
			absAlt := s.targetAlt + req.altOffset
			id, err := s.client.SpawnAirTraffic(req.reqID, title, "FORM", formLat, formLon, absAlt, tel.Heading)
			if err != nil {
				s.logger.Error("Error spawning formation beacon", "error", err)
				continue
			}
			s.spawnedBeacons = append(s.spawnedBeacons, SpawnedBeacon{ID: id, IsTarget: false, AltOffset: req.altOffset})
		}
		s.formationActive = true
	} else {
		s.formationActive = false
	}

	s.active = true
	s.logger.Info("Beacon system SetTarget complete", "active_beacons", len(s.spawnedBeacons))

	return nil
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
			s.updateStep(tel)
		}
	}
	if recv.ID == simconnect.RECV_ID_QUIT {
		s.logger.Info("Simulator quit detected in beacon loop")
		return false
	}
	return true
}

func (s *Service) updateStep(tel *simconnect.TelemetryData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active || len(s.spawnedBeacons) == 0 {
		return
	}

	// 1. Calculate Vector to Target
	dLat := s.targetLat - tel.Latitude
	dLon := s.targetLon - tel.Longitude
	latRad := tel.Latitude * (math.Pi / 180.0)

	// Meters/Km conversion
	dy := dLat * 111.0
	dx := dLon * 111.0 * math.Cos(latRad)
	distKm := math.Sqrt(dx*dx + dy*dy)

	// Bearing
	bearingRad := math.Atan2(dx, dy)
	bearingDeg := bearingRad * (180.0 / math.Pi)
	if bearingDeg < 0 {
		bearingDeg += 360.0
	}

	// 2. Check Trigger Distance
	if s.formationActive && distKm < TriggerDistKm {
		s.logger.Info("Target Distance < Trigger Distance. Despawning formation.", "dist_km", distKm, "trigger_km", TriggerDistKm)

		var kept []SpawnedBeacon
		for _, b := range s.spawnedBeacons {
			if !b.IsTarget {
				// Always use s.client for remove - objects were spawned via s.client
				_ = s.client.RemoveObject(b.ID, reqIDRemove)
			} else {
				kept = append(kept, b)
			}
		}
		s.spawnedBeacons = kept
		s.formationActive = false
	}

	// 3. Update Positions
	// Logic:
	// - If AGL >= 2000ft: Balloons follow aircraft MSL (targetAlt = aircraft MSL)
	// - If AGL < 2000ft: Balloons lock at last good MSL (targetAlt = held value)
	const SafetyFloorAGL = 2000.0

	// Note: We use s.targetAlt as the "Base MSL" for the formation
	if tel.AltitudeAGL >= SafetyFloorAGL {
		s.targetAlt = tel.AltitudeMSL
		s.isHoldingAlt = false
	} else if !s.isHoldingAlt {
		// Below safety floor
		// If we weren't holding already, we lock now.
		// If we spawned low, isHoldingAlt might be false initially, but SetTarget sets initial s.targetAlt
		// based on spawn rules, so we just set holding=true and KEEP the existing s.targetAlt.
		s.logger.Debug("Below 2000ft AGL, holding beacon altitude", "agl", tel.AltitudeAGL, "hold_msl", s.targetAlt)
		s.isHoldingAlt = true
	}
	// targetAlt remains unchanged in the else case (whether we just locked or were already locked)

	// Calculate Formation Target Pos
	formLat, formLon := calculateNewPos(tel.Latitude, tel.Longitude, bearingRad, latRad, FormationDistKm)

	for _, b := range s.spawnedBeacons {
		// Calculate final absolute altitude for this specific beacon
		absAlt := s.targetAlt + b.AltOffset

		var upd simconnect.MarkerUpdateData
		if b.IsTarget {
			upd = simconnect.MarkerUpdateData{
				Latitude:    s.targetLat,
				Longitude:   s.targetLon,
				AltitudeMSL: absAlt,
				Heading:     0,
			}
		} else {
			upd = simconnect.MarkerUpdateData{
				Latitude:    formLat,
				Longitude:   formLon,
				AltitudeMSL: absAlt,
				Heading:     bearingDeg,
			}
		}

		// Move it
		if s.handle != 0 {
			_ = simconnect.SetDataOnSimObject(s.handle, DefIDObjectPos, b.ID, 0, 0, uint32(unsafe.Sizeof(upd)), unsafe.Pointer(&upd))
		} else {
			_ = s.client.SetObjectPosition(b.ID, upd.Latitude, upd.Longitude, upd.AltitudeMSL, upd.Pitch, upd.Bank, upd.Heading)
		}
	}
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
