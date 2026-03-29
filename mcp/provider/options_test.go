package provider

import (
	"testing"

	"nofx/mcp"
)

func TestOptionsWithDeepSeekClient(t *testing.T) {
	logger := mcp.NewNoopLogger()

	client := NewDeepSeekClientWithOptions(
		mcp.WithAPIKey("sk-deepseek-key"),
		mcp.WithLogger(logger),
		mcp.WithMaxTokens(5000),
	)

	dsClient := client.(*DeepSeekClient)

	// Verify DeepSeek default values
	if dsClient.Provider != mcp.ProviderDeepSeek {
		t.Error("Provider should be DeepSeek")
	}

	if dsClient.BaseURL != mcp.DefaultDeepSeekBaseURL {
		t.Error("BaseURL should be DeepSeek default")
	}

	if dsClient.Model != mcp.DefaultDeepSeekModel {
		t.Error("Model should be DeepSeek default")
	}

	// Verify custom options
	if dsClient.APIKey != "sk-deepseek-key" {
		t.Error("APIKey should be set from options")
	}

	if dsClient.Log != logger {
		t.Error("Log should be set from options")
	}

	if dsClient.MaxTokens != 5000 {
		t.Error("MaxTokens should be 5000")
	}
}

func TestOptionsWithQwenClient(t *testing.T) {
	logger := mcp.NewNoopLogger()

	client := NewQwenClientWithOptions(
		mcp.WithAPIKey("sk-qwen-key"),
		mcp.WithLogger(logger),
		mcp.WithMaxTokens(6000),
	)

	qwenClient := client.(*QwenClient)

	// Verify Qwen default values
	if qwenClient.Provider != mcp.ProviderQwen {
		t.Error("Provider should be Qwen")
	}

	if qwenClient.BaseURL != mcp.DefaultQwenBaseURL {
		t.Error("BaseURL should be Qwen default")
	}

	if qwenClient.Model != mcp.DefaultQwenModel {
		t.Error("Model should be Qwen default")
	}

	// Verify custom options
	if qwenClient.APIKey != "sk-qwen-key" {
		t.Error("APIKey should be set from options")
	}

	if qwenClient.Log != logger {
		t.Error("Log should be set from options")
	}

	if qwenClient.MaxTokens != 6000 {
		t.Error("MaxTokens should be 6000")
	}
}

func TestOptionsWithMiniMaxClient(t *testing.T) {
	logger := mcp.NewNoopLogger()

	client := NewMiniMaxClientWithOptions(
		mcp.WithAPIKey("sk-minimax-key"),
		mcp.WithLogger(logger),
		mcp.WithMaxTokens(8000),
	)

	minimaxClient := client.(*MiniMaxClient)

	// Verify MiniMax default values
	if minimaxClient.Provider != mcp.ProviderMiniMax {
		t.Error("Provider should be MiniMax")
	}

	if minimaxClient.BaseURL != DefaultMiniMaxBaseURL {
		t.Errorf("BaseURL should be '%s', got '%s'", DefaultMiniMaxBaseURL, minimaxClient.BaseURL)
	}

	if minimaxClient.Model != DefaultMiniMaxModel {
		t.Errorf("Model should be '%s', got '%s'", DefaultMiniMaxModel, minimaxClient.Model)
	}

	// Verify custom options
	if minimaxClient.APIKey != "sk-minimax-key" {
		t.Error("APIKey should be set from options")
	}

	if minimaxClient.Log != logger {
		t.Error("Log should be set from options")
	}

	if minimaxClient.MaxTokens != 8000 {
		t.Error("MaxTokens should be 8000")
	}
}

func TestMiniMaxClientDefaultModel(t *testing.T) {
	client := NewMiniMaxClientWithOptions()
	minimaxClient := client.(*MiniMaxClient)

	if minimaxClient.Model != "MiniMax-M2.7" {
		t.Errorf("Default model should be 'MiniMax-M2.7', got '%s'", minimaxClient.Model)
	}
}

func TestMiniMaxClientCustomModel(t *testing.T) {
	client := NewMiniMaxClientWithOptions(
		mcp.WithModel("MiniMax-M2.7-highspeed"),
	)
	minimaxClient := client.(*MiniMaxClient)

	if minimaxClient.Model != "MiniMax-M2.7-highspeed" {
		t.Errorf("Model should be 'MiniMax-M2.7-highspeed', got '%s'", minimaxClient.Model)
	}
}

func TestMiniMaxClientCustomURL(t *testing.T) {
	client := NewMiniMaxClientWithOptions(
		mcp.WithBaseURL("https://custom.minimax.io/v1"),
	)
	minimaxClient := client.(*MiniMaxClient)

	if minimaxClient.BaseURL != "https://custom.minimax.io/v1" {
		t.Errorf("BaseURL should be custom, got '%s'", minimaxClient.BaseURL)
	}
}
