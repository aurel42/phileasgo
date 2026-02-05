package beacon

import (
	"context"
	"phileasgo/pkg/sim/simconnect"
)

func (s *Service) calculateTargetAltitude(ctx context.Context, tel *simconnect.TelemetryData, pLat, pLon, pBaseAlt, distMeters float64) float64 {
	// Pre-calc limit constants for sinking logic
	sinkDistFar := float64(s.prov.BeaconSinkDistanceFar(ctx))
	sinkDistNear := float64(s.prov.BeaconSinkDistanceClose(ctx))
	targetFloorFt := float64(s.prov.BeaconTargetFloorAGL(ctx)) * 3.28084
	minSpawnAltFt := float64(s.prov.BeaconMinSpawnAltitude(ctx)) * 3.28084

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
		baseAlt := pBaseAlt

		// Floor Altitude Calculation
		var floorAlt float64
		if s.elev != nil {
			// Query ETOPO1 for terrain height at POI
			elevMeters, err := s.elev.GetElevation(pLat, pLon)
			if err == nil {
				// Convert meters to feet + offset
				floorAlt = (float64(elevMeters) * 3.28084) + targetFloorFt
			} else {
				// Fallback to plane-relative heuristic on error
				floorAlt = (tel.AltitudeMSL - tel.AltitudeAGL) + targetFloorFt
			}
		} else {
			// Standard plane-relative heuristic (Assumes plane and balloon share same ground)
			floorAlt = (tel.AltitudeMSL - tel.AltitudeAGL) + targetFloorFt
		}

		return (1-t)*baseAlt + t*floorAlt
	}

	// Far out or low altitude -> Standard behavior
	return s.targetAlt
}
