package logging

import (
	"sync"
)

// LogCaptureWriter is a thread-safe writer that stores the last written line.
type LogCaptureWriter struct {
	mu       sync.RWMutex
	lastLine string
}

// GlobalLogCapture is the singleton instance for capturing logs.
var GlobalLogCapture = &LogCaptureWriter{}

// GlobalEventCapture is the singleton instance for capturing trip events.
var GlobalEventCapture = &LogCaptureWriter{}

// Write implements io.Writer. It updates the lastLine field.
func (w *LogCaptureWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastLine = string(p)
	return len(p), nil
}

// GetLastLine returns the most recent log line.
func (w *LogCaptureWriter) GetLastLine() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastLine
}
