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
// Order must match the AddToDataDefinition calls.
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
	_           int32 // Padding for 8-byte alignment
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
