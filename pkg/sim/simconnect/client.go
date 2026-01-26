// Package simconnect provides a SimConnect client for Microsoft Flight Simulator.
package simconnect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
	"unsafe"

	"phileasgo/pkg/geo"
	"phileasgo/pkg/sim"
)

// cStringToGo converts a null-terminated C string byte array to a Go string.
func cStringToGo(b []byte) string {
	if idx := bytes.IndexByte(b, 0); idx >= 0 {
		return string(b[:idx])
	}
	return string(b)
}

// Constants for Camera States (from 80days/backend/logic/sim_helpers.py)
const (
	CameraStateCockpit   = 2
	CameraStateChase     = 3
	CameraStateDrone     = 4
	CameraStateCockpitVR = 30
	CameraStateChaseVR   = 34
)

// InFlightStates set of valid camera states for active flight.
var InFlightStates = map[int]bool{
	CameraStateCockpit:   true,
	CameraStateChase:     true,
	CameraStateDrone:     true,
	CameraStateCockpitVR: true,
	CameraStateChaseVR:   true,
}

// Request and Definition IDs
const (
	DefIDTelemetry = 0
	DefIDObjectPos = 1 // New definition for setting object data
	ReqIDTelemetry = 0
	EvtIDSimStop   = 0 // Client-side ID for SimStop
)

// Client implements sim.Client for Microsoft Flight Simulator via SimConnect.
type Client struct {
	handle           uintptr
	connected        bool
	stopChan         chan struct{}
	telemetry        sim.Telemetry
	cameraState      int32
	simState         sim.State
	telemetryMu      sync.RWMutex
	logger           *slog.Logger
	appName          string
	dllPath          string
	reconnectInt     time.Duration
	predictionWindow time.Duration

	// Spawning synchronization
	spawnMu       sync.Mutex
	pendingSpawns map[uint32]chan uint32

	// Watchdog
	lastMessageTime time.Time

	// Ground Track Calculation
	trackBuf *geo.TrackBuffer

	// Telemetry Validity
	hasValidData bool
}

// NewClient creates a new SimConnect client.
// If dllPath is empty, it will attempt to auto-discover SimConnect.dll.
func NewClient(appName, dllPath string) (*Client, error) {
	// Auto-discover DLL if path is empty
	if dllPath == "" {
		var err error
		dllPath, err = FindDLL()
		if err != nil {
			return nil, fmt.Errorf("failed to find SimConnect.dll: %w", err)
		}
	}

	c := &Client{
		connected:        false,
		simState:         sim.StateDisconnected,
		stopChan:         make(chan struct{}),
		logger:           slog.Default().With("component", "simconnect"),
		appName:          appName,
		dllPath:          dllPath,
		reconnectInt:     5 * time.Second,
		predictionWindow: 60 * time.Second,
		pendingSpawns:    make(map[uint32]chan uint32),
		trackBuf:         geo.NewTrackBuffer(5),
	}

	// Load DLL
	if err := LoadDLL(dllPath); err != nil {
		return nil, err
	}

	go c.connectionLoop()

	return c, nil
}

// GetTelemetry returns the latest telemetry state.
func (c *Client) GetTelemetry(ctx context.Context) (sim.Telemetry, error) {
	c.telemetryMu.RLock()
	defer c.telemetryMu.RUnlock()

	if !c.hasValidData {
		return sim.Telemetry{}, sim.ErrWaitingForTelemetry
	}
	return c.telemetry, nil
}

// GetState returns the current simulator connection/activity state.
func (c *Client) GetState() sim.State {
	c.telemetryMu.RLock()
	defer c.telemetryMu.RUnlock()
	return c.simState
}

// SetPredictionWindow sets the time duration for future position prediction.
func (c *Client) SetPredictionWindow(d time.Duration) {
	c.telemetryMu.Lock()
	defer c.telemetryMu.Unlock()
	c.predictionWindow = d
}

// Close disconnects and cleans up.
func (c *Client) Close() error {
	close(c.stopChan)
	if c.handle != 0 {
		return Close(c.handle)
	}
	return nil
}

