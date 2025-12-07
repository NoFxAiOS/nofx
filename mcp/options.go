package mcp

import (
	"net/http"
	"time"
)

// ClientOption client option function (Functional Options pattern)
type ClientOption func(*Config)

// ============================================================
// Dependency Injection Options
// ============================================================

// WithLogger sets custom logger
//
// Usage example:
//   client := mcp.NewClient(mcp.WithLogger(customLogger))
func WithLogger(logger Logger) ClientOption {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithHTTPClient sets custom HTTP client
//
// Usage example:
//   httpClient := &http.Client{Timeout: 60 * time.Second}
//   client := mcp.NewClient(mcp.WithHTTPClient(httpClient))
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// ============================================================
// Timeout and Retry Options
// ============================================================

// WithTimeout sets request timeout duration
//
// Usage example:
//   client := mcp.NewClient(mcp.WithTimeout(60 * time.Second))
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Config) {
		c.Timeout = timeout
		c.HTTPClient.Timeout = timeout
	}
}

// WithMaxRetries sets maximum retry count
//
// Usage example:
//   client := mcp.NewClient(mcp.WithMaxRetries(5))
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Config) {
		c.MaxRetries = maxRetries
	}
}

// WithRetryWaitBase sets base retry wait duration
//
// Usage example:
//   client := mcp.NewClient(mcp.WithRetryWaitBase(3 * time.Second))
func WithRetryWaitBase(waitTime time.Duration) ClientOption {
	return func(c *Config) {
		c.RetryWaitBase = waitTime
	}
}

// ============================================================
// AI Parameter Options
// ============================================================

// WithMaxTokens sets maximum token count
//
// Usage example:
//   client := mcp.NewClient(mcp.WithMaxTokens(4000))
func WithMaxTokens(maxTokens int) ClientOption {
	return func(c *Config) {
		c.MaxTokens = maxTokens
	}
}

// WithTemperature sets temperature parameter
//
// Usage example:
//   client := mcp.NewClient(mcp.WithTemperature(0.7))
func WithTemperature(temperature float64) ClientOption {
	return func(c *Config) {
		c.Temperature = temperature
	}
}

// ============================================================
// Provider Configuration Options
// ============================================================

// WithAPIKey sets API Key
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Config) {
		c.APIKey = apiKey
	}
}

// WithBaseURL sets base URL
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Config) {
		c.BaseURL = baseURL
	}
}

// WithModel sets model name
func WithModel(model string) ClientOption {
	return func(c *Config) {
		c.Model = model
	}
}

// WithProvider sets provider
func WithProvider(provider string) ClientOption {
	return func(c *Config) {
		c.Provider = provider
	}
}

// WithUseFullURL sets whether to use full URL
func WithUseFullURL(useFullURL bool) ClientOption {
	return func(c *Config) {
		c.UseFullURL = useFullURL
	}
}

// ============================================================
// Combined Options (Convenience Methods)
// ============================================================

// WithDeepSeekConfig sets DeepSeek configuration
//
// Usage example:
//   client := mcp.NewClient(mcp.WithDeepSeekConfig("sk-xxx"))
func WithDeepSeekConfig(apiKey string) ClientOption {
	return func(c *Config) {
		c.Provider = ProviderDeepSeek
		c.APIKey = apiKey
		c.BaseURL = DefaultDeepSeekBaseURL
		c.Model = DefaultDeepSeekModel
	}
}

// WithQwenConfig sets Qwen configuration
//
// Usage example:
//   client := mcp.NewClient(mcp.WithQwenConfig("sk-xxx"))
func WithQwenConfig(apiKey string) ClientOption {
	return func(c *Config) {
		c.Provider = ProviderQwen
		c.APIKey = apiKey
		c.BaseURL = DefaultQwenBaseURL
		c.Model = DefaultQwenModel
	}
}
