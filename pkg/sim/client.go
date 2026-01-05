package sim

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotConnected is returned when a client action requires a connection.
	ErrNotConnected = errors.New("simulator not connected")
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
	Heading       float64 // Degrees True
	GroundSpeed   float64 // Knots
	VerticalSpeed float64 // Feet per minute
	// Predicted position (1 min ahead)
	PredictedLatitude  float64
	PredictedLongitude float64
	// Visibility / Weather
	AmbientInCloud    float64 // 1.0 if inside cloud, 0.0 otherwise
	AmbientVisibility float64 // Meters

	IsOnGround  bool   // True if parked or taxiing
	FlightStage string // Ground, Takeoff, Climb, Cruise, Approach, Landing
}

// DetermineFlightStage calculates the flight phase based on telemetry.
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
