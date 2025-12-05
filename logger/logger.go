package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	// Log 全局logger实例
	Log *logrus.Logger
)

func init() {
	// 自动初始化默认 logger，确保在 Init 被调用前也能使用
	Log = logrus.New()
	Log.SetLevel(logrus.InfoLevel)
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	Log.SetOutput(os.Stdout)
}

// ============================================================================
// 初始化函数
// ============================================================================

// Init 初始化全局logger
// 如果config为nil，使用默认配置（console输出，info级别）
func Init(cfg *Config) error {
	Log = logrus.New()

	// 如果没有配置，使用默认值
	if cfg == nil {
		cfg = &Config{Level: "info"}
	}

	// 设置默认值
	cfg.SetDefaults()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// 设置格式化器（固定使用彩色文本格式）
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	// 设置输出目标（默认stdout）
	Log.SetOutput(os.Stdout)

	// 启用调用位置信息
	Log.SetReportCaller(true)

	return nil
}

// InitWithSimpleConfig 使用简化配置初始化logger
// 适用于只需要基本功能的场景
func InitWithSimpleConfig(level string) error {
	return Init(&Config{Level: level})
}

// Shutdown 优雅关闭logger
func Shutdown() {
	// 预留用于未来扩展
}

// ============================================================================
// 日志记录函数
// ============================================================================

// WithFields 创建带字段的logger entry
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

// WithField 创建带单个字段的logger entry
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
// MCP Logger 适配器
// ============================================================================

// MCPLogger 适配器，使 MCP 包使用全局 logger
// 实现 mcp.Logger 接口
type MCPLogger struct{}

// NewMCPLogger 创建 MCP 日志适配器
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
