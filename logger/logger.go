package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	// Log is the global logger instance
	Log *logrus.Logger
)

// compactFormatter is a custom formatter for cleaner log output
type compactFormatter struct {
	logrus.TextFormatter
}

func (f *compactFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())[0:4]

	// Skip frames to find actual caller (skip logrus + our wrapper functions)
	caller := ""
	for i := 3; i < 10; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Skip logrus internal and our logger.go
		if !strings.Contains(file, "logrus") && !strings.HasSuffix(file, "logger/logger.go") {
			// Get package name from path (e.g., "nofx/manager/trader_manager.go" -> "manager")
			dir := filepath.Dir(file)
			pkg := filepath.Base(dir)
			caller = fmt.Sprintf("%s/%s:%d", pkg, filepath.Base(file), line)
			break
		}
	}

	msg := fmt.Sprintf("[%s] %s %s\n", level, caller, entry.Message)
	return []byte(msg), nil
}

func init() {
	// Auto-initialize default logger to ensure it works before Init is called
	Log = logrus.New()
	Log.SetLevel(logrus.InfoLevel)
	Log.SetFormatter(&compactFormatter{})
	Log.SetOutput(os.Stdout)
}

// ============================================================================
// Initialization functions
// ============================================================================

// Init initializes the global logger
// If config is nil, uses default configuration (console output, info level)
func Init(cfg *Config) error {
	Log = logrus.New()

	// Use default values if no config provided
	if cfg == nil {
		cfg = &Config{Level: "info"}
	}

	// Set default values
	cfg.SetDefaults()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Set compact formatter
	Log.SetFormatter(&compactFormatter{})
	Log.SetOutput(os.Stdout)
	Log.SetReportCaller(true)

	return nil
}

// InitWithSimpleConfig initializes logger with simplified config
// Suitable for scenarios that only need basic functionality
func InitWithSimpleConfig(level string) error {
	return Init(&Config{Level: level})
}

// Shutdown gracefully shuts down the logger
func Shutdown() {
	// Reserved for future extensions
}

// ============================================================================
// Logging functions
// ============================================================================

// WithFields creates logger entry with fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

// WithField creates logger entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Log.WithField(key, value)
}

// add debug, info, warn
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

func Info(args ...interface{}) {
	Log.Info(args...)
}

func Warn(args ...interface{}) {
	Log.Warn(args...)
}

func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

func Error(args ...interface{}) {
	Log.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	Log.Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	Log.Panicf(format, args...)
}

// ============================================================================
// MCP Logger adapter
// ============================================================================

// MCPLogger adapter that allows MCP package to use the global logger
// Implements mcp.Logger interface
type MCPLogger struct{}

// NewMCPLogger creates MCP log adapter
func NewMCPLogger() *MCPLogger {
	return &MCPLogger{}
}

func (l *MCPLogger) Debugf(format string, args ...any) {
	Log.Debugf(format, args...)
}

func (l *MCPLogger) Infof(format string, args ...any) {
	Log.Infof(format, args...)
}

func (l *MCPLogger) Warnf(format string, args ...any) {
	Log.Warnf(format, args...)
}

func (l *MCPLogger) Errorf(format string, args ...any) {
	Log.Errorf(format, args...)
}