// SpawnAirTraffic spawns a non-ATC aircraft (AI object) and returns its ObjectID.
func (c *Client) SpawnAirTraffic(reqID uint32, title, tailNum string, lat, lon, alt, hdg float64) (uint32, error) {
	initPos := InitPosition{
		Latitude:    lat,
		Longitude:   lon,
		AltitudeMSL: alt,
		Pitch:       0,
		Bank:        0,
		Heading:     hdg,
		OnGround:    0,
		Airspeed:    0,
	}

	// Ensure connected
	if !c.connected {
		return 0, sim.ErrNotConnected
	}

	// Create response channel
	respChan := make(chan uint32, 1)

	c.spawnMu.Lock()
	c.pendingSpawns[reqID] = respChan
	c.spawnMu.Unlock()

	// Clean up map on exit if timed out or failed
	// We only remove if it's still there (i.e., timeout or error)
	// Actually, careful with double close if we delete.
	// Best pattern: The receiver closes or we rely on map removal.
	defer func() {
		c.spawnMu.Lock()
		delete(c.pendingSpawns, reqID)
		c.spawnMu.Unlock()
	}()

	if err := AICreateNonATCAircraft(c.handle, title, tailNum, &initPos, reqID); err != nil {
		return 0, err
	}

	// Wait for ID
	select {
	case id := <-respChan:
		return id, nil
	case <-time.After(5 * time.Second):
		return 0, errors.New("timeout waiting for object spawn")
	}
}

// RemoveObject removes a sim object by its ID.
func (c *Client) RemoveObject(objectID, reqID uint32) error {
	if !c.connected {
		return sim.ErrNotConnected
	}
	return AIRemoveObject(c.handle, objectID, reqID)
}

// SetObjectPosition updates the position of a sim object.
func (c *Client) SetObjectPosition(objectID uint32, lat, lon, alt, pitch, bank, hdg float64) error {
	if !c.connected {
		return sim.ErrNotConnected
	}

	// Basic positioning struct used by SetDataOnSimObject
	// Order matches the definition we will ensure exists: DefIDObjectPos
	data := struct {
		Lat, Lon, Alt, Pitch, Bank, Hdg, OnGround, Airspeed float64
	}{
		lat, lon, alt, pitch, bank, hdg, 0, 0,
	}

	// We assume DefIDObjectPos exists (added in setupDataDefinitions)
	return SetDataOnSimObject(c.handle, DefIDObjectPos, objectID, 0, 0,
		uint32(unsafe.Sizeof(data)), unsafe.Pointer(&data))
}

func (c *Client) connectionLoop() {
	ticker := time.NewTicker(c.reconnectInt)
	defer ticker.Stop()

	// Initial attempt
	c.connect()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			if !c.connected {
				c.connect()
			}
		}
	}
}

func (c *Client) connect() {
	c.logger.Debug("Attempting SimConnect connection...")

	handle, err := Open(c.appName)
	if err != nil {
		c.logger.Debug("Connection failed", "error", err)
		return
	}

	c.handle = handle
	c.connected = true

	c.telemetryMu.Lock()
	c.simState = sim.StateInactive
	c.hasValidData = false
	c.telemetryMu.Unlock()

	c.lastMessageTime = time.Now() // Initialize watchdog
	c.logger.Info("SimConnect Connected")

	// Setup data definitions
	if err := c.setupDataDefinitions(); err != nil {
		c.logger.Error("Failed to setup data definitions", "error", err)
		c.disconnect()
		return
	}

	// Start dispatch loop
	go c.dispatchLoop()

	// Subscribe to SimStop to detect quit reliably
	if err := SubscribeToSystemEvent(c.handle, EvtIDSimStop, "SimStop"); err != nil {
		c.logger.Error("Failed to subscribe to SimStop", "error", err)
	}
}

func (c *Client) disconnect() {
	if !c.connected {
		return
	}

	c.telemetryMu.Lock()
	c.simState = sim.StateDisconnected
	c.hasValidData = false
	c.telemetryMu.Unlock()

	if c.handle != 0 {
		_ = Close(c.handle)
		c.handle = 0
	}
	c.connected = false
	c.logger.Info("SimConnect Disconnected")
}

