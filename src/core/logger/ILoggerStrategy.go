package logger

// ILoggerStrategy defines the interface for logging strategies.
// Each strategy implementation can decide how and where to log messages.
// All methods accept only a string message parameter and print on a new line.
type ILoggerStrategy interface {
	// Debug logs a debug-level message
	Debug(message string)

	// Info logs an info-level message
	Info(message string)

	// Warn logs a warning-level message
	Warn(message string)

	// Error logs an error-level message
	Error(message string)
}
