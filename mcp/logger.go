package mcp

import "log"

// Logger 日志接口（抽象依赖）
// 使用 Printf 风格的方法名，方便集成 logrus、zap 等主流日志库
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// defaultLogger 默认日志实现（包装标准库 log）
type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, args ...any) {
	log.Printf("[DEBUG] "+format, args...)
}

func (l *defaultLogger) Infof(format string, args ...any) {
	log.Printf("[INFO] "+format, args...)
}

func (l *defaultLogger) Warnf(format string, args ...any) {
	log.Printf("[WARN] "+format, args...)
}

func (l *defaultLogger) Errorf(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
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

// ============================================================
// 适配第三方日志库示例
// ============================================================

// Logrus 适配示例：
// type LogrusLogger struct {
//     logger *logrus.Logger
// }
//
// func (l *LogrusLogger) Infof(format string, args ...any) {
//     l.logger.Infof(format, args...)
// }
//
// Zap 适配示例：
// type ZapLogger struct {
//     logger *zap.Logger
// }
//
// func (l *ZapLogger) Infof(format string, args ...any) {
//     l.logger.Sugar().Infof(format, args...)
// }
//
// 然后通过 WithLogger(logger) 注入
