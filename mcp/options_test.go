package mcp

import (
	"net/http"
	"testing"
	"time"
)

// ============================================================
// 测试基础选项
// ============================================================

func TestWithProvider(t *testing.T) {
	cfg := DefaultConfig()
	WithProvider("custom-provider")(cfg)

	if cfg.Provider != "custom-provider" {
		t.Errorf("expected 'custom-provider', got '%s'", cfg.Provider)
	}
}

func TestWithAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	WithAPIKey("sk-test-key")(cfg)

	if cfg.APIKey != "sk-test-key" {
		t.Errorf("expected 'sk-test-key', got '%s'", cfg.APIKey)
	}
}

func TestWithBaseURL(t *testing.T) {
	cfg := DefaultConfig()
	WithBaseURL("https://api.test.com")(cfg)

	if cfg.BaseURL != "https://api.test.com" {
		t.Errorf("expected 'https://api.test.com', got '%s'", cfg.BaseURL)
	}
}

func TestWithModel(t *testing.T) {
	cfg := DefaultConfig()
	WithModel("test-model")(cfg)

	if cfg.Model != "test-model" {
		t.Errorf("expected 'test-model', got '%s'", cfg.Model)
	}
}

func TestWithMaxTokens(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxTokens(4000)(cfg)

	if cfg.MaxTokens != 4000 {
		t.Errorf("expected 4000, got %d", cfg.MaxTokens)
	}
}

func TestWithTemperature(t *testing.T) {
	cfg := DefaultConfig()
	WithTemperature(0.8)(cfg)

	if cfg.Temperature != 0.8 {
		t.Errorf("expected 0.8, got %f", cfg.Temperature)
	}
}

func TestWithUseFullURL(t *testing.T) {
	cfg := DefaultConfig()
	WithUseFullURL(true)(cfg)

	if !cfg.UseFullURL {
		t.Error("UseFullURL should be true")
	}
}

func TestWithMaxRetries(t *testing.T) {
	cfg := DefaultConfig()
	WithMaxRetries(5)(cfg)

	if cfg.MaxRetries != 5 {
		t.Errorf("expected 5, got %d", cfg.MaxRetries)
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := DefaultConfig()
	WithTimeout(60 * time.Second)(cfg)

	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected 60s, got %v", cfg.Timeout)
	}
}

func TestWithLogger(t *testing.T) {
	cfg := DefaultConfig()
	mockLogger := NewMockLogger()
	WithLogger(mockLogger)(cfg)

	if cfg.Logger != mockLogger {
		t.Error("Logger should be set to mockLogger")
	}
}

func TestWithHTTPClient(t *testing.T) {
	cfg := DefaultConfig()
	customClient := &http.Client{Timeout: 30 * time.Second}
	WithHTTPClient(customClient)(cfg)

	if cfg.HTTPClient != customClient {
		t.Error("HTTPClient should be set to customClient")
	}

	if cfg.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", cfg.HTTPClient.Timeout)
	}
}

// ============================================================
// 测试预设配置选项
// ============================================================

func TestWithDeepSeekConfig(t *testing.T) {
	cfg := DefaultConfig()
	WithDeepSeekConfig("sk-deepseek-key")(cfg)

	if cfg.Provider != ProviderDeepSeek {
		t.Errorf("Provider should be '%s', got '%s'", ProviderDeepSeek, cfg.Provider)
	}

	if cfg.APIKey != "sk-deepseek-key" {
		t.Errorf("APIKey should be 'sk-deepseek-key', got '%s'", cfg.APIKey)
	}

	if cfg.BaseURL != DefaultDeepSeekBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultDeepSeekBaseURL, cfg.BaseURL)
	}

	if cfg.Model != DefaultDeepSeekModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultDeepSeekModel, cfg.Model)
	}
}

func TestWithQwenConfig(t *testing.T) {
	cfg := DefaultConfig()
	WithQwenConfig("sk-qwen-key")(cfg)

	if cfg.Provider != ProviderQwen {
		t.Errorf("Provider should be '%s', got '%s'", ProviderQwen, cfg.Provider)
	}

	if cfg.APIKey != "sk-qwen-key" {
		t.Errorf("APIKey should be 'sk-qwen-key', got '%s'", cfg.APIKey)
	}

	if cfg.BaseURL != DefaultQwenBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultQwenBaseURL, cfg.BaseURL)
	}

	if cfg.Model != DefaultQwenModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultQwenModel, cfg.Model)
	}
}

// ============================================================
// 测试选项组合
// ============================================================

