package api

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	modpsapi    = syscall.NewLazyDLL("psapi.dll")

	procGetProcessTimes          = modkernel32.NewProc("GetProcessTimes")
	procGetProcessMemoryInfo     = modpsapi.NewProc("GetProcessMemoryInfo")
	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = modkernel32.NewProc("Process32FirstW")
	procProcess32Next            = modkernel32.NewProc("Process32NextW")
)

type FILETIME struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func (ft *FILETIME) Nanoseconds() int64 {
	// 100-nanosecond intervals since January 1, 1601 (UTC)
	return (int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)) * 100
}

type PROCESS_MEMORY_COUNTERS_EX struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type PROCESSENTRY32 struct {
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

const (
	TH32CS_SNAPPROCESS = 0x00000002
)

// GetProcessStats returns the CPU time (nanoseconds) and RSS memory (bytes) for a given PID.
func GetProcessStats(pid int) (cpuNS int64, rssBytes uint64, err error) {
	const PROCESS_QUERY_INFORMATION = 0x0400
	const PROCESS_VM_READ = 0x0010

	h, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = syscall.CloseHandle(h) }()

	// CPU Times
	var creationTime, exitTime, kernelTime, userTime FILETIME
	ret, _, _ := procGetProcessTimes.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret != 0 {
		cpuNS = kernelTime.Nanoseconds() + userTime.Nanoseconds()
	}

	// Memory
	var counters PROCESS_MEMORY_COUNTERS_EX
	counters.CB = uint32(unsafe.Sizeof(counters))
	ret, _, _ = procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.CB),
	)
	if ret != 0 {
		rssBytes = uint64(counters.WorkingSetSize)
	}

	return cpuNS, rssBytes, nil
}

// GetChildPIDs returns all children of a given PID.
func GetChildPIDs(parentPID int) ([]int, error) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return nil, os.NewSyscallError("CreateToolhelp32Snapshot", syscall.GetLastError())
	}
	defer func() { _ = syscall.CloseHandle(syscall.Handle(snapshot)) }()

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, os.NewSyscallError("Process32First", syscall.GetLastError())
	}

	var children []int
	for {
		if int(entry.ParentProcessID) == parentPID {
			children = append(children, int(entry.ProcessID))
		}

		ret, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return children, nil
}
