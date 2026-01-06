// Package main provides a PoC test app for SimConnect marker spawning.
package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"phileasgo/pkg/sim/simconnect"
)

const (
	dllPath = "./lib/SimConnect.dll"

	// Define IDs
	defIDUserPos    uint32 = 0
	defIDSetBalloon uint32 = 1 // Logic for setting marker data

	// Request IDs
	reqIDUserPos     uint32 = 0
	reqIDSpawn1      uint32 = 1
	reqIDSpawn2      uint32 = 2
	reqIDSpawn3      uint32 = 3
	reqIDSpawnTarget uint32 = 4
	reqIDRemove      uint32 = 100
)

var (
	// Container titles for AI objects (must exist in sim content)
	titlesToTry = []string{
		"Asobo PassiveAircraft Hot Air Balloon",
		"Generic Hot Air Balloon",
		"Airbus A320 Neo Asobo",
		"Cessna Skyhawk Asobo",
	}
)

// UserPosition holds the user aircraft position
type UserPosition struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Heading   float64
}

// MarkerData defines structure for setting marker position
type MarkerData struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Pitch     float64
	Bank      float64
	Heading   float64
	OnGround  float64
	Airspeed  float64
}

type SpawnedBalloon struct {
	ID           uint32
	BaseAltitude float64 // Fixed altitude at spawn time
	Lat          float64 // Fixed Latitude (for Target)
	Lon          float64 // Fixed Longitude (for Target)
	IsTarget     bool
}

var (
	handle          uintptr
	spawnedBalloons []SpawnedBalloon
	userPos         UserPosition
	currentTitleIdx int
	spawnRequested  bool
	formationActive bool = true // Start active
)

func main() {
	fmt.Println("SimConnect Marker PoC Test - Target Bearing Indicator")
	fmt.Println("====================================================")

	// Load DLL
	fmt.Printf("Loading DLL from: %s\n", dllPath)
	if err := simconnect.LoadDLL(dllPath); err != nil {
		fmt.Printf("ERROR: Failed to load DLL: %v\n", err)
		os.Exit(1)
	}
	// Open connection
	var err error
	handle, err = simconnect.Open("SimTest PoC")
	if err != nil {
		fmt.Printf("ERROR: Failed to connect: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Connected! Handle: 0x%x\n", handle)

	// Setup signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start dispatch loop in goroutine
	done := make(chan struct{})
	go dispatchLoop(done)

	// Wait for signal
	<-sigChan
	fmt.Println("\nShutting down...")
	close(done)

	// Remove spawned objects
	removeAllObjects()

	// Close connection
	_ = simconnect.Close(handle)
	fmt.Println("Disconnected. Goodbye!")
}

func removeAllObjects() {
	if len(spawnedBalloons) > 0 {
		fmt.Printf("Removing %d balloons...\n", len(spawnedBalloons))
		for _, b := range spawnedBalloons {
			_ = simconnect.AIRemoveObject(handle, b.ID, reqIDRemove)
		}
		spawnedBalloons = []SpawnedBalloon{} // Clear list
		fmt.Println("All objects removed")
	}
}

func dispatchLoop(done chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
		}

		ppData, cbData, err := simconnect.GetNextDispatch(handle)
		if err != nil || cbData == 0 {
			time.Sleep(2 * time.Millisecond)
			continue
		}

		recv := (*simconnect.Recv)(ppData)
		switch recv.ID {
		case simconnect.RECV_ID_OPEN:
			fmt.Println("Session opened, setting up data definition...")
			setupDataDefinition()

		case simconnect.RECV_ID_SIMOBJECT_DATA:
			handleSimobjectData(ppData)

		case simconnect.RECV_ID_ASSIGNED_OBJECT_ID:
			handleAssignedObject(ppData)

		case simconnect.RECV_ID_QUIT:
			return
		}
	}
}

