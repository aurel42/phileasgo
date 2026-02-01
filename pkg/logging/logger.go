package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/model"
)

// RequestLogger is the logger instance for HTTP requests.
var RequestLogger *slog.Logger

// eventLogPath is the path to the event log file.
var eventLogPath string

// eventLogMu protects concurrent writes to the event log.
var eventLogMu sync.Mutex

// Init initializes the logging system based on configuration.
// It returns a cleanup function to close log files.
func Init(cfg *config.LogConfig, hCfg *config.HistoryConfig) (func(), error) {
	// Rotate standard log files at startup
	rotatePaths(cfg.Server.Path, cfg.Requests.Path, cfg.Events.Path)

	// Set up event log path
	SetEventLogPath(cfg.Events.Path)

	// Rotate history files only if enabled
	if hCfg != nil {
		if hCfg.LLM.Enabled {
			rotatePaths(hCfg.LLM.Path)
		}
		if hCfg.TTS.Enabled {
			rotatePaths(hCfg.TTS.Path)
		}
	}

	var closers []io.Closer

	// 1. Setup Server Logger (Stdout + File)
	serverHandler, file1, err := setupHandler(cfg.Server.Path, cfg.Server.Level, true)
	if err != nil {
		return nil, fmt.Errorf("failed to setup server logger: %w", err)
	}
	if file1 != nil {
		closers = append(closers, file1)
	}
	slog.SetDefault(slog.New(serverHandler))

	// 2. Setup Requests Logger (File Only)
	requestHandler, file2, err := setupHandler(cfg.Requests.Path, cfg.Requests.Level, false)
	if err != nil {
		// Try to close first file if second fails (best effort)
		if file1 != nil {
			file1.Close()
		}
		return nil, fmt.Errorf("failed to setup requests logger: %w", err)
	}
	if file2 != nil {
		closers = append(closers, file2)
	}
	RequestLogger = slog.New(requestHandler)

	return func() {
		for _, c := range closers {
			c.Close()
		}
	}, nil
}

func setupHandler(path, levelStr string, stdout bool) (handler slog.Handler, file *os.File, err error) {
	// Parse Level
	var level slog.Level
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create Directory
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, err
	}

	// Open File (Append mode, truncation handled in Init)
	file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	// Options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	// Create File Handler
	fileHandler := slog.NewTextHandler(file, opts)

	if !stdout {
		return fileHandler, file, nil
	}

	// Console Handler - only INFO and up
	consoleOpts := &slog.HandlerOptions{
		Level: mathMaxLevel(level, slog.LevelInfo), // Cap at Info unless file is even higher
	}
	consoleHandler := slog.NewTextHandler(os.Stdout, consoleOpts)

	// 4. Capture Handler - for Overlay (INFO+)
	captureHandler := slog.NewTextHandler(GlobalLogCapture, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	handlers := []slog.Handler{fileHandler, consoleHandler, captureHandler}
	return &multiHandler{handlers: handlers}, file, nil
}

func mathMaxLevel(a, b slog.Level) slog.Level {
	if a > b {
		return a
	}
	return b
}

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle implements slog.Handler
// nolint:gocritic // r must be passed by value to implement slog.Handler
func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// rotatePaths rotates the given log files if they exist by renaming them to .old.
// This is called at the start of Init to ensure logs are fresh each run but previous logs are kept.
func rotatePaths(paths ...string) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}

		// If file exists, rotate it
		if _, err := os.Stat(p); err == nil {
			oldPath := p + ".old"
			// Remove existing .old if present
			_ = os.Remove(oldPath)
			// Rename current to .old
			_ = os.Rename(p, oldPath)
		}
	}
}

// SetEventLogPath configures the path for the event log file.
func SetEventLogPath(path string) {
	eventLogMu.Lock()
	defer eventLogMu.Unlock()
	eventLogPath = path
}

// LogEvent writes a trip event to the event log file.
func LogEvent(event *model.TripEvent) {
	eventLogMu.Lock()
	defer eventLogMu.Unlock()

	if eventLogPath == "" {
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(eventLogPath), 0o755); err != nil {
		slog.Error("failed to create event log directory", "error", err)
		return
	}

	// Open file in append mode
	f, err := os.OpenFile(eventLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("failed to open event log", "error", err)
		return
	}
	defer f.Close()

	// Format: [2006-01-02 15:04:05] [type] Title - Summary
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	line := fmt.Sprintf("[%s] [%s] %s", ts.Format("2006-01-02 15:04:05"), event.Type, event.Title)

	if event.Summary != "" {
		line += " - " + event.Summary
	}
	line += "\n"

	if _, err := f.WriteString(line); err != nil {
		slog.Error("failed to write event log", "error", err)
	}

	// Also capture for the overlay
	_, _ = GlobalEventCapture.Write([]byte(strings.TrimSpace(line)))
}
