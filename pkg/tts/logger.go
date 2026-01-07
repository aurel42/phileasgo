package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logPath = "logs/tts.log"
	mu      sync.RWMutex
)

// SetLogPath configures the path for the TTS log file.
func SetLogPath(path string) {
	mu.Lock()
	defer mu.Unlock()
	logPath = path
}

// Log appends the TTS prompt and status to the configured log file.
// This is a shared helper for all TTS providers to ensure consistent debugging visibility.
func Log(provider, prompt string, status int, err error) {
	mu.RLock()
	path := logPath
	mu.RUnlock()

	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	f, fileErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if fileErr != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	statusStr := fmt.Sprintf("%d", status)
	if err != nil {
		statusStr = fmt.Sprintf("ERROR(%v)", err)
	}

	// Format: [TIMESTAMP] [PROVIDER] STATUS: <code> | PROMPT: <prompt>
	entry := fmt.Sprintf("[%s] [%s] STATUS: %s\nPROMPT:\n%s\n--------------------------------------------------\n",
		timestamp, provider, statusStr, prompt)

	_, _ = f.WriteString(entry)
}
