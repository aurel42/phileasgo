package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"phileasgo/pkg/config"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	webview "github.com/webview/webview_go"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	procCreateMutex        = kernel32.NewProc("CreateMutexW")
	procGetLastError       = kernel32.NewProc("GetLastError")
	procGetModuleHandle    = kernel32.NewProc("GetModuleHandleW")
	procLoadIcon           = user32.NewProc("LoadIconW")
	procSendMessage        = user32.NewProc("SendMessageW")
	procFindWindow         = user32.NewProc("FindWindowW")
	procGetWindowPlacement = user32.NewProc("GetWindowPlacement")
	procSetWindowPlacement = user32.NewProc("SetWindowPlacement")
	procMonitorFromRect    = user32.NewProc("MonitorFromRect")
	procSetWindowLongPtr   = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc     = user32.NewProc("CallWindowProcW")
)

var originalWndProc uintptr

const (
	WM_SETICON = 0x0080
	ICON_SMALL = 0
	ICON_BIG   = 1

	MONITOR_DEFAULTTONEAREST = 0x00000002
)

type POINT struct {
	X int32
	Y int32
}

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type WINDOWPLACEMENT struct {
	Length         uint32
	Flags          uint32
	ShowCmd        uint32
	MinPosition    POINT
	MaxPosition    POINT
	NormalPosition RECT
}

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

	// Load Main Config (for Server Address)
	mainCfg, err := config.Load("configs/phileas.yaml")
	if err != nil {
		// Fallback if load fails (though in prod it should exist)
		mainCfg = config.DefaultConfig()
	}

	// Load GUI Config (for Window State)
	guiCfg, err := config.LoadGUIConfig("configs/gui.yaml")
	if err != nil {
		// Fallback to defaults
		guiCfg, _ = config.LoadGUIConfig("configs/gui.yaml") // Force default
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
	// Set initial size, but we might override it with placement
	w.SetSize(guiCfg.Window.Width, guiCfg.Window.Height, webview.HintNone)

	// GUI Maintenance (Icon, Restore, Hook)
	go func() {
		iconSet := false
		var hwnd uintptr
		for {
			hwnd = uintptr(w.Window())
			if hwnd != 0 {
				break
			}
			runtime.Gosched()
		}

		// Initial Restore
		restoreWindowPlacement(hwnd, guiCfg)

		// Native Hook for "Save on Close"
		subclassWindow(hwnd, guiCfg)

		// Icon Maintenance
		for {
			hwnd = uintptr(w.Window())
			if hwnd == 0 {
				break
			}
			if !iconSet {
				if setWindowIcon("PhileasGUI") {
					iconSet = true
				}
			}
			time.Sleep(1 * time.Second)
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

	mgr := NewManager(logProxy, termProxy, appProxy, mainCfg.Server.Address)

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
	mgr.Stop()
}

func restoreWindowPlacement(hwnd uintptr, cfg *config.GUIConfig) {
	if cfg.Window.X == -1 {
		return // Let OS decide
	}

	wp := WINDOWPLACEMENT{}
	wp.Length = uint32(unsafe.Sizeof(wp))

	// Get current placement to fill in defaults
	_, _, _ = procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))

	// Override with saved values
	wp.NormalPosition = RECT{
		Left:   int32(cfg.Window.X),
		Top:    int32(cfg.Window.Y),
		Right:  int32(cfg.Window.X + cfg.Window.Width),
		Bottom: int32(cfg.Window.Y + cfg.Window.Height),
	}

	if cfg.Window.Maximized {
		wp.ShowCmd = 3 // SW_SHOWMAXIMIZED
	}

	// Safety Check: Ensure the window is on a visible monitor
	hMonitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&wp.NormalPosition)), MONITOR_DEFAULTTONEAREST)
	if hMonitor == 0 {
		return // Something is wrong, don't apply
	}

	_, _, _ = procSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
}

func subclassWindow(hwnd uintptr, cfg *config.GUIConfig) {
	// WM_CLOSE is 0x0010
	// GWLP_WNDPROC is -4
	callback := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		if msg == 0x0010 { // WM_CLOSE
			saveWindowPlacement(hwnd, cfg)
		}
		ret, _, _ := procCallWindowProc.Call(originalWndProc, hwnd, uintptr(msg), wParam, lParam)
		return ret
	})

	gwlpWndProc := int32(-4)
	ptr, _, _ := procSetWindowLongPtr.Call(hwnd, uintptr(gwlpWndProc), callback)
	originalWndProc = ptr
}

func saveWindowPlacement(hwnd uintptr, cfg *config.GUIConfig) {
	if hwnd == 0 {
		return
	}

	wp := WINDOWPLACEMENT{}
	wp.Length = uint32(unsafe.Sizeof(wp))

	ret, _, _ := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
	if ret == 0 {
		return
	}

	cfg.Window.X = int(wp.NormalPosition.Left)
	cfg.Window.Y = int(wp.NormalPosition.Top)
	cfg.Window.Width = int(wp.NormalPosition.Right - wp.NormalPosition.Left)
	cfg.Window.Height = int(wp.NormalPosition.Bottom - wp.NormalPosition.Top)
	cfg.Window.Maximized = (wp.ShowCmd == 3) // SW_SHOWMAXIMIZED

	_ = config.SaveGUIConfig("configs/gui.yaml", cfg)
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
