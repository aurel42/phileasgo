// Package simconnect provides direct bindings to SimConnect.dll for MSFS integration.
package simconnect

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// DLL and procedure handles
var (
	dll                                *syscall.LazyDLL
	procOpen                           *syscall.LazyProc
	procClose                          *syscall.LazyProc
	procAddToDataDefinition            *syscall.LazyProc
	procRequestDataOnSimObject         *syscall.LazyProc
	procGetNextDispatch                *syscall.LazyProc
	procAICreateNonATCAircraft         *syscall.LazyProc
	procSetDataOnSimObject             *syscall.LazyProc
	procAIRemoveObject                 *syscall.LazyProc
	procSubscribeToSystemEvent         *syscall.LazyProc
	procEnumerateSimObjectsAndLiveries *syscall.LazyProc
	procAICreateNonATCAircraftEX1      *syscall.LazyProc
)

// Error codes
const (
	SOK   = 0
	EFAIL = 0x80004005
)

// Data types
const (
	DATATYPE_INVALID   uint32 = 0
	DATATYPE_INT32     uint32 = 1
	DATATYPE_INT64     uint32 = 2
	DATATYPE_FLOAT32   uint32 = 3
	DATATYPE_FLOAT64   uint32 = 4
	DATATYPE_STRING8   uint32 = 5
	DATATYPE_STRING32  uint32 = 6
	DATATYPE_STRING64  uint32 = 7
	DATATYPE_STRING128 uint32 = 8
	DATATYPE_STRING256 uint32 = 9
)

// Periods
const (
	PERIOD_NEVER        uint32 = 0
	PERIOD_ONCE         uint32 = 1
	PERIOD_VISUAL_FRAME uint32 = 2
	PERIOD_SIM_FRAME    uint32 = 3
	PERIOD_SECOND       uint32 = 4
)

// Object types
const (
	SIMOBJECT_TYPE_USER            uint32 = 0
	SIMOBJECT_TYPE_ALL             uint32 = 1
	SIMOBJECT_TYPE_AIRCRAFT        uint32 = 2
	SIMOBJECT_TYPE_HELICOPTER      uint32 = 3
	SIMOBJECT_TYPE_BOAT            uint32 = 4
	SIMOBJECT_TYPE_GROUND          uint32 = 5
	SIMOBJECT_TYPE_HOT_AIR_BALLOON uint32 = 6
)

// Recv IDs
const (
	RECV_ID_NULL                              uint32 = 0
	RECV_ID_EXCEPTION                         uint32 = 1
	RECV_ID_OPEN                              uint32 = 2
	RECV_ID_QUIT                              uint32 = 3
	RECV_ID_EVENT                             uint32 = 4
	RECV_ID_SIMOBJECT_DATA                    uint32 = 8
	RECV_ID_SIMOBJECT_DATA_BYTYPE             uint32 = 9
	RECV_ID_ASSIGNED_OBJECT_ID                uint32 = 12
	RECV_ID_ENUMERATE_SIMOBJECTS_AND_LIVERIES uint32 = 38
)

// Special Object IDs
const (
	OBJECT_ID_USER uint32 = 0
)

// FindDLL returns the path to SimConnect.dll.
// It first tries to extract the embedded DLL (bundled at build time).
// Falls back to SDK paths for development environments.
func FindDLL() (string, error) {
	// Try embedded DLL first (bundled at build time)
	if path, err := extractEmbeddedDLL(); err == nil {
		return path, nil
	}

	// Fallback: Check SDK paths for development
	var paths []string

	// Check MSFS_SDK environment variable
	if sdkPath := os.Getenv("MSFS_SDK"); sdkPath != "" {
		paths = append(paths, filepath.Join(sdkPath, "SimConnect SDK", "lib", "SimConnect.dll"))
	}

	// Check common SDK installation paths (MSFS 2020 and 2024)
	paths = append(paths,
		`C:\MSFS 2024 SDK\SimConnect SDK\lib\SimConnect.dll`,
		`C:\MSFS SDK\SimConnect SDK\lib\SimConnect.dll`,
		`C:\Program Files (x86)\Microsoft Flight Simulator SDK\SimConnect SDK\lib\SimConnect.dll`,
	)

	// Check each path
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("SimConnect.dll not found; embedded DLL missing and no SDK installed")
}

