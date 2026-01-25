package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	webview "github.com/webview/webview_go"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	user32              = syscall.NewLazyDLL("user32.dll")
	procCreateMutex     = kernel32.NewProc("CreateMutexW")
	procGetLastError    = kernel32.NewProc("GetLastError")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	procLoadIcon        = user32.NewProc("LoadIconW")
	procSendMessage     = user32.NewProc("SendMessageW")
	procFindWindow      = user32.NewProc("FindWindowW")
)

const (
	WM_SETICON = 0x0080
	ICON_SMALL = 0
	ICON_BIG   = 1
)

func main() {
	// Single Instance Check (Windows Named Mutex)
	const mutexName = "Global\\PhileasGUI_Session"
	ptrName, _ := syscall.UTF16PtrFromString(mutexName)

	// Call CreateMutexW(security_attributes, initial_owner, name)
	handle, _, _ := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(ptrName)))

	// Check GetLastError for ERROR_ALREADY_EXISTS (183)
	errCode, _, _ := procGetLastError.Call()
	if errCode == 183 {
		// Already running.
		return
	}
	defer func() {
		_ = syscall.CloseHandle(syscall.Handle(handle))
	}()

	// Webview requires main thread
	runtime.LockOSThread()

	// Ensure we run from the executable directory to find data/ and .env
	exe, _ := os.Executable()
	if err := os.Chdir(filepath.Dir(exe)); err != nil {
		panic(err)
	}

	w := webview.New(true)
	defer w.Destroy()

	// Aggressively block context menu via injection
	w.Init(`
		window.addEventListener('contextmenu', function(e) {
			e.preventDefault();
		}, true); // Use capture phase
	`)

	w.SetTitle("PhileasGUI")
	w.SetSize(614, 960, webview.HintNone)

	// Set window icon from embedded resource
	go func() {
		// Give the window time to be created
		for i := 0; i < 50; i++ {
			if setWindowIcon("PhileasGUI") {
				break
			}
			// Brief sleep
			runtime.Gosched()
		}
	}()

	// Go bindings calling JS functions
	logProxy := func(msg string) {
		w.Dispatch(func() {
			w.Eval("window.addLogLine(" + escapeJS(msg) + ")")
		})
	}

	termProxy := func(name string) {
		w.Dispatch(func() {
			w.Eval("window.setTerminalTitle(" + escapeJS(name) + ")")
		})
	}

	appProxy := func(url string) {
		w.Dispatch(func() {
			w.Eval("window.enableApp(" + escapeJS(url) + ")")
		})
	}

	mgr := NewManager(logProxy, termProxy, appProxy)
	defer mgr.Stop()

	_ = w.Bind("appReady", func() {
		// Callback from JS if needed
	})

	// Start local server to serve UI (avoids "Public connection" errors)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	go func() {
		if err := http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(htmlContent))
		})); err != nil {
			panic(err)
		}
	}()

	w.Navigate("http://" + ln.Addr().String())

	// Start manager loop
	mgr.Start()

	w.Run()
}

func escapeJS(s string) string {
	b, _ := json.Marshal(s)
	// json.Marshal returns "string", surrounding quotes included.
	return string(b)
}

// setWindowIcon finds the window by title and sets its icon from the embedded resource.
func setWindowIcon(title string) bool {
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	// Find the window by title
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return false
	}

	// Get module handle (NULL = current exe)
	hInstance, _, _ := procGetModuleHandle.Call(0)

	// Load the "APP" icon from resources (resource ID used by go-winres)
	// The default resource name is "APP" which go-winres maps to an ordinal
	iconName, _ := syscall.UTF16PtrFromString("APP")
	hIcon, _, _ := procLoadIcon.Call(hInstance, uintptr(unsafe.Pointer(iconName)))
	if hIcon == 0 {
		// Try with ordinal 1 as fallback
		hIcon, _, _ = procLoadIcon.Call(hInstance, uintptr(1))
	}
	if hIcon == 0 {
		return false
	}

	// Send WM_SETICON for both small and big icons
	_, _, _ = procSendMessage.Call(hwnd, WM_SETICON, ICON_SMALL, hIcon)
	_, _, _ = procSendMessage.Call(hwnd, WM_SETICON, ICON_BIG, hIcon)

	return true
}
