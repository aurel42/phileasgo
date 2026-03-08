package simconnect

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = modkernel32.NewProc("Process32FirstW")
	procProcess32Next            = modkernel32.NewProc("Process32NextW")
)

const (
	th32csSnapprocess = 0x00000002
)

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// IsSimulatorRunning efficiently checks if a Microsoft Flight Simulator process is active.
// It checks for any executable containing the provided process name.
func IsSimulatorRunning(processName string) bool {
	if processName == "" {
		processName = "flightsimulator" // Failsafe default
	}
	processName = strings.ToLower(processName)

	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapprocess, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return false // Failsafe, assume not running on API failure
	}
	defer func() { _ = syscall.CloseHandle(syscall.Handle(snapshot)) }()

	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return false
	}

	for {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		nameLower := strings.ToLower(name)

		// Matches based on configured name (e.g. "flightsimulator" for 2020/2024)
		if strings.Contains(nameLower, processName) {
			return true
		}

		ret, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return false
}