// LoadDLL loads the SimConnect.dll from the specified path.
func LoadDLL(path string) error {
	dll = syscall.NewLazyDLL(path)
	if err := dll.Load(); err != nil {
		return fmt.Errorf("failed to load SimConnect.dll: %w", err)
	}

	procOpen = dll.NewProc("SimConnect_Open")
	procClose = dll.NewProc("SimConnect_Close")
	procAddToDataDefinition = dll.NewProc("SimConnect_AddToDataDefinition")
	procRequestDataOnSimObject = dll.NewProc("SimConnect_RequestDataOnSimObject")
	procGetNextDispatch = dll.NewProc("SimConnect_GetNextDispatch")
	procAICreateNonATCAircraft = dll.NewProc("SimConnect_AICreateNonATCAircraft")
	procSetDataOnSimObject = dll.NewProc("SimConnect_SetDataOnSimObject")
	procAIRemoveObject = dll.NewProc("SimConnect_AIRemoveObject")
	procSubscribeToSystemEvent = dll.NewProc("SimConnect_SubscribeToSystemEvent")
	procEnumerateSimObjectsAndLiveries = dll.NewProc("SimConnect_EnumerateSimObjectsAndLiveries")
	procAICreateNonATCAircraftEX1 = dll.NewProc("SimConnect_AICreateNonATCAircraft_EX1")
	return nil
}

// IsLoaded returns true if the SimConnect DLL and procedures are loaded.
func IsLoaded() bool {
	return dll != nil && procOpen != nil
}

// Open establishes a connection to SimConnect.
// Returns the handle on success.
func Open(name string) (uintptr, error) {
	if !IsLoaded() {
		return 0, fmt.Errorf("SimConnect DLL not loaded")
	}
	var handle uintptr
	namePtr, _ := syscall.UTF16PtrFromString(name)

	r1, _, err := procOpen.Call(
		uintptr(unsafe.Pointer(&handle)),
		uintptr(unsafe.Pointer(namePtr)),
		0, // hWnd
		0, // UserEventWin32
		0, // EventHandle
		0, // ConfigIndex
	)

	if int32(r1) < 0 {
		return 0, fmt.Errorf("SimConnect_Open failed: %v (0x%x)", err, r1)
	}

	return handle, nil
}

// Close terminates the SimConnect connection.
func Close(handle uintptr) error {
	if !IsLoaded() {
		return nil // already closed or not loaded
	}
	r1, _, err := procClose.Call(handle)
	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_Close failed: %v (0x%x)", err, r1)
	}
	return nil
}

// AddToDataDefinition adds a SimVar to a data definition.
func AddToDataDefinition(handle uintptr, defineID uint32, datumName, unitsName string, datumType uint32) error {
	if !IsLoaded() {
		return fmt.Errorf("DLL not loaded")
	}
	namePtr := append([]byte(datumName), 0)
	var unitsPtr []byte
	if unitsName != "" {
		unitsPtr = append([]byte(unitsName), 0)
	}

	var unitsArg uintptr
	if len(unitsPtr) > 0 {
		unitsArg = uintptr(unsafe.Pointer(&unitsPtr[0]))
	}

	r1, _, err := procAddToDataDefinition.Call(
		handle,
		uintptr(defineID),
		uintptr(unsafe.Pointer(&namePtr[0])),
		unitsArg,
		uintptr(datumType),
		uintptr(0),          // fEpsilon (float32)
		uintptr(0xFFFFFFFF), // DatumID
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_AddToDataDefinition failed for %s: %v (0x%x)", datumName, err, r1)
	}

	return nil
}

// RequestDataOnSimObject requests data updates for a sim object.
func RequestDataOnSimObject(handle uintptr, requestID, defineID, objectID, period, flags, origin, interval, limit uint32) error {
	if !IsLoaded() {
		return fmt.Errorf("DLL not loaded")
	}
	r1, _, err := procRequestDataOnSimObject.Call(
		handle,
		uintptr(requestID),
		uintptr(defineID),
		uintptr(objectID),
		uintptr(period),
		uintptr(flags),
		uintptr(origin),
		uintptr(interval),
		uintptr(limit),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_RequestDataOnSimObject failed: %v (0x%x)", err, r1)
	}

	return nil
}