func handleAssignedObject(ppData unsafe.Pointer) {
	assigned := (*simconnect.RecvAssignedObjectID)(ppData)

	// Handle Spawned Objects
	if assigned.RequestID >= 1 && assigned.RequestID <= 4 {
		isTarget := (assigned.RequestID == reqIDSpawnTarget)

		// Calculate correct altitude for tracking
		var altOffset float64
		switch assigned.RequestID {
		case reqIDSpawn1:
			altOffset = 200
		case reqIDSpawn2:
			altOffset = 0
		case reqIDSpawn3:
			altOffset = -200
		case reqIDSpawnTarget:
			altOffset = 0 // Target at user alt
		}
		absAlt := userPos.Altitude + altOffset

		b := SpawnedBalloon{
			ID:           assigned.ObjectID,
			BaseAltitude: absAlt,
			IsTarget:     isTarget,
		}

		if isTarget {
			b.Lat = targetLatGlobal
			b.Lon = targetLonGlobal
			b.BaseAltitude = targetAltGlobal
			fmt.Printf("SUCCESS: Target Balloon spawned! ID: %d at %0.4f, %0.4f\n", assigned.ObjectID, b.Lat, b.Lon)
		} else {
			fmt.Printf("SUCCESS: Formation Balloon #%d spawned! ID: %d\n", assigned.RequestID, assigned.ObjectID)
		}

		spawnedBalloons = append(spawnedBalloons, b)
	}
}

func setupDataDefinition() {
	// 1. User Pos (Read)
	_ = simconnect.AddToDataDefinition(handle, defIDUserPos, "PLANE LATITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDUserPos, "PLANE LONGITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDUserPos, "PLANE ALTITUDE", "feet", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDUserPos, "PLANE HEADING DEGREES TRUE", "degrees", simconnect.DATATYPE_FLOAT64)

	// 2. Balloon Pos (Write) - using same def for all
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "PLANE LATITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "PLANE LONGITUDE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "PLANE ALTITUDE", "feet", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "PLANE PITCH DEGREES", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "PLANE BANK DEGREES", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "PLANE HEADING DEGREES TRUE", "degrees", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "SIM ON GROUND", "bool", simconnect.DATATYPE_FLOAT64)
	_ = simconnect.AddToDataDefinition(handle, defIDSetBalloon, "AIRSPEED TRUE", "knots", simconnect.DATATYPE_FLOAT64)

	fmt.Println("Requesting high-frequency user position (VisualFrame, Interval=3)...")
	_ = simconnect.RequestDataOnSimObject(handle, reqIDUserPos, defIDUserPos, simconnect.OBJECT_ID_USER,
		simconnect.PERIOD_VISUAL_FRAME, 0, 0, 3, 0)
}

func handleSimobjectData(ppData unsafe.Pointer) {
	recvData := (*simconnect.RecvSimobjectData)(ppData)
	if recvData.RequestID != reqIDUserPos {
		return
	}

	dataPtr := unsafe.Pointer(uintptr(ppData) + unsafe.Sizeof(simconnect.RecvSimobjectData{}))
	userPos = *(*UserPosition)(dataPtr)

	// Initial spawn
	if !spawnRequested && currentTitleIdx < len(titlesToTry) {
		spawnRequested = true
		spawnFormationAndTarget()
	}

	if len(spawnedBalloons) > 0 {
		updateLogic()
	}
}

// Global target coords to share with dispatch loop
var targetLatGlobal, targetLonGlobal, targetAltGlobal float64

