package logger

import (
	"log"
)

// DebugLoggerStrategy is a logger strategy that only logs when debug mode is enabled.
// It uses the standard Go log package with prefixed log levels.
type DebugLoggerStrategy struct {
	// debugEnabled determines if logging should be active
	debugEnabled bool
}

// NewDebugLoggerStrategy creates a new debug logger strategy.
// The strategy will only log messages if debugEnabled is true.
func NewDebugLoggerStrategy(debugEnabled bool) ILoggerStrategy {
	return &DebugLoggerStrategy{
		debugEnabled: debugEnabled,
	}
}

// Debug logs a debug-level message if debug mode is enabled.
// The message is prefixed with [DEBUG].
func (d *DebugLoggerStrategy) Debug(message string) {
	if d.debugEnabled {
		log.Printf("[DEBUG] %s", message)
	}
}

// Info logs an info-level message if debug mode is enabled.
// The message is prefixed with [INFO].
func (d *DebugLoggerStrategy) Info(message string) {
	if d.debugEnabled {
		log.Printf("[INFO] %s", message)
	}
}

// Warn logs a warning-level message if debug mode is enabled.
// The message is prefixed with [WARN].
func (d *DebugLoggerStrategy) Warn(message string) {
	if d.debugEnabled {
		log.Printf("[WARN] %s", message)
	}
}

// Error logs an error-level message if debug mode is enabled.
// The message is prefixed with [ERROR].
func (d *DebugLoggerStrategy) Error(message string) {
	if d.debugEnabled {
		log.Printf("[ERROR] %s", message)
	}
}
