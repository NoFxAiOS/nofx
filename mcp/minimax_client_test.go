package mcp

import (
	"testing"
	"time"
)

// ============================================================
// Test MiniMaxClient Creation and Configuration
// ============================================================

func TestNewMiniMaxClient_Default(t *testing.T) {
	client := NewMiniMaxClient()

	if client == nil {
		t.Fatal("client should not be nil")
	}

	// Type assertion check
	mmClient, ok := client.(*MiniMaxClient)
	if !ok {
		t.Fatal("client should be *MiniMaxClient")
	}

	// Verify default values
	if mmClient.Provider != ProviderMiniMax {
		t.Errorf("Provider should be '%s', got '%s'", ProviderMiniMax, mmClient.Provider)
	}

	if mmClient.BaseURL != DefaultMiniMaxBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultMiniMaxBaseURL, mmClient.BaseURL)
	}

	if mmClient.Model != DefaultMiniMaxModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultMiniMaxModel, mmClient.Model)
	}

	if mmClient.logger == nil {
		t.Error("logger should not be nil")
	}

	if mmClient.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewMiniMaxClientWithOptions(t *testing.T) {
	mockLogger := NewMockLogger()
	customModel := "MiniMax-M2.5-highspeed"
	customAPIKey := "sk-custom-key"

	client := NewMiniMaxClientWithOptions(
		WithLogger(mockLogger),
		WithModel(customModel),
		WithAPIKey(customAPIKey),
		WithMaxTokens(4000),
	)

	mmClient := client.(*MiniMaxClient)

	// Verify custom options are applied
	if mmClient.logger != mockLogger {
		t.Error("logger should be set from option")
	}

	if mmClient.Model != customModel {
		t.Error("Model should be set from option")
	}

	if mmClient.APIKey != customAPIKey {
		t.Error("APIKey should be set from option")
	}

	if mmClient.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}

	// Verify MiniMax default values are retained
	if mmClient.Provider != ProviderMiniMax {
		t.Errorf("Provider should still be '%s'", ProviderMiniMax)
	}

	if mmClient.BaseURL != DefaultMiniMaxBaseURL {
		t.Errorf("BaseURL should still be '%s'", DefaultMiniMaxBaseURL)
	}
}

// ============================================================
// Test SetAPIKey
// ============================================================

func TestMiniMaxClient_SetAPIKey(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewMiniMaxClientWithOptions(
		WithLogger(mockLogger),
	)

	mmClient := client.(*MiniMaxClient)

	// Test setting API Key (default URL and Model)
	mmClient.SetAPIKey("sk-test-key-12345678", "", "")

	if mmClient.APIKey != "sk-test-key-12345678" {
		t.Errorf("APIKey should be 'sk-test-key-12345678', got '%s'", mmClient.APIKey)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	if len(logs) == 0 {
		t.Error("should have logged API key setting")
	}

	// Verify BaseURL and Model remain default
	if mmClient.BaseURL != DefaultMiniMaxBaseURL {
		t.Error("BaseURL should remain default")
	}

	if mmClient.Model != DefaultMiniMaxModel {
		t.Error("Model should remain default")
	}
}

func TestMiniMaxClient_SetAPIKey_WithCustomURL(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewMiniMaxClientWithOptions(
		WithLogger(mockLogger),
	)

	mmClient := client.(*MiniMaxClient)

	customURL := "https://api.minimaxi.com/v1"
	mmClient.SetAPIKey("sk-test-key-12345678", customURL, "")

	if mmClient.BaseURL != customURL {
		t.Errorf("BaseURL should be '%s', got '%s'", customURL, mmClient.BaseURL)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomURLLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] MiniMax using custom BaseURL: %s" {
			hasCustomURLLog = true
			break
		}
	}

	if !hasCustomURLLog {
		t.Error("should have logged custom BaseURL")
	}
}

func TestMiniMaxClient_SetAPIKey_WithCustomModel(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewMiniMaxClientWithOptions(
		WithLogger(mockLogger),
	)

	mmClient := client.(*MiniMaxClient)

	customModel := "MiniMax-M2.5-highspeed"
	mmClient.SetAPIKey("sk-test-key-12345678", "", customModel)

	if mmClient.Model != customModel {
		t.Errorf("Model should be '%s', got '%s'", customModel, mmClient.Model)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomModelLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] MiniMax using custom Model: %s" {
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

func TestMiniMaxClient_CallWithMessages_Success(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("MiniMax AI response")
	mockLogger := NewMockLogger()

	client := NewMiniMaxClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("sk-test-key"),
	)

	result, err := client.CallWithMessages("system prompt", "user prompt")

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	if result != "MiniMax AI response" {
		t.Errorf("expected 'MiniMax AI response', got '%s'", result)
	}

	// Verify request
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	req := requests[0]

	// Verify URL
	expectedURL := DefaultMiniMaxBaseURL + "/chat/completions"
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

func TestMiniMaxClient_Timeout(t *testing.T) {
	client := NewMiniMaxClientWithOptions(
		WithTimeout(30 * time.Second),
	)

	mmClient := client.(*MiniMaxClient)

	if mmClient.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", mmClient.httpClient.Timeout)
	}

	// Test SetTimeout
	client.SetTimeout(60 * time.Second)

	if mmClient.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s after SetTimeout, got %v", mmClient.httpClient.Timeout)
	}
}

// ============================================================
// Test hooks Mechanism
// ============================================================

func TestMiniMaxClient_HooksIntegration(t *testing.T) {
	client := NewMiniMaxClientWithOptions()
	mmClient := client.(*MiniMaxClient)

	// Verify hooks point to mmClient itself (implements polymorphism)
	if mmClient.hooks != mmClient {
		t.Error("hooks should point to mmClient for polymorphism")
	}

	// Verify buildUrl uses MiniMax configuration
	url := mmClient.buildUrl()
	expectedURL := DefaultMiniMaxBaseURL + "/chat/completions"
	if url != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, url)
	}
}
