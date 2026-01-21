package mcp

import (
	"testing"
	"time"
)

// ============================================================
// Test OpenGradientClient Creation and Configuration
// ============================================================

func TestNewOpenGradientClient_Default(t *testing.T) {
	client := NewOpenGradientClient()

	if client == nil {
		t.Fatal("client should not be nil")
	}

	// Type assertion check
	ogClient, ok := client.(*OpenGradientClient)
	if !ok {
		t.Fatal("client should be *OpenGradientClient")
	}

	// Verify default values
	if ogClient.Provider != ProviderOpenGradient {
		t.Errorf("Provider should be '%s', got '%s'", ProviderOpenGradient, ogClient.Provider)
	}

	if ogClient.BaseURL != DefaultOpenGradientBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultOpenGradientBaseURL, ogClient.BaseURL)
	}

	if ogClient.Model != DefaultOpenGradientModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultOpenGradientModel, ogClient.Model)
	}

	if ogClient.logger == nil {
		t.Error("logger should not be nil")
	}

	if ogClient.httpClient == nil {
		t.Error("httpClient should not be nil")
	}

	// x402 should not be initialized without private key
	if ogClient.x402Wrapped {
		t.Error("x402 should not be wrapped without private key")
	}
}

func TestNewOpenGradientClientWithOptions(t *testing.T) {
	mockLogger := NewMockLogger()
	customModel := "llama-3.1-8b"

	client := NewOpenGradientClientWithOptions(
		WithLogger(mockLogger),
		WithModel(customModel),
		WithMaxTokens(4000),
	)

	ogClient := client.(*OpenGradientClient)

	// Verify custom options are applied
	if ogClient.logger != mockLogger {
		t.Error("logger should be set from option")
	}

	if ogClient.Model != customModel {
		t.Errorf("Model should be '%s', got '%s'", customModel, ogClient.Model)
	}

	if ogClient.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}

	// Verify OpenGradient default values are retained
	if ogClient.Provider != ProviderOpenGradient {
		t.Errorf("Provider should still be '%s'", ProviderOpenGradient)
	}

	if ogClient.BaseURL != DefaultOpenGradientBaseURL {
		t.Errorf("BaseURL should still be '%s'", DefaultOpenGradientBaseURL)
	}
}

func TestNewOpenGradientClientWithOptions_CustomBaseURL(t *testing.T) {
	customURL := "https://custom.opengradient.com/v2"

	client := NewOpenGradientClientWithOptions(
		WithBaseURL(customURL),
	)

	ogClient := client.(*OpenGradientClient)

	if ogClient.BaseURL != customURL {
		t.Errorf("BaseURL should be '%s', got '%s'", customURL, ogClient.BaseURL)
	}
}

// ============================================================
// Test SetAPIKey
// ============================================================

func TestOpenGradientClient_SetAPIKey(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewOpenGradientClientWithOptions(
		WithLogger(mockLogger),
	)

	ogClient := client.(*OpenGradientClient)

	// Test setting API Key (which is used as private key for OpenGradient)
	ogClient.SetAPIKey("0x1234567890abcdef", "", "")

	if ogClient.privateKey != "0x1234567890abcdef" {
		t.Errorf("privateKey should be '0x1234567890abcdef', got '%s'", ogClient.privateKey)
	}

	// APIKey should be set to placeholder
	if ogClient.APIKey != "x402-authenticated" {
		t.Errorf("APIKey should be 'x402-authenticated', got '%s'", ogClient.APIKey)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	if len(logs) == 0 {
		t.Error("should have logged private key setting")
	}

	// Verify BaseURL and Model remain default
	if ogClient.BaseURL != DefaultOpenGradientBaseURL {
		t.Error("BaseURL should remain default")
	}

	if ogClient.Model != DefaultOpenGradientModel {
		t.Error("Model should remain default")
	}
}

func TestOpenGradientClient_SetAPIKey_WithCustomURL(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewOpenGradientClientWithOptions(
		WithLogger(mockLogger),
	)

	ogClient := client.(*OpenGradientClient)

	customURL := "https://custom.api.com/v1"
	ogClient.SetAPIKey("0x1234567890abcdef", customURL, "")

	if ogClient.BaseURL != customURL {
		t.Errorf("BaseURL should be '%s', got '%s'", customURL, ogClient.BaseURL)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomURLLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] OpenGradient using custom BaseURL: %s" {
			hasCustomURLLog = true
			break
		}
	}

	if !hasCustomURLLog {
		t.Error("should have logged custom BaseURL")
	}
}