func (c *Client) setupDataDefinitions() error {
	// 1. Telemetry Data
	defs := []struct {
		name     string
		unit     string
		dataType uint32
	}{
		// Core telemetry
		{"PLANE LATITUDE", "Degrees", DATATYPE_FLOAT64},
		{"PLANE LONGITUDE", "Degrees", DATATYPE_FLOAT64},
		{"PLANE ALTITUDE", "Feet", DATATYPE_FLOAT64},
		{"PLANE ALT ABOVE GROUND", "Feet", DATATYPE_FLOAT64},
		{"PLANE HEADING DEGREES TRUE", "Degrees", DATATYPE_FLOAT64},
		{"GROUND VELOCITY", "Knots", DATATYPE_FLOAT64},
		{"SIM ON GROUND", "Bool", DATATYPE_INT32},
		{"GENERAL ENG COMBUSTION:1", "Bool", DATATYPE_INT32},
		{"CAMERA STATE", "Enum", DATATYPE_INT32},
		{"SIM DISABLED", "Bool", DATATYPE_INT32},
		{"TRANSPONDER CODE:1", "Number", DATATYPE_INT32},
		{"TRANSPONDER IDENT:1", "Bool", DATATYPE_INT32},
		// Autopilot Master/FD/YD
		{"AUTOPILOT MASTER", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT FLIGHT DIRECTOR ACTIVE", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT YAW DAMPER", "Bool", DATATYPE_FLOAT64},
		// Lateral Modes
		{"AUTOPILOT HEADING LOCK", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT NAV1 LOCK", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT APPROACH HOLD", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT BANK HOLD", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT BACKCOURSE HOLD", "Bool", DATATYPE_FLOAT64},
		{"GPS DRIVES NAV1", "Bool", DATATYPE_FLOAT64},
		// Vertical Modes
		{"AUTOPILOT ALTITUDE LOCK", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT VERTICAL HOLD", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT FLIGHT LEVEL CHANGE", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT GLIDESLOPE HOLD", "Bool", DATATYPE_FLOAT64},
		{"AUTOPILOT PITCH HOLD", "Bool", DATATYPE_FLOAT64},
		// Reference Values
		{"AUTOPILOT VERTICAL HOLD VAR", "Feet/minute", DATATYPE_FLOAT64},
		{"AUTOPILOT AIRSPEED HOLD VAR", "Knots", DATATYPE_FLOAT64},
		{"AUTOPILOT ALTITUDE LOCK VAR", "Feet", DATATYPE_FLOAT64},
		// For display: HDG bug and DTK
		{"AUTOPILOT HEADING LOCK DIR", "Degrees", DATATYPE_FLOAT64},
		{"GPS WP DESIRED TRACK", "Degrees", DATATYPE_FLOAT64},
	}

	for _, d := range defs {
		if err := AddToDataDefinition(c.handle, DefIDTelemetry, d.name, d.unit, d.dataType); err != nil {
			return err
		}
	}

	// 2. Object Positioning Data (Write-only usually)
	// Matches SetObjectPosition struct
	objDefs := []struct {
		name     string
		unit     string
		dataType uint32
	}{
		{"PLANE LATITUDE", "Degrees", DATATYPE_FLOAT64},
		{"PLANE LONGITUDE", "Degrees", DATATYPE_FLOAT64},
		{"PLANE ALTITUDE", "Feet", DATATYPE_FLOAT64},
		{"PLANE PITCH DEGREES", "degrees", DATATYPE_FLOAT64},
		{"PLANE BANK DEGREES", "degrees", DATATYPE_FLOAT64},
		{"PLANE HEADING DEGREES TRUE", "degrees", DATATYPE_FLOAT64},
		{"SIM ON GROUND", "bool", DATATYPE_FLOAT64},
		{"AIRSPEED TRUE", "knots", DATATYPE_FLOAT64},
	}
	for _, d := range objDefs {
		// Note: Using same float64 for all
		if err := AddToDataDefinition(c.handle, DefIDObjectPos, d.name, d.unit, d.dataType); err != nil {
			return err
		}
	}

	// Request data at 1Hz (PERIOD_SECOND)
	return RequestDataOnSimObject(c.handle, ReqIDTelemetry, DefIDTelemetry, OBJECT_ID_USER, PERIOD_SECOND, 0, 0, 0, 0)
}

func (c *Client) dispatchLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
			if !c.connected || c.handle == 0 {
				return
			}
			ppData, _, err := GetNextDispatch(c.handle)
			if err != nil {
				c.logger.Error("GetNextDispatch error", "error", err)
				c.disconnect()
				return
			}

			if ppData == nil {
				// Watchdog check
				if time.Since(c.lastMessageTime) > 5*time.Second {
					c.logger.Warn("Watchdog timeout (no data for 5s), resetting connection")
					c.disconnect()
					return
				}
				// No message, sleep briefly to prevent busy loop
				time.Sleep(10 * time.Millisecond)
				continue
			}

			c.lastMessageTime = time.Now() // Update watchdog
			c.handleMessage(ppData)
		}
	}
}

