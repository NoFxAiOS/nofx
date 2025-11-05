package logger

import (
	"github.com/sirupsen/logrus"
)

// Config 日志配置（简化版）
type Config struct {
	Level    string          `json:"level"`    // 日志级别: debug, info, warn, error (默认: info)
	Telegram *TelegramConfig `json:"telegram"` // Telegram推送配置（可选）
}

// TelegramConfig Telegram推送配置（简化版，高级参数使用默认值）
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`   // 是否启用（默认: false）
	BotToken string `json:"bot_token"` // Bot Token
	ChatID   int64  `json:"chat_id"`   // Chat ID
	MinLevel string `json:"min_level"` // 最低日志级别，该级别及以上的日志会推送到Telegram（可选，默认: error）
}

// SetDefaults 设置默认值
func (c *Config) SetDefaults() {
	if c.Level == "" {
		c.Level = "info"
	}
}

// GetLogrusLevels 返回要推送到Telegram的日志级别
// 根据配置的MinLevel返回该级别及以上的所有日志级别
// 如果未配置或配置无效，默认返回error, fatal, panic（向后兼容）
func (tc *TelegramConfig) GetLogrusLevels() []logrus.Level {
	// 如果未配置，使用默认值error（向后兼容）
	minLevelStr := tc.MinLevel
	if minLevelStr == "" {
		minLevelStr = "error"
	}

	// 解析配置的日志级别
	minLevel, err := logrus.ParseLevel(minLevelStr)
	if err != nil {
		// 如果解析失败，使用默认值error（向后兼容）
		minLevel = logrus.ErrorLevel
	}

	// 定义所有日志级别（从高到低：panic, fatal, error, warn, info, debug）
	allLevels := []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}

	// 返回所有大于等于minLevel的日志级别
	var result []logrus.Level
	for _, level := range allLevels {
		if level <= minLevel {
			result = append(result, level)
		}
	}

	return result
}
