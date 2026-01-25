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
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex  = kernel32.NewProc("CreateMutexW")
	procGetLastError = kernel32.NewProc("GetLastError")
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
