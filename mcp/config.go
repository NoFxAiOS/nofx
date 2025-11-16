package mcp

import (
	"net/http"
	"os"
	"strconv"
	"time"
)

// Config 客户端配置（集中管理所有配置）
type Config struct {
	// Provider 配置
	Provider string
	APIKey   string
	BaseURL  string
	Model    string

	// 行为配置
	MaxTokens   int
	Temperature float64
	UseFullURL  bool

	// 重试配置
	MaxRetries     int
	RetryWaitBase  time.Duration
	RetryableErrors []string

	// 超时配置
	Timeout time.Duration

	// 依赖注入
	Logger     Logger
	HTTPClient *http.Client
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		// 默认值
		MaxTokens:      getEnvInt("AI_MAX_TOKENS", 2000),
		Temperature:    MCPClientTemperature,
		MaxRetries:     MaxRetryTimes,
		RetryWaitBase:  2 * time.Second,
		Timeout:        DefaultTimeout,
		RetryableErrors: retryableErrors,

		// 默认依赖
		Logger:     &defaultLogger{},
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

// getEnvInt 从环境变量读取整数，失败则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

// getEnvString 从环境变量读取字符串，为空则返回默认值
func getEnvString(key string, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
