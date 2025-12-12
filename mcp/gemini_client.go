package mcp

import (
	"net/http"
)

const (
	ProviderGemini       = "gemini"
	DefaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	DefaultGeminiModel   = "gemini-3-pro-preview"
)

type GeminiClient struct {
	*Client
}

// NewGeminiClient creates Gemini client (backward compatible)
func NewGeminiClient() AIClient {
	return NewGeminiClientWithOptions()
}

// NewGeminiClientWithOptions creates Gemini client (supports options pattern)
func NewGeminiClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Gemini preset options
	geminiOpts := []ClientOption{
		WithProvider(ProviderGemini),
		WithModel(DefaultGeminiModel),
		WithBaseURL(DefaultGeminiBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(geminiOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Gemini client
	geminiClient := &GeminiClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to GeminiClient (implement dynamic dispatch)
	baseClient.hooks = geminiClient

	return geminiClient
}

func (c *GeminiClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] Gemini API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] Gemini using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] Gemini using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] Gemini using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] Gemini using default Model: %s", c.Model)
	}
}

// Gemini OpenAI-compatible API uses standard Bearer auth
func (c *GeminiClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}