func TestMultipleOptions(t *testing.T) {
	mockLogger := NewMockLogger()

	cfg := DefaultConfig()

	// 应用多个选项
	options := []ClientOption{
		WithProvider("test-provider"),
		WithAPIKey("sk-test-key"),
		WithBaseURL("https://api.test.com"),
		WithModel("test-model"),
		WithMaxTokens(4000),
		WithTemperature(0.8),
		WithLogger(mockLogger),
		WithTimeout(60 * time.Second),
	}

	for _, opt := range options {
		opt(cfg)
	}

	// 验证所有选项都被应用
	if cfg.Provider != "test-provider" {
		t.Error("Provider should be set")
	}

	if cfg.APIKey != "sk-test-key" {
		t.Error("APIKey should be set")
	}

	if cfg.BaseURL != "https://api.test.com" {
		t.Error("BaseURL should be set")
	}

	if cfg.Model != "test-model" {
		t.Error("Model should be set")
	}

	if cfg.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}

	if cfg.Temperature != 0.8 {
		t.Error("Temperature should be 0.8")
	}

	if cfg.Logger != mockLogger {
		t.Error("Logger should be mockLogger")
	}

	if cfg.Timeout != 60*time.Second {
		t.Error("Timeout should be 60s")
	}
}

func TestOptionsOverride(t *testing.T) {
	cfg := DefaultConfig()

	// 先应用 DeepSeek 配置
	WithDeepSeekConfig("sk-deepseek-key")(cfg)

	// 然后覆盖某些选项
	WithModel("custom-model")(cfg)
	WithMaxTokens(5000)(cfg)

	// 验证覆盖成功
	if cfg.Model != "custom-model" {
		t.Errorf("Model should be overridden to 'custom-model', got '%s'", cfg.Model)
	}

	if cfg.MaxTokens != 5000 {
		t.Errorf("MaxTokens should be overridden to 5000, got %d", cfg.MaxTokens)
	}

	// 验证其他 DeepSeek 配置保持不变
	if cfg.Provider != ProviderDeepSeek {
		t.Error("Provider should still be DeepSeek")
	}

	if cfg.BaseURL != DefaultDeepSeekBaseURL {
		t.Error("BaseURL should still be DeepSeek default")
	}
}

// ============================================================
// 测试与客户端集成
// ============================================================

func TestOptionsWithNewClient(t *testing.T) {
	mockLogger := NewMockLogger()

	client := NewClient(
		WithProvider("test-provider"),
		WithAPIKey("sk-test-key"),
		WithModel("test-model"),
		WithLogger(mockLogger),
		WithMaxTokens(4000),
	)

	c := client.(*Client)

	// 验证选项被正确应用到客户端
	if c.Provider != "test-provider" {
		t.Error("Provider should be set from options")
	}

	if c.APIKey != "sk-test-key" {
		t.Error("APIKey should be set from options")
	}

	if c.Model != "test-model" {
		t.Error("Model should be set from options")
	}

	if c.logger != mockLogger {
		t.Error("logger should be set from options")
	}

	if c.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}
}

func TestOptionsWithDeepSeekClient(t *testing.T) {
	mockLogger := NewMockLogger()

	client := NewDeepSeekClientWithOptions(
		WithAPIKey("sk-deepseek-key"),
		WithLogger(mockLogger),
		WithMaxTokens(5000),
	)

	dsClient := client.(*DeepSeekClient)

	// 验证 DeepSeek 默认值
	if dsClient.Provider != ProviderDeepSeek {
		t.Error("Provider should be DeepSeek")
	}

	if dsClient.BaseURL != DefaultDeepSeekBaseURL {
		t.Error("BaseURL should be DeepSeek default")
	}

	if dsClient.Model != DefaultDeepSeekModel {
		t.Error("Model should be DeepSeek default")
	}

	// 验证自定义选项
	if dsClient.APIKey != "sk-deepseek-key" {
		t.Error("APIKey should be set from options")
	}

	if dsClient.logger != mockLogger {
		t.Error("logger should be set from options")
	}

	if dsClient.MaxTokens != 5000 {
		t.Error("MaxTokens should be 5000")
	}
}

func TestOptionsWithQwenClient(t *testing.T) {
	mockLogger := NewMockLogger()

	client := NewQwenClientWithOptions(
		WithAPIKey("sk-qwen-key"),
		WithLogger(mockLogger),
		WithMaxTokens(6000),
	)

	qwenClient := client.(*QwenClient)

	// 验证 Qwen 默认值
	if qwenClient.Provider != ProviderQwen {
		t.Error("Provider should be Qwen")
	}

	if qwenClient.BaseURL != DefaultQwenBaseURL {
		t.Error("BaseURL should be Qwen default")
	}

	if qwenClient.Model != DefaultQwenModel {
		t.Error("Model should be Qwen default")
	}

	// 验证自定义选项
	if qwenClient.APIKey != "sk-qwen-key" {
		t.Error("APIKey should be set from options")
	}

	if qwenClient.logger != mockLogger {
		t.Error("logger should be set from options")
	}

	if qwenClient.MaxTokens != 6000 {
		t.Error("MaxTokens should be 6000")
	}
}