func TestOpenGradientClient_SetAPIKey_WithCustomModel(t *testing.T) {
	mockLogger := NewMockLogger()
	client := NewOpenGradientClientWithOptions(
		WithLogger(mockLogger),
	)

	ogClient := client.(*OpenGradientClient)

	customModel := "llama-3.1-405b"
	ogClient.SetAPIKey("0x1234567890abcdef", "", customModel)

	if ogClient.Model != customModel {
		t.Errorf("Model should be '%s', got '%s'", customModel, ogClient.Model)
	}

	// Verify logging
	logs := mockLogger.GetLogsByLevel("INFO")
	hasCustomModelLog := false
	for _, log := range logs {
		if log.Format == "🔧 [MCP] OpenGradient using custom Model: %s" {
			hasCustomModelLog = true
			break
		}
	}

	if !hasCustomModelLog {
		t.Error("should have logged custom Model")
	}
}

// ============================================================
// Test Timeout
// ============================================================

func TestOpenGradientClient_Timeout(t *testing.T) {
	client := NewOpenGradientClientWithOptions(
		WithTimeout(30 * time.Second),
	)

	ogClient := client.(*OpenGradientClient)

	if ogClient.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", ogClient.httpClient.Timeout)
	}

	// Test SetTimeout
	client.SetTimeout(60 * time.Second)

	if ogClient.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s after SetTimeout, got %v", ogClient.httpClient.Timeout)
	}
}

// ============================================================
// Test hooks Mechanism
// ============================================================

func TestOpenGradientClient_HooksIntegration(t *testing.T) {
	client := NewOpenGradientClientWithOptions()
	ogClient := client.(*OpenGradientClient)

	// Verify hooks point to ogClient itself (implements polymorphism)
	if ogClient.hooks != ogClient {
		t.Error("hooks should point to ogClient for polymorphism")
	}

	// Verify buildUrl uses OpenGradient configuration
	url := ogClient.buildUrl()
	expectedURL := DefaultOpenGradientBaseURL + "/chat/completions"
	if url != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, url)
	}
}

// ============================================================
// Test setAuthHeader (sets placeholder for x402)
// ============================================================

func TestOpenGradientClient_SetAuthHeader(t *testing.T) {
	client := NewOpenGradientClientWithOptions()
	ogClient := client.(*OpenGradientClient)

	// Create mock headers
	headers := make(map[string][]string)

	// Call setAuthHeader
	ogClient.setAuthHeader(headers)

	// Verify Authorization header is set to "Bearer x402"
	// OpenGradient requires an Authorization header (actual auth is handled by x402 payment)
	if auth := headers["Authorization"]; len(auth) == 0 || auth[0] != "Bearer x402" {
		t.Errorf("Authorization header should be 'Bearer x402', got '%v'", headers["Authorization"])
	}
}

// ============================================================
// Test Constants
// ============================================================

func TestOpenGradientConstants(t *testing.T) {
	if ProviderOpenGradient != "opengradient" {
		t.Errorf("ProviderOpenGradient should be 'opengradient', got '%s'", ProviderOpenGradient)
	}

	if DefaultOpenGradientBaseURL != "https://llmogevm.opengradient.ai/v1" {
		t.Errorf("DefaultOpenGradientBaseURL should be 'https://llmogevm.opengradient.ai/v1', got '%s'", DefaultOpenGradientBaseURL)
	}

	if DefaultOpenGradientModel != "gemini-2.5-flash" {
		t.Errorf("DefaultOpenGradientModel should be 'gemini-2.5-flash', got '%s'", DefaultOpenGradientModel)
	}
}

// ============================================================
// Test WithOpenGradientPrivateKey Option
// ============================================================

func TestWithOpenGradientPrivateKey(t *testing.T) {
	// Note: We can't fully test x402 initialization without a valid private key
	// This test verifies the option is properly passed to config
	mockLogger := NewMockLogger()

	client := NewOpenGradientClientWithOptions(
		WithLogger(mockLogger),
		WithOpenGradientPrivateKey("0xinvalidkey"), // Invalid key, will fail x402 init
	)

	ogClient := client.(*OpenGradientClient)

	// Private key should be set
	if ogClient.privateKey != "0xinvalidkey" {
		t.Errorf("privateKey should be '0xinvalidkey', got '%s'", ogClient.privateKey)
	}

	// x402 init should have failed (logged warning)
	warnLogs := mockLogger.GetLogsByLevel("WARN")
	hasX402Warning := false
	for _, log := range warnLogs {
		if log.Format == "⚠️ [MCP] Failed to initialize x402: %v" {
			hasX402Warning = true
			break
		}
	}

	if !hasX402Warning {
		t.Error("should have logged x402 initialization warning for invalid key")
	}
}
