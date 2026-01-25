package simconnect

// Recv is the base struct for all received messages.
type Recv struct {
	Size    uint32
	Version uint32
	ID      uint32
}

// RecvOpen is received when connection is established.
type RecvOpen struct {
	Recv
	ApplicationName         [256]byte
	ApplicationVersionMajor uint32
	ApplicationVersionMinor uint32
	ApplicationBuildMajor   uint32
	ApplicationBuildMinor   uint32
	SimConnectVersionMajor  uint32
	SimConnectVersionMinor  uint32
	SimConnectBuildMajor    uint32
	SimConnectBuildMinor    uint32
	Reserved1               uint32
	Reserved2               uint32
}

// RecvException is received when an error occurs.
type RecvException struct {
	Recv
	Exception uint32
	SendID    uint32
	Index     uint32
}

// RecvEvent is received when a subscribed system event occurs.
type RecvEvent struct {
	Recv
	UEventID uint32
	Data     uint32
}

// RecvSimobjectData is received with requested sim object data.
type RecvSimobjectData struct {
	Recv
	RequestID   uint32
	ObjectID    uint32
	DefineID    uint32
	Flags       uint32
	EntryNumber uint32
	OutOf       uint32
	DefineCount uint32
	// Data bytes follow immediately after this struct
}

// RecvAssignedObjectID is received after spawning an AI object.
type RecvAssignedObjectID struct {
	Recv
	RequestID uint32
	ObjectID  uint32
}

// InitPosition is used for spawning and positioning objects.
// Must match SIMCONNECT_DATA_INITPOSITION exactly.
type InitPosition struct {
	Latitude    float64
	Longitude   float64
	AltitudeMSL float64
	Pitch       float64
	Bank        float64
	Heading     float64
	OnGround    uint32
	Airspeed    uint32
}

// TelemetryData is the struct for reading user aircraft telemetry.
// Order must match the AddToDataDefinition calls EXACTLY.
// WARNING: SimConnect data is packed (no padding). This struct must be
// carefully aligned to avoid Go's implicit padding breaking the layout.
// Current layout: 6x float64 (48 bytes) + 3x int32 (12 bytes) = 60 bytes
// Go will pad to 64 bytes before next float64, but SimConnect sends 60 bytes
// followed immediately by AP float64s. We work around this by reading into
// a byte buffer and manually parsing, OR by reordering fields.
// WORKAROUND: Add a 4th int32 simvar to maintain alignment.
type TelemetryData struct {
	Latitude    float64
	Longitude   float64
	AltitudeMSL float64
	AltitudeAGL float64
	Heading     float64
	GroundSpeed float64
	OnGround    int32
	Engine      int32
	Camera      int32
	SimDisabled int32 // Added to maintain 8-byte alignment (SIM DISABLED)
	Squawk      int32 // TRANSPONDER CODE:1
	Ident       int32 // TRANSPONDER IDENT:1

	// Autopilot State (all float64 for SimConnect Bool compatibility)
	APMaster      float64 // AUTOPILOT MASTER
	FDActive      float64 // AUTOPILOT FLIGHT DIRECTOR ACTIVE
	YDActive      float64 // AUTOPILOT YAW DAMPER
	HDGLock       float64 // AUTOPILOT HEADING LOCK
	NAV1Lock      float64 // AUTOPILOT NAV1 LOCK
	APRHold       float64 // AUTOPILOT APPROACH HOLD
	BankHold      float64 // AUTOPILOT BANK HOLD
	BCHold        float64 // AUTOPILOT BACKCOURSE HOLD
	GPSDrivesNAV1 float64 // GPS DRIVES NAV1
	ALTLock       float64 // AUTOPILOT ALTITUDE LOCK
	VSHold        float64 // AUTOPILOT VERTICAL HOLD
	FLCHold       float64 // AUTOPILOT FLIGHT LEVEL CHANGE
	GSHold        float64 // AUTOPILOT GLIDESLOPE HOLD
	PitchHold     float64 // AUTOPILOT PITCH HOLD
	VSVar         float64 // AUTOPILOT VERTICAL HOLD VAR (ft/min)
	IASVar        float64 // AUTOPILOT AIRSPEED HOLD VAR (kts)
	ALTVar        float64 // AUTOPILOT ALTITUDE LOCK VAR (ft)
	HDGBug        float64 // AUTOPILOT HEADING LOCK DIR (degrees)
	DTK           float64 // GPS WP DESIRED TRACK (degrees)
}

// MarkerUpdateData is the struct for updating marker positions.
// Order must match the AddToDataDefinition calls for markers.
type MarkerUpdateData struct {
	Latitude    float64
	Longitude   float64
	AltitudeMSL float64
	Pitch       float64
	Bank        float64
	Heading     float64
	OnGround    int32
	Airspeed    int32
}
