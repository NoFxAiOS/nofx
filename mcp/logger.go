package mcp

// Logger 日志接口（抽象依赖）
// 使用 Printf 风格的方法名，方便集成 logrus、zap 等主流日志库
// 默认使用全局 logger 包（见 mcp/config.go）
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// noopLogger 空日志实现（测试时使用）
type noopLogger struct{}

func (l *noopLogger) Debugf(format string, args ...any) {}
func (l *noopLogger) Infof(format string, args ...any)  {}
func (l *noopLogger) Warnf(format string, args ...any)  {}
func (l *noopLogger) Errorf(format string, args ...any) {}

// NewNoopLogger 创建空日志器（测试使用）
func NewNoopLogger() Logger {
	return &noopLogger{}
}
