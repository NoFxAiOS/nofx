//go:build integration

package mcp

import (
	"os"
	"strings"
	"testing"
)

// Integration tests for OpenGradient client with x402 payments.
// These tests require a real EVM private key with funds.
//
// Run with:
//   OPENGRADIENT_PRIVATE_KEY="0x..." go test ./mcp/... -v -tags=integration -run TestOpenGradientIntegration

func getTestPrivateKey(t *testing.T) string {
	privateKey := os.Getenv("OPENGRADIENT_PRIVATE_KEY")
	if privateKey == "" {
		t.Skip("OPENGRADIENT_PRIVATE_KEY not set, skipping integration test")
	}
	return privateKey
}

// ============================================================
// Basic Integration Tests
// ============================================================

func TestOpenGradientIntegration_SimpleCall(t *testing.T) {
	privateKey := getTestPrivateKey(t)

	client := NewOpenGradientClientWithOptions(
		WithOpenGradientPrivateKey(privateKey),
	)

	result, err := client.CallWithMessages(
		"You are a helpful assistant. Respond concisely.",
		"What is 2 + 2? Reply with just the number.",
	)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	if result == "" {
		t.Fatal("Response should not be empty")
	}

	t.Logf("Response: %s", result)

	// Basic sanity check - response should contain "4"
	if !strings.Contains(result, "4") {
		t.Logf("Warning: Expected response to contain '4', got: %s", result)
	}
}

func TestOpenGradientIntegration_WithCustomModel(t *testing.T) {
	privateKey := getTestPrivateKey(t)

	client := NewOpenGradientClientWithOptions(
		WithOpenGradientPrivateKey(privateKey),
		WithModel("openai/gpt-4.1"), // Use default model explicitly
	)

	result, err := client.CallWithMessages(
		"You are a helpful assistant.",
		"Say hello in one word.",
	)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	if result == "" {
		t.Fatal("Response should not be empty")
	}

	t.Logf("Response: %s", result)
}

func TestOpenGradientIntegration_SetAPIKeyMethod(t *testing.T) {
	privateKey := getTestPrivateKey(t)

	// Test using SetAPIKey method (backward compatible approach)
	client := NewOpenGradientClient()
	client.SetAPIKey(privateKey, "", "")

	result, err := client.CallWithMessages(
		"You are a helpful assistant.",
		"What color is the sky? Reply in one word.",
	)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	if result == "" {
		t.Fatal("Response should not be empty")
	}

	t.Logf("Response: %s", result)
}

// ============================================================
// Builder Pattern Tests
// ============================================================

func TestOpenGradientIntegration_CallWithRequest(t *testing.T) {
	privateKey := getTestPrivateKey(t)

	client := NewOpenGradientClientWithOptions(
		WithOpenGradientPrivateKey(privateKey),
	)

	req := NewRequestBuilder().
		WithSystemPrompt("You are a helpful coding assistant.").
		WithUserPrompt("Write a one-line Python hello world program.").
		WithMaxTokens(100).
		WithTemperature(0.3).
		MustBuild()

	result, err := client.CallWithRequest(req)

	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	if result == "" {
		t.Fatal("Response should not be empty")
	}

	t.Logf("Response: %s", result)

	// Should contain print or hello
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "print") && !strings.Contains(lower, "hello") {
		t.Logf("Warning: Expected Python hello world, got: %s", result)
	}
}

// ============================================================
// Error Handling Tests
// ============================================================

func TestOpenGradientIntegration_InvalidPrivateKey(t *testing.T) {
	// This test verifies behavior with an invalid private key
	client := NewOpenGradientClientWithOptions(
		WithOpenGradientPrivateKey("invalid-key"),
	)

	_, err := client.CallWithMessages(
		"You are a helpful assistant.",
		"Hello",
	)

	// Should fail - either during x402 init or API call
	if err == nil {
		t.Log("Note: Call succeeded with invalid key (x402 may not have initialized)")
	} else {
		t.Logf("Expected error with invalid key: %v", err)
	}
}

// ============================================================
// Longer Conversation Test
// ============================================================

func TestOpenGradientIntegration_MultiTurn(t *testing.T) {
	privateKey := getTestPrivateKey(t)

	client := NewOpenGradientClientWithOptions(
		WithOpenGradientPrivateKey(privateKey),
		WithMaxTokens(500),
	)

	// First turn
	result1, err := client.CallWithMessages(
		"You are a helpful assistant. Remember the context of our conversation.",
		"My favorite color is blue. What did I just tell you?",
	)

	if err != nil {
		t.Fatalf("First API call failed: %v", err)
	}

	t.Logf("Response 1: %s", result1)

	if !strings.Contains(strings.ToLower(result1), "blue") {
		t.Logf("Warning: Expected response to mention 'blue'")
	}

	// Second turn (note: this is a new call, not true multi-turn without message history)
	result2, err := client.CallWithMessages(
		"You are a helpful assistant.",
		"Name three things that are typically blue.",
	)

	if err != nil {
		t.Fatalf("Second API call failed: %v", err)
	}

	t.Logf("Response 2: %s", result2)
}
