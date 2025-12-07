package mcp

// Logger interface (abstract dependency)
// Uses Printf-style method names for easy integration with mainstream logging libraries like logrus, zap, etc.
// Default uses global logger package (see mcp/config.go)
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// noopLogger no-op logger implementation (used in tests)
type noopLogger struct{}

func (l *noopLogger) Debugf(format string, args ...any) {}
func (l *noopLogger) Infof(format string, args ...any)  {}
func (l *noopLogger) Warnf(format string, args ...any)  {}
func (l *noopLogger) Errorf(format string, args ...any) {}

// NewNoopLogger creates no-op logger (for testing)
func NewNoopLogger() Logger {
	return &noopLogger{}
}