// GetNextDispatch retrieves the next message from SimConnect.
// Returns nil, 0, nil if no message is available.
func GetNextDispatch(handle uintptr) (ppData unsafe.Pointer, cbData uint32, err error) {
	if !IsLoaded() {
		return nil, 0, fmt.Errorf("DLL not loaded")
	}
	r1, _, _ := procGetNextDispatch.Call(
		handle,
		uintptr(unsafe.Pointer(&ppData)),
		uintptr(unsafe.Pointer(&cbData)),
	)

	if uint32(r1) == EFAIL {
		// No message available
		return nil, 0, nil
	}

	if int32(r1) < 0 {
		return nil, 0, fmt.Errorf("SimConnect_GetNextDispatch failed: 0x%x", r1)
	}

	return ppData, cbData, nil
}

// SetDataOnSimObject sets data on a sim object.
func SetDataOnSimObject(handle uintptr, defineID, objectID, flags, arrayCount, cbUnitSize uint32, pDataSet unsafe.Pointer) error {
	if !IsLoaded() {
		return fmt.Errorf("DLL not loaded")
	}
	r1, _, err := procSetDataOnSimObject.Call(
		handle,
		uintptr(defineID),
		uintptr(objectID),
		uintptr(flags),
		uintptr(arrayCount),
		uintptr(cbUnitSize),
		uintptr(pDataSet),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_SetDataOnSimObject failed: %v (0x%x)", err, r1)
	}

	return nil
}

// AICreateNonATCAircraft spawns an AI aircraft.
func AICreateNonATCAircraft(handle uintptr, containerTitle, tailNumber string, initPos *InitPosition, requestID uint32) error {
	if !IsLoaded() {
		return fmt.Errorf("DLL not loaded")
	}
	titlePtr := append([]byte(containerTitle), 0)
	tailPtr := append([]byte(tailNumber), 0)

	r1, _, err := procAICreateNonATCAircraft.Call(
		handle,
		uintptr(unsafe.Pointer(&titlePtr[0])),
		uintptr(unsafe.Pointer(&tailPtr[0])),
		uintptr(unsafe.Pointer(initPos)),
		uintptr(requestID),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_AICreateNonATCAircraft failed: %v (0x%x)", err, r1)
	}

	return nil
}

// AIRemoveObject removes an AI object.
func AIRemoveObject(handle uintptr, objectID, requestID uint32) error {
	if !IsLoaded() {
		return fmt.Errorf("DLL not loaded")
	}
	r1, _, err := procAIRemoveObject.Call(
		handle,
		uintptr(objectID),
		uintptr(requestID),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_AIRemoveObject failed: %v (0x%x)", err, r1)
	}

	return nil
}

// SubscribeToSystemEvent subscribes to a system event like "SimStart" or "SimStop".
func SubscribeToSystemEvent(handle uintptr, clientEventID uint32, eventName string) error {
	if !IsLoaded() {
		return fmt.Errorf("DLL not loaded")
	}
	namePtr := append([]byte(eventName), 0)

	r1, _, err := procSubscribeToSystemEvent.Call(
		handle,
		uintptr(clientEventID),
		uintptr(unsafe.Pointer(&namePtr[0])),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_SubscribeToSystemEvent failed for %s: %v (0x%x)", eventName, err, r1)
	}

	return nil
}

// EnumerateSimObjectsAndLiveries retrieves a list of spawnable SimObjects and their liveries.
func EnumerateSimObjectsAndLiveries(handle uintptr, requestID, objType uint32) error {
	r1, _, err := procEnumerateSimObjectsAndLiveries.Call(
		handle,
		uintptr(requestID),
		uintptr(objType),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_EnumerateSimObjectsAndLiveries failed: %v (0x%x)", err, r1)
	}

	return nil
}

// AICreateNonATCAircraftEX1 spawns an AI aircraft with a specific livery (MSFS 2024).
func AICreateNonATCAircraftEX1(handle uintptr, containerTitle, livery, tailNumber string, initPos *InitPosition, requestID uint32) error {
	titlePtr := append([]byte(containerTitle), 0)
	liveryPtr := append([]byte(livery), 0)
	tailPtr := append([]byte(tailNumber), 0)

	r1, _, err := procAICreateNonATCAircraftEX1.Call(
		handle,
		uintptr(unsafe.Pointer(&titlePtr[0])),
		uintptr(unsafe.Pointer(&liveryPtr[0])),
		uintptr(unsafe.Pointer(&tailPtr[0])),
		uintptr(unsafe.Pointer(initPos)),
		uintptr(requestID),
	)

	if int32(r1) < 0 {
		return fmt.Errorf("SimConnect_AICreateNonATCAircraft_EX1 failed: %v (0x%x)", err, r1)
	}

	return nil
}