func spawnFormationAndTarget() {
	if currentTitleIdx >= len(titlesToTry) {
		return
	}
	title := titlesToTry[currentTitleIdx]

	// 1. Calculate Target Position (10km ahead)
	targetDistKm := 10.0
	hdgRad := userPos.Heading * (math.Pi / 180.0)
	latRad := userPos.Latitude * (math.Pi / 180.0)

	dLat10 := (targetDistKm / 111.0) * math.Cos(hdgRad)
	dLon10 := (targetDistKm / (111.0 * math.Cos(latRad))) * math.Sin(hdgRad)

	targetLatGlobal = userPos.Latitude + dLat10
	targetLonGlobal = userPos.Longitude + dLon10
	targetAltGlobal = userPos.Altitude

	fmt.Printf("User: %.4f, %.4f | Heading: %.0f\n", userPos.Latitude, userPos.Longitude, userPos.Heading)
	fmt.Printf("Target (10km): %.4f, %.4f\n", targetLatGlobal, targetLonGlobal)

	// List to spawn
	type SpawnReq struct {
		id   uint32
		lat  float64
		lon  float64
		alt  float64
		desc string
	}

	// Initial formation spawn just 2km ahead of user heading initially to verify spawn
	dLat2 := (2.0 / 111.0) * math.Cos(hdgRad)
	dLon2 := (2.0 / (111.0 * math.Cos(latRad))) * math.Sin(hdgRad)

	reqs := []SpawnReq{
		// Formation
		{reqIDSpawn1, userPos.Latitude + dLat2, userPos.Longitude + dLon2, userPos.Altitude + 200, "Form Top"},
		{reqIDSpawn2, userPos.Latitude + dLat2, userPos.Longitude + dLon2, userPos.Altitude, "Form Center"},
		{reqIDSpawn3, userPos.Latitude + dLat2, userPos.Longitude + dLon2, userPos.Altitude - 200, "Form Bottom"},
		// Target
		{reqIDSpawnTarget, targetLatGlobal, targetLonGlobal, targetAltGlobal, "TARGET"},
	}

	for _, r := range reqs {
		initPos := simconnect.InitPosition{
			Latitude: r.lat, Longitude: r.lon, AltitudeMSL: r.alt,
			Heading: userPos.Heading, OnGround: 0, Airspeed: 0,
		}
		// Failure to spawn just prints error in PoC
		if err := simconnect.AICreateNonATCAircraft(handle, title, fmt.Sprintf("M%d", r.id), &initPos, r.id); err != nil {
			fmt.Printf("Failed to spawn %s: %v\n", r.desc, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func updateLogic() {
	// 1. Calculate Vector to Target
	dLat := targetLatGlobal - userPos.Latitude
	dLon := targetLonGlobal - userPos.Longitude
	latRad := userPos.Latitude * (math.Pi / 180.0)

	// Convert to meters/km for distance calculation
	// lat deg ~ 111km, lon deg ~ 111km * cos(lat)
	dy := dLat * 111.0                    // North component
	dx := dLon * 111.0 * math.Cos(latRad) // East component
	distKm := math.Sqrt(dx*dx + dy*dy)

	// Calculate Bearing (from North clockwise)
	// atan2(x, y) returns angle from Y axis (North) in range -pi to pi.
	// We want dx (East) and dy (North).
	// Math: atan2(dx, dy) gives angle from North.
	bearingRad := math.Atan2(dx, dy)

	// Convert to degrees for Heading (0-360)
	bearingDeg := bearingRad * (180.0 / math.Pi)
	if bearingDeg < 0 {
		bearingDeg += 360.0
	}

	// 2. Check Trigger
	if formationActive && distKm < 3.0 {
		fmt.Printf("Target Distance %.2f km < 3.0 km! Despawning formation.\n", distKm)
		var kept []SpawnedBalloon
		for _, b := range spawnedBalloons {
			if !b.IsTarget {
				_ = simconnect.AIRemoveObject(handle, b.ID, reqIDRemove)
			} else {
				kept = append(kept, b)
			}
		}
		spawnedBalloons = kept
		formationActive = false
	}

	// 3. Update Positions

	// Formation Target Pos: 2km along BEARING line from User
	formDistKm := 2.0

	// Offsets for 2km along bearing
	// dLat = (dist / 111) * cos(bearing)  <- Since bearing is from North (Y axis), cos gives projection on Y (North)
	// dLon = (dist / 111*cosLat) * sin(bearing) <- sin gives projection on X (East)
	dLatForm := (formDistKm / 111.0) * math.Cos(bearingRad)

	cosLat := math.Cos(latRad)
	if math.Abs(cosLat) < 0.0001 {
		cosLat = 0.0001
	}
	dLonForm := (formDistKm / (111.0 * cosLat)) * math.Sin(bearingRad)

	formLat := userPos.Latitude + dLatForm
	formLon := userPos.Longitude + dLonForm

	for _, b := range spawnedBalloons {
		var setLat, setLon, setAlt, setHdg float64

		if b.IsTarget {
			// Anti-drift: Keep at global target coords
			setLat = targetLatGlobal
			setLon = targetLonGlobal
			setAlt = b.BaseAltitude
			setHdg = 0 // Orientation doesn't matter much for target
		} else {
			// Formation: 2km along bearing line
			setLat = formLat
			setLon = formLon
			setAlt = b.BaseAltitude
			// Point formation balloons TOWARDS the target
			setHdg = bearingDeg
		}

		payload := struct {
			Lat, Lon, Alt, Pitch, Bank, Hdg, OnGround, Airspeed float64
		}{
			setLat, setLon, setAlt, 0, 0, setHdg, 0, 0,
		}

		_ = simconnect.SetDataOnSimObject(handle, defIDSetBalloon, b.ID, 0, 0,
			uint32(unsafe.Sizeof(payload)), unsafe.Pointer(&payload))
	}
}
