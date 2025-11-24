package mcp

import (
	"testing"
	"time"
)

// ============================================================
// 测试 DeepSeekClient 创建和配置
// ============================================================

func TestNewDeepSeekClient_Default(t *testing.T) {
	client := NewDeepSeekClient()

	if client == nil {
		t.Fatal("client should not be nil")
	}

	// 类型断言检查
	dsClient, ok := client.(*DeepSeekClient)
	if !ok {
		t.Fatal("client should be *DeepSeekClient")
	}

	// 验证默认值
	if dsClient.Provider != ProviderDeepSeek {
		t.Errorf("Provider should be '%s', got '%s'", ProviderDeepSeek, dsClient.Provider)
	}

	if dsClient.BaseURL != DefaultDeepSeekBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultDeepSeekBaseURL, dsClient.BaseURL)
	}

	if dsClient.Model != DefaultDeepSeekModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultDeepSeekModel, dsClient.Model)
	}

	if dsClient.logger == nil {
		t.Error("logger should not be nil")
	}

	if dsClient.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewDeepSeekClientWithOptions(t *testing.T) {
	mockLogger := NewMockLogger()
	customModel := "deepseek-v2"
	customAPIKey := "sk-custom-key"

	client := NewDeepSeekClientWithOptions(
		WithLogger(mockLogger),
		WithModel(customModel),
		WithAPIKey(customAPIKey),
		WithMaxTokens(4000),
	)

	dsClient := client.(*DeepSeekClient)

	// 验证自定义选项被应用
	if dsClient.logger != mockLogger {
		t.Error("logger should be set from option")
	}

	if dsClient.Model != customModel {
		t.Error("Model should be set from option")
	}

	if dsClient.APIKey != customAPIKey {
		t.Error("APIKey should be set from option")
	}

	if dsClient.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}

	// 验证 DeepSeek 默认值仍然保留
	if dsClient.Provider != ProviderDeepSeek {
		t.Errorf("Provider should still be '%s'", ProviderDeepSeek)
	}

	if dsClient.BaseURL != DefaultDeepSeekBaseURL {
		t.Errorf("BaseURL should still be '%s'", DefaultDeepSeekBaseURL)
	}
}

// ============================================================
// 测试 SetAPIKey
// ============================================================

func TestDeepSeekClient_SetAPIKey(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewDeepSeekClientWithOptions(
		WithLogger(mockLogger),
	)

	dsClient := client.(*DeepSeekClient)

	// 测试设置 API Key（默认 URL 和 Model）
	dsClient.SetAPIKey("sk-test-key-12345678", "", "")

	if dsClient.APIKey != "sk-test-key-12345678" {
		t.Errorf("APIKey should be 'sk-test-key-12345678', got '%s'", dsClient.APIKey)
	}

	// 验证日志记录
	logs := mockLogger.GetLogsByLevel("INFO")
	if len(logs) == 0 {
		t.Error("should have logged API key setting")
	}

	// 验证 BaseURL 和 Model 保持默认
	if dsClient.BaseURL != DefaultDeepSeekBaseURL {
		t.Error("BaseURL should remain default")
	}

	if dsClient.Model != DefaultDeepSeekModel {
		t.Error("Model should remain default")
	}
}

func TestDeepSeekClient_SetAPIKey_WithCustomURL(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewDeepSeekClientWithOptions(
		WithLogger(mockLogger),
	)

	dsClient := client.(*DeepSeekClient)

	customURL := "https://custom.api.com/v1"
	dsClient.SetAPIKey("sk-test-key-12345678", customURL, "")

	if dsClient.BaseURL != customURL {
		t.Errorf("BaseURL should be '%s', got '%s'", customURL, dsClient.BaseURL)
	}

	// 验证日志记录
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomURLLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] DeepSeek 使用自定义 BaseURL: %s" {
			hasCustomURLLog = true
			break
		}
	}

	if !hasCustomURLLog {
		t.Error("should have logged custom BaseURL")
	}
}

func TestDeepSeekClient_SetAPIKey_WithCustomModel(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewDeepSeekClientWithOptions(
		WithLogger(mockLogger),
	)

	dsClient := client.(*DeepSeekClient)

	customModel := "deepseek-v3"
	dsClient.SetAPIKey("sk-test-key-12345678", "", customModel)

	if dsClient.Model != customModel {
		t.Errorf("Model should be '%s', got '%s'", customModel, dsClient.Model)
	}

	// 验证日志记录
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomModelLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] DeepSeek 使用自定义 Model: %s" {
			hasCustomModelLog = true
			break
		}
	}

	if !hasCustomModelLog {
		t.Error("should have logged custom Model")
	}
}

// ============================================================
// 测试集成功能
// ============================================================

func TestDeepSeekClient_CallWithMessages_Success(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("DeepSeek AI response")
	mockLogger := NewMockLogger()

	client := NewDeepSeekClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	result, err := client.CallWithMessages("system prompt", "user prompt")

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	if result != "DeepSeek AI response" {
		t.Errorf("expected 'DeepSeek AI response', got '%s'", result)
	}

	// 验证请求
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	req := requests[0]

	// 验证 URL
	expectedURL := DefaultDeepSeekBaseURL + "/chat/completions"
	if req.URL.String() != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL.String())
	}

	// 验证 Authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer sk-test-key" {
		t.Errorf("expected 'Bearer sk-test-key', got '%s'", authHeader)
	}

	// 验证 Content-Type
	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type should be application/json")
	}
}

func TestDeepSeekClient_Timeout(t *testing.T) {
	client := NewDeepSeekClientWithOptions(
		WithTimeout(30 * time.Second),
	)

	dsClient := client.(*DeepSeekClient)

	if dsClient.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", dsClient.httpClient.Timeout)
	}

	// 测试 SetTimeout
	client.SetTimeout(60 * time.Second)

	if dsClient.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s after SetTimeout, got %v", dsClient.httpClient.Timeout)
	}
}

// ============================================================
// 测试 hooks 机制
// ============================================================

func TestDeepSeekClient_HooksIntegration(t *testing.T) {
	client := NewDeepSeekClientWithOptions()
	dsClient := client.(*DeepSeekClient)

	// 验证 hooks 指向 dsClient 自己（实现多态）
	if dsClient.hooks != dsClient {
		t.Error("hooks should point to dsClient for polymorphism")
	}

	// 验证 buildUrl 使用 DeepSeek 配置
	url := dsClient.buildUrl()
	expectedURL := DefaultDeepSeekBaseURL + "/chat/completions"
	if url != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, url)
	}
}
