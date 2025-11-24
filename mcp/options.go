package mcp

import (
	"net/http"
	"time"
)

// ClientOption 客户端选项函数（Functional Options 模式）
type ClientOption func(*Config)

// ============================================================
// 依赖注入选项
// ============================================================

// WithLogger 设置自定义日志器
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithLogger(customLogger))
func WithLogger(logger Logger) ClientOption {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端
//
// 使用示例：
//   httpClient := &http.Client{Timeout: 60 * time.Second}
//   client := mcp.NewClient(mcp.WithHTTPClient(httpClient))
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// ============================================================
// 超时和重试选项
// ============================================================

// WithTimeout 设置请求超时时间
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithTimeout(60 * time.Second))
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Config) {
		c.Timeout = timeout
		c.HTTPClient.Timeout = timeout
	}
}

// WithMaxRetries 设置最大重试次数
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithMaxRetries(5))
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Config) {
		c.MaxRetries = maxRetries
	}
}

// WithRetryWaitBase 设置重试等待基础时长
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithRetryWaitBase(3 * time.Second))
func WithRetryWaitBase(waitTime time.Duration) ClientOption {
	return func(c *Config) {
		c.RetryWaitBase = waitTime
	}
}

// ============================================================
// AI 参数选项
// ============================================================

// WithMaxTokens 设置最大 token 数
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithMaxTokens(4000))
func WithMaxTokens(maxTokens int) ClientOption {
	return func(c *Config) {
		c.MaxTokens = maxTokens
	}
}

// WithTemperature 设置温度参数
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithTemperature(0.7))
func WithTemperature(temperature float64) ClientOption {
	return func(c *Config) {
		c.Temperature = temperature
	}
}

// ============================================================
// Provider 配置选项
// ============================================================

// WithAPIKey 设置 API Key
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Config) {
		c.APIKey = apiKey
	}
}

// WithBaseURL 设置基础 URL
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Config) {
		c.BaseURL = baseURL
	}
}

// WithModel 设置模型名称
func WithModel(model string) ClientOption {
	return func(c *Config) {
		c.Model = model
	}
}

// WithProvider 设置提供商
func WithProvider(provider string) ClientOption {
	return func(c *Config) {
		c.Provider = provider
	}
}

// WithUseFullURL 设置是否使用完整 URL
func WithUseFullURL(useFullURL bool) ClientOption {
	return func(c *Config) {
		c.UseFullURL = useFullURL
	}
}

// ============================================================
// 组合选项（便捷方法）
// ============================================================

// WithDeepSeekConfig 设置 DeepSeek 配置
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithDeepSeekConfig("sk-xxx"))
func WithDeepSeekConfig(apiKey string) ClientOption {
	return func(c *Config) {
		c.Provider = ProviderDeepSeek
		c.APIKey = apiKey
		c.BaseURL = DefaultDeepSeekBaseURL
		c.Model = DefaultDeepSeekModel
	}
}

// WithQwenConfig 设置 Qwen 配置
//
// 使用示例：
//   client := mcp.NewClient(mcp.WithQwenConfig("sk-xxx"))
func WithQwenConfig(apiKey string) ClientOption {
	return func(c *Config) {
		c.Provider = ProviderQwen
		c.APIKey = apiKey
		c.BaseURL = DefaultQwenBaseURL
		c.Model = DefaultQwenModel
	}
}
