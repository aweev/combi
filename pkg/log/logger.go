// internal/platform/log/logger.go
package log

import "context"

// Attr represents a single key-value pair for structured logging.
type Attr struct {
	Key   string
	Value any
}

// Logger defines the standardized interface for structured logging in the application.
// It is designed to be a thin abstraction over a concrete logging library like Zap or Slog.
type Logger interface {
	Debug(msg string, attrs ...Attr)
	Info(msg string, attrs ...Attr)
	Warn(msg string, attrs ...Attr)
	Error(msg string, err error, attrs ...Attr)

	// With returns a new logger that includes the given attributes in all subsequent logs.
	With(attrs ...Attr) Logger
}

// --- Attribute Helper Functions ---
// These helpers provide a typed and convenient way to create log attributes.

func String(key, value string) Attr {
	return Attr{Key: key, Value: value}
}

func Int(key string, value int) Attr {
	return Attr{Key: key, Value: value}
}

func Bool(key string, value bool) Attr {
	return Attr{Key: key, Value: value}
}

func Float64(key string, value float64) Attr {
	return Attr{Key: key, Value: value}
}

func Any(key string, value any) Attr {
	return Attr{Key: key, Value: value}
}

func ErrorAttr(err error) Attr {
	return Attr{Key: "error", Value: err}
}

// --- Context Handling ---

type contextKey string

const loggerKey = contextKey("logger")

// WithContext returns a new context with the provided logger.
func WithContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from the context.
// If no logger is found, it returns a no-op logger to prevent nil pointer errors.
func FromContext(ctx context.Context) Logger {
	if logger, ok := ctx.Value(loggerKey).(Logger); ok {
		return logger
	}
	// Return a no-op logger so that code can safely call log.FromContext(ctx).Info(...)
	// without needing to check for nil.
	return &NoOpLogger{}
}

// NoOpLogger is a logger that does nothing. It's useful for tests or for
// FromContext to return a safe default.
type NoOpLogger struct{}

func (l *NoOpLogger) Debug(msg string, attrs ...Attr)            {}
func (l *NoOpLogger) Info(msg string, attrs ...Attr)             {}
func (l *NoOpLogger) Warn(msg string, attrs ...Attr)             {}
func (l *NoOpLogger) Error(msg string, err error, attrs ...Attr) {}
func (l *NoOpLogger) With(attrs ...Attr) Logger                  { return l }
