package mcp

import (
	"testing"
	"time"
)

// ============================================================
// 测试 QwenClient 创建和配置
// ============================================================

func TestNewQwenClient_Default(t *testing.T) {
	client := NewQwenClient()

	if client == nil {
		t.Fatal("client should not be nil")
	}

	// 类型断言检查
	qwenClient, ok := client.(*QwenClient)
	if !ok {
		t.Fatal("client should be *QwenClient")
	}

	// 验证默认值
	if qwenClient.Provider != ProviderQwen {
		t.Errorf("Provider should be '%s', got '%s'", ProviderQwen, qwenClient.Provider)
	}

	if qwenClient.BaseURL != DefaultQwenBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultQwenBaseURL, qwenClient.BaseURL)
	}

	if qwenClient.Model != DefaultQwenModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultQwenModel, qwenClient.Model)
	}

	if qwenClient.logger == nil {
		t.Error("logger should not be nil")
	}

	if qwenClient.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewQwenClientWithOptions(t *testing.T) {
	mockLogger := NewMockLogger()
	customModel := "qwen-plus"
	customAPIKey := "sk-custom-qwen-key"

	client := NewQwenClientWithOptions(
		WithLogger(mockLogger),
		WithModel(customModel),
		WithAPIKey(customAPIKey),
		WithMaxTokens(4000),
	)

	qwenClient := client.(*QwenClient)

	// 验证自定义选项被应用
	if qwenClient.logger != mockLogger {
		t.Error("logger should be set from option")
	}

	if qwenClient.Model != customModel {
		t.Error("Model should be set from option")
	}

	if qwenClient.APIKey != customAPIKey {
		t.Error("APIKey should be set from option")
	}

	if qwenClient.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}

	// 验证 Qwen 默认值仍然保留
	if qwenClient.Provider != ProviderQwen {
		t.Errorf("Provider should still be '%s'", ProviderQwen)
	}

	if qwenClient.BaseURL != DefaultQwenBaseURL {
		t.Errorf("BaseURL should still be '%s'", DefaultQwenBaseURL)
	}
}

// ============================================================
// 测试 SetAPIKey
// ============================================================

func TestQwenClient_SetAPIKey(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewQwenClientWithOptions(
		WithLogger(mockLogger),
	)

	qwenClient := client.(*QwenClient)

	// 测试设置 API Key（默认 URL 和 Model）
	qwenClient.SetAPIKey("sk-test-key-12345678", "", "")

	if qwenClient.APIKey != "sk-test-key-12345678" {
		t.Errorf("APIKey should be 'sk-test-key-12345678', got '%s'", qwenClient.APIKey)
	}

	// 验证日志记录
	logs := mockLogger.GetLogsByLevel("INFO")
	if len(logs) == 0 {
		t.Error("should have logged API key setting")
	}

	// 验证 BaseURL 和 Model 保持默认
	if qwenClient.BaseURL != DefaultQwenBaseURL {
		t.Error("BaseURL should remain default")
	}

	if qwenClient.Model != DefaultQwenModel {
		t.Error("Model should remain default")
	}
}

func TestQwenClient_SetAPIKey_WithCustomURL(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewQwenClientWithOptions(
		WithLogger(mockLogger),
	)

	qwenClient := client.(*QwenClient)

	customURL := "https://custom.qwen.api.com/v1"
	qwenClient.SetAPIKey("sk-test-key-12345678", customURL, "")

	if qwenClient.BaseURL != customURL {
		t.Errorf("BaseURL should be '%s', got '%s'", customURL, qwenClient.BaseURL)
	}

	// 验证日志记录
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomURLLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] Qwen 使用自定义 BaseURL: %s" {
			hasCustomURLLog = true
			break
		}
	}

	if !hasCustomURLLog {
		t.Error("should have logged custom BaseURL")
	}
}

func TestQwenClient_SetAPIKey_WithCustomModel(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewQwenClientWithOptions(
		WithLogger(mockLogger),
	)

	qwenClient := client.(*QwenClient)

	customModel := "qwen-turbo"
	qwenClient.SetAPIKey("sk-test-key-12345678", "", customModel)

	if qwenClient.Model != customModel {
		t.Errorf("Model should be '%s', got '%s'", customModel, qwenClient.Model)
	}

	// 验证日志记录
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomModelLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] Qwen 使用自定义 Model: %s" {
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

func TestQwenClient_CallWithMessages_Success(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("Qwen AI response")
	mockLogger := NewMockLogger()

	client := NewQwenClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	result, err := client.CallWithMessages("system prompt", "user prompt")

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	if result != "Qwen AI response" {
		t.Errorf("expected 'Qwen AI response', got '%s'", result)
	}

	// 验证请求
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	req := requests[0]

	// 验证 URL
	expectedURL := DefaultQwenBaseURL + "/chat/completions"
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

func TestQwenClient_Timeout(t *testing.T) {
	client := NewQwenClientWithOptions(
		WithTimeout(30 * time.Second),
	)

	qwenClient := client.(*QwenClient)

	if qwenClient.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", qwenClient.httpClient.Timeout)
	}

	// 测试 SetTimeout
	client.SetTimeout(60 * time.Second)

	if qwenClient.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s after SetTimeout, got %v", qwenClient.httpClient.Timeout)
	}
}

// ============================================================
// 测试 hooks 机制
// ============================================================

func TestQwenClient_HooksIntegration(t *testing.T) {
	client := NewQwenClientWithOptions()
	qwenClient := client.(*QwenClient)

	// 验证 hooks 指向 qwenClient 自己（实现多态）
	if qwenClient.hooks != qwenClient {
		t.Error("hooks should point to qwenClient for polymorphism")
	}

	// 验证 buildUrl 使用 Qwen 配置
	url := qwenClient.buildUrl()
	expectedURL := DefaultQwenBaseURL + "/chat/completions"
	if url != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, url)
	}
}
