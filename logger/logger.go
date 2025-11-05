package logger

import (
	"nofx/config"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	// Log 全局logger实例
	Log *logrus.Logger

	// telegramHook 保存hook引用，用于优雅关闭
	telegramHook *TelegramHook
)

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

	// 添加Telegram Hook（可选）
	if cfg.Telegram != nil && cfg.Telegram.Enabled {
		if err := setupTelegramHook(cfg.Telegram); err != nil {
			Log.Warnf("初始化Telegram推送失败，将继续使用普通日志: %v", err)
		}
	}

	return nil
}

// setupTelegramHook 设置Telegram Hook
func setupTelegramHook(telegramCfg *TelegramConfig) error {
	hook, err := NewTelegramHook(telegramCfg)
	if err != nil {
		return err
	}

	Log.AddHook(hook)
	telegramHook = hook
	Log.Info("✅ Telegram日志推送已启用")
	return nil
}

// InitWithSimpleConfig 使用简化配置初始化logger
// 适用于只需要基本功能的场景
func InitWithSimpleConfig(level string) error {
	return Init(&Config{Level: level})
}

// InitWithTelegram 使用Telegram配置初始化logger
func InitWithTelegram(botToken string, chatID int64) error {
	return Init(&Config{
		Level: "info",
		Telegram: &TelegramConfig{
			Enabled:  true,
			BotToken: botToken,
			ChatID:   chatID,
		},
	})
}

// InitFromLogConfig 从config.LogConfig初始化logger
func InitFromLogConfig(logConfig *config.LogConfig) error {
	if logConfig == nil {
		return InitWithSimpleConfig("info")
	}

	cfg := &Config{
		Level: logConfig.Level,
	}

	if cfg.Level == "" {
		cfg.Level = "info"
	}

	// 如果启用了Telegram，添加配置
	if logConfig.Telegram != nil && logConfig.Telegram.Enabled {
		if botToken := logConfig.Telegram.BotToken; botToken != "" && logConfig.Telegram.ChatID != 0 {
			cfg.Telegram = &TelegramConfig{
				Enabled:  true,
				BotToken: botToken,
				ChatID:   logConfig.Telegram.ChatID,
				MinLevel: logConfig.Telegram.MinLevel,
			}
		}
	}

	return Init(cfg)
}

// InitFromParams 从参数初始化logger
// 适用于不依赖config包的场景
func InitFromParams(level string, telegramEnabled bool, botToken string, chatID int64) error {
	cfg := &Config{Level: level}

	if telegramEnabled && botToken != "" && chatID != 0 {
		cfg.Telegram = &TelegramConfig{
			Enabled:  true,
			BotToken: botToken,
			ChatID:   chatID,
		}
	}

	return Init(cfg)
}

// Shutdown 优雅关闭logger（主要用于关闭Telegram发送器）
func Shutdown() {
	if telegramHook != nil {
		telegramHook.Stop()
		telegramHook = nil
	}
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