func (c *Client) handleMessage(ppData unsafe.Pointer) {
	recv := (*Recv)(ppData)
	// Verbose logging to debug missing QUIT message
	// c.logger.Debug("Received Message", "ID", recv.ID, "Size", recv.Size)

	switch recv.ID {
	case RECV_ID_OPEN:
		c.handleOpen(ppData)

	case RECV_ID_QUIT:
		c.handleQuit("Msg")

	case RECV_ID_EVENT:
		evt := (*RecvEvent)(ppData)
		if evt.UEventID == EvtIDSimStop {
			c.handleQuit("Event")
		}

	case RECV_ID_EXCEPTION:
		recvEx := (*RecvException)(ppData)
		c.logger.Warn("SimConnect Exception", "exception", recvEx.Exception, "sendID", recvEx.SendID)

	case RECV_ID_ASSIGNED_OBJECT_ID:
		c.handleAssignedObject(ppData)

	case RECV_ID_SIMOBJECT_DATA:
		c.handleSimObjectData(ppData)
	}
}

func (c *Client) handleOpen(ppData unsafe.Pointer) {
	recvOpen := (*RecvOpen)(ppData)
	// Convert null-terminated C string
	appName := cStringToGo(recvOpen.ApplicationName[:])
	c.logger.Info("SimConnect Session Opened", "app", appName)
}

func (c *Client) handleQuit(source string) {
	c.logger.Info("Simulator Quit detected", "source", source)
	c.disconnect()
}

func (c *Client) handleAssignedObject(ppData unsafe.Pointer) {
	assigned := (*RecvAssignedObjectID)(ppData)
	c.spawnMu.Lock()
	if ch, ok := c.pendingSpawns[assigned.RequestID]; ok {
		// Non-blocking send (buffered channel)
		select {
		case ch <- assigned.ObjectID:
		default:
		}
	}
	c.spawnMu.Unlock()
}

func (c *Client) handleSimObjectData(ppData unsafe.Pointer) {
	recvData := (*RecvSimobjectData)(ppData)
	if recvData.RequestID == ReqIDTelemetry {
		// Data follows immediately after the header
		dataPtr := unsafe.Pointer(uintptr(ppData) + unsafe.Sizeof(RecvSimobjectData{}))
		data := (*TelemetryData)(dataPtr)

		c.telemetryMu.Lock()
		defer c.telemetryMu.Unlock()

		// Validate telemetry before processing
		if !c.validateTelemetry(data) {
			return
		}

		// Log camera state changes at DEBUG
		if c.cameraState != data.Camera {
			c.logger.Debug("Camera state changed", "from", c.cameraState, "to", data.Camera)
		}
		c.cameraState = data.Camera

		// Update simState based on camera value
		if newState := sim.UpdateState(data.Camera); newState != nil {
			if c.simState != *newState {
				c.logger.Info("SimState changed", "from", c.simState, "to", *newState, "camera", data.Camera)
				c.simState = *newState
			}
		}

		// Only update telemetry when active
		if c.simState == sim.StateActive {
			// Calculate predicted position
			// Speed in Knots -> Meters/Second
			// 1 Knot = 0.514444 m/s
			// Distance = Speed * WindowDuration
			distMeters := data.GroundSpeed * 0.514444 * c.predictionWindow.Seconds()

			var predLat, predLon float64
			if distMeters > 0 {
				pred := geo.DestinationPoint(
					geo.Point{Lat: data.Latitude, Lon: data.Longitude},
					distMeters,
					data.Heading,
				)
				predLat, predLon = pred.Lat, pred.Lon
			} else {
				// Stationary: predicted position = current position
				predLat, predLon = data.Latitude, data.Longitude
			}

			// Calculate TrackTrue (Ground Track)
			currentPos := geo.Point{Lat: data.Latitude, Lon: data.Longitude}
			trackTrue := data.Heading // Default

			isOnGround := data.OnGround != 0 || data.AltitudeAGL < 50
			if isOnGround {
				c.trackBuf.Reset()
			} else {
				trackTrue = c.trackBuf.Push(currentPos, data.Heading)
			}

			c.telemetry = sim.Telemetry{
				Latitude:           data.Latitude,
				Longitude:          data.Longitude,
				AltitudeMSL:        data.AltitudeMSL,
				AltitudeAGL:        data.AltitudeAGL,
				Heading:            trackTrue,
				GroundSpeed:        data.GroundSpeed,
				PredictedLatitude:  predLat,
				PredictedLongitude: predLon,
				IsOnGround:         isOnGround,
				APStatus:           formatAPStatus(data),
				Squawk:             int(data.Squawk),
				Ident:              data.Ident != 0,
			}
			c.telemetry.FlightStage = sim.DetermineFlightStage(&c.telemetry)
			c.hasValidData = true
		}
	}
}

