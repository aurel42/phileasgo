package logging

import "log/slog"

// EnableTrace is a variable to enable/disable trace logs.
// Default is false to reduce noise.
var EnableTrace = false

// Trace logs a message at DEBUG level, but only if EnableTrace is true.
// This allows us to have "super debug" logs that are compiled out or skipped cheaply.
func Trace(logger *slog.Logger, msg string, args ...any) {
	if EnableTrace {
		logger.Debug(msg, args...)
	}
}

// TraceDefault logs to the default logger if EnableTrace is true.
func TraceDefault(msg string, args ...any) {
	if EnableTrace {
		slog.Debug(msg, args...)
	}
}
