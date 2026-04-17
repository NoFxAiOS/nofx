package provider

import (
	"net/http"

	"nofx/mcp"
)

const (
	DefaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	DefaultGeminiModel   = "gemini-3.1-pro"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderGemini, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewGeminiClientWithOptions(opts...)
	})
}

type GeminiClient struct {
	*mcp.Client
}

func (c *GeminiClient) BaseClient() *mcp.Client { return c.Client }

// NewGeminiClient creates Gemini client (backward compatible)
func NewGeminiClient() mcp.AIClient {
	return NewGeminiClientWithOptions()
}

// NewGeminiClientWithOptions creates Gemini client (supports options pattern)
func NewGeminiClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	geminiOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderGemini),
		mcp.WithModel(DefaultGeminiModel),
		mcp.WithBaseURL(DefaultGeminiBaseURL),
	}

	allOpts := append(geminiOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	geminiClient := &GeminiClient{
		Client: baseClient,
	}

	baseClient.Hooks = geminiClient
	return geminiClient
}

func (c *GeminiClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.Log.Infof("🔧 [MCP] Gemini API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.Log.Infof("🔧 [MCP] Gemini using custom BaseURL: %s", customURL)
	} else {
		c.Log.Infof("🔧 [MCP] Gemini using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] Gemini using custom Model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] Gemini using default Model: %s", c.Model)
	}
}

// Gemini OpenAI-compatible API uses standard Bearer auth
func (c *GeminiClient) SetAuthHeader(reqHeaders http.Header) {
	c.Client.SetAuthHeader(reqHeaders)
}
