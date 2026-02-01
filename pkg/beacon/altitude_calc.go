package beacon

import (
	"phileasgo/pkg/sim/simconnect"
)

func (s *Service) calculateTargetAltitude(tel *simconnect.TelemetryData, distMeters float64) float64 {
	// Pre-calc limit constants for sinking logic
	sinkDistFar := float64(s.config.TargetSinkDistFar)   // e.g. 4000
	sinkDistNear := float64(s.config.TargetSinkDistNear) // e.g. 1000
	targetFloorFt := float64(s.config.TargetFloorAGL) * 3.28084
	minSpawnAltFt := float64(s.config.MinSpawnAltitude) * 3.28084

	// Only sink if we are above the safety spawn altitude (safety first)
	// AND we are within range
	shouldSink := tel.AltitudeAGL > minSpawnAltFt && distMeters < sinkDistFar

	if shouldSink {
		// Interpolation factor t (0=Far, 1=Near)
		var t float64
		if distMeters <= sinkDistNear {
			t = 1.0
		} else {
			// Linear t
			linearT := (sinkDistFar - distMeters) / (sinkDistFar - sinkDistNear)
			// Quadratic t (sinks faster closer to target)
			t = linearT * linearT
		}

		// Target Altitude Calculation
		baseAlt := s.targetAlt
		// Approximate Ground + TargetFloor
		// We use current AGL to estimate ground MSL under the balloon (UserAGL approx BalloonAGL assumption)
		floorAlt := (tel.AltitudeMSL - tel.AltitudeAGL) + targetFloorFt

		return (1-t)*baseAlt + t*floorAlt
	}

	// Far out or low altitude -> Standard behavior
	return s.targetAlt
}
