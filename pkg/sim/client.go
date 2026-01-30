package sim

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotConnected is returned when a client action requires a connection.
	ErrNotConnected = errors.New("simulator not connected")
	// ErrWaitingForTelemetry is returned when connected but no valid data has been received yet.
	ErrWaitingForTelemetry = errors.New("waiting for initial telemetry")
)

// Client defines the interface for simulator interaction.
type Client interface {
	// GetTelemetry returns the current state of the aircraft.
	GetTelemetry(ctx context.Context) (Telemetry, error)
	// GetState returns the current simulator connection/activity state.
	GetState() State
	// SetPredictionWindow sets the time duration for future position prediction.
	SetPredictionWindow(d time.Duration)
	// Close cleans up resources associated with the client.
	Close() error
}

// ObjectClient defines the interface for creation and manipulating sim objects.
type ObjectClient interface {
	// SpawnAirTraffic spawns a non-ATC aircraft (AI object).
	SpawnAirTraffic(reqID uint32, title, tailNum string, lat, lon, alt, hdg float64) (uint32, error)
	// RemoveObject removes a sim object by its ID.
	RemoveObject(objectID, reqID uint32) error
	// SetObjectPosition updates the position of a sim object.
	SetObjectPosition(objectID uint32, lat, lon, alt, pitch, bank, hdg float64) error
}

// Telemetry represents a snapshot of aircraft state.
type Telemetry struct {
	Latitude      float64 // Degrees
	Longitude     float64 // Degrees
	AltitudeMSL   float64 // Feet MSL
	AltitudeAGL   float64 // Feet AGL
	Heading       float64 // Degrees True (Ground Track when airborne)
	GroundSpeed   float64 // Knots
	VerticalSpeed float64 // Feet per minute
	// Predicted position (1 min ahead)
	// Predicted position (1 min ahead)
	PredictedLatitude  float64
	PredictedLongitude float64

	IsOnGround  bool   // True if parked or taxiing
	EngineOn    bool   // True if any engine is running
	FlightStage string // Detailed stage (parked, taxi, climb, etc)
	APStatus    string // G1000-style autopilot status (e.g. "HDG 270  AP  ALT 5000ft")

	// Transponder
	Squawk int  // TRANSPONDER CODE
	Ident  bool // TRANSPONDER IDENT
}

// DetermineFlightStage calculates a basic flight phase.
// Deprecated: Use StageMachine for stateful flight stage tracking.
func DetermineFlightStage(t *Telemetry) string {
	if t.IsOnGround {
		return "GROUND"
	}
	// Simple heuristics for airborne phases
	if t.AltitudeAGL < 2000 {
		if t.VerticalSpeed > 300 {
			return "TAKEOFF/CLIMB"
		}
		if t.VerticalSpeed < -300 {
			return "APPROACH/LANDING"
		}
	}
	return "CRUISE"
}
