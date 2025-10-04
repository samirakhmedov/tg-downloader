package logger

import "go.uber.org/fx/fxevent"

// Logger is the main logging class that delegates logging to multiple strategies.
// It allows dynamic control of logging behavior by using different strategy implementations.
type Logger struct {
	// strategies is a list of logging strategies that will receive all log messages
	strategies []ILoggerStrategy
}

func (l *Logger) NewFxLogger(logger *Logger) fxevent.Logger {
	return &FxLogger{logger: logger}
}

// NewLogger creates a new Logger with the provided list of strategies.
// All logging calls will be delegated to each strategy in the list.
func NewLogger(strategies []ILoggerStrategy) *Logger {
	return &Logger{
		strategies: strategies,
	}
}

// Debug logs a debug-level message to all registered strategies.
func (l *Logger) Debug(message string) {
	for _, strategy := range l.strategies {
		strategy.Debug(message)
	}
}

// Info logs an info-level message to all registered strategies.
func (l *Logger) Info(message string) {
	for _, strategy := range l.strategies {
		strategy.Info(message)
	}
}

// Warn logs a warning-level message to all registered strategies.
func (l *Logger) Warn(message string) {
	for _, strategy := range l.strategies {
		strategy.Warn(message)
	}
}

// Error logs an error-level message to all registered strategies.
func (l *Logger) Error(message string) {
	for _, strategy := range l.strategies {
		strategy.Error(message)
	}
}
