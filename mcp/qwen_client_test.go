package mcp

import (
	"testing"
	"time"
)

// ============================================================
// Test QwenClient Creation and Configuration
// ============================================================

func TestNewQwenClient_Default(t *testing.T) {
	client := NewQwenClient()

	if client == nil {
		t.Fatal("client should not be nil")
	}

	// Type assertion check
	qwenClient, ok := client.(*QwenClient)
	if !ok {
		t.Fatal("client should be *QwenClient")
	}

	// Verify default values
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

	// Verify custom options are applied
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

	// Verify Qwen default values are retained
	if qwenClient.Provider != ProviderQwen {
		t.Errorf("Provider should still be '%s'", ProviderQwen)
	}

	if qwenClient.BaseURL != DefaultQwenBaseURL {
		t.Errorf("BaseURL should still be '%s'", DefaultQwenBaseURL)
	}
}

// ============================================================
// Test SetAPIKey
// ============================================================

func TestQwenClient_SetAPIKey(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewQwenClientWithOptions(
		WithLogger(mockLogger),
	)

	qwenClient := client.(*QwenClient)

	// Test setting API Key (default URL and Model)
	qwenClient.SetAPIKey("sk-test-key-12345678", "", "")

	if qwenClient.APIKey != "sk-test-key-12345678" {
		t.Errorf("APIKey should be 'sk-test-key-12345678', got '%s'", qwenClient.APIKey)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	if len(logs) == 0 {
		t.Error("should have logged API key setting")
	}

	// Verify BaseURL and Model remain default
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

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomURLLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] Qwen using custom BaseURL: %s" {
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

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomModelLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] Qwen using custom Model: %s" {
			hasCustomModelLog = true
			break
		}
	}

	if !hasCustomModelLog {
		t.Error("should have logged custom Model")
	}
}

// ============================================================
// Test Integration Features
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

	// Verify request
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	req := requests[0]

	// Verify URL
	expectedURL := DefaultQwenBaseURL + "/chat/completions"
	if req.URL.String() != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL.String())
	}

	// Verify Authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer sk-test-key" {
		t.Errorf("expected 'Bearer sk-test-key', got '%s'", authHeader)
	}

	// Verify Content-Type
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

	// Test SetTimeout
	client.SetTimeout(60 * time.Second)

	if qwenClient.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s after SetTimeout, got %v", qwenClient.httpClient.Timeout)
	}
}

// ============================================================
// Test hooks Mechanism
// ============================================================

func TestQwenClient_HooksIntegration(t *testing.T) {
	client := NewQwenClientWithOptions()
	qwenClient := client.(*QwenClient)

	// Verify hooks point to qwenClient itself (implements polymorphism)
	if qwenClient.hooks != qwenClient {
		t.Error("hooks should point to qwenClient for polymorphism")
	}

	// Verify buildUrl uses Qwen configuration
	url := qwenClient.buildUrl()
	expectedURL := DefaultQwenBaseURL + "/chat/completions"
	if url != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, url)
	}
}