// validateTelemetry checks for spurious data patterns common in SimConnect.
// Returns true if telemetry is valid, false if it should be discarded.
func (c *Client) validateTelemetry(data *TelemetryData) bool {
	// 1. Null Island check: Lat/Lon both effectively zero
	if math.Abs(data.Latitude) < 0.1 && math.Abs(data.Longitude) < 0.1 {
		return false
	}

	// 2. Spurious Equatorial check: Lat ~0 AND |Lon| ~90
	// This specific pattern is known to occur as a glitch.
	if math.Abs(data.Latitude) < 0.1 && math.Abs(math.Abs(data.Longitude)-90.0) < 0.1 {
		return false
	}

	// 3. Ground/Altitude Contradiction
	// If sim says we are on ground, but AGL is > 1000ft, something is wrong.
	isOnGround := data.OnGround != 0
	if isOnGround && data.AltitudeAGL > 1000 {
		return false
	}

	return true
}

// formatAPStatus returns a G1000-style autopilot status string.
func formatAPStatus(d *TelemetryData) string {
	center := formatCenterStatus(d)
	if center == "" {
		return ""
	}

	lat := formatLateralMode(d)
	vert := formatVerticalMode(d)

	return fmt.Sprintf("%-9s  %-5s  %s", lat, center, vert)
}

func formatCenterStatus(d *TelemetryData) string {
	if d.APMaster > 0.5 {
		if d.YDActive > 0.5 {
			return "AP YD"
		}
		return "AP"
	}
	if d.YDActive > 0.5 {
		return "YD"
	}
	if d.FDActive > 0.5 {
		return "FD"
	}
	return ""
}

func formatLateralMode(d *TelemetryData) string {
	switch {
	case d.APRHold > 0.5 && d.GSHold > 0.5:
		return fmt.Sprintf("LOC→%03.0f", d.DTK)
	case d.BCHold > 0.5:
		return "BC"
	case d.HDGLock > 0.5:
		return fmt.Sprintf("HDG %03.0f", d.HDGBug)
	case d.NAV1Lock > 0.5:
		if d.GPSDrivesNAV1 > 0.5 {
			return fmt.Sprintf("GPS→%03.0f", d.DTK)
		}
		return fmt.Sprintf("VOR→%03.0f", d.DTK)
	case d.BankHold > 0.5:
		return "ROL"
	default:
		return "ROL"
	}
}

func formatVerticalMode(d *TelemetryData) string {
	switch {
	case d.GSHold > 0.5:
		return "GS"
	case d.FLCHold > 0.5:
		return fmt.Sprintf("FLC %dkt", int(math.Round(d.IASVar)))
	case d.VSHold > 0.5:
		vs := int(math.Round(d.VSVar))
		if vs >= 0 {
			return fmt.Sprintf("VS +%dfpm", vs)
		}
		return fmt.Sprintf("VS %dfpm", vs)
	case d.ALTLock > 0.5:
		return fmt.Sprintf("ALT %dft", int(math.Round(d.ALTVar)))
	case d.PitchHold > 0.5:
		return "PIT"
	default:
		return "PIT"
	}
}
