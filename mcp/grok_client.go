package mcp

import (
	"net/http"
)

const (
	ProviderGrok       = "grok"
	DefaultGrokBaseURL = "https://api.x.ai/v1"
	DefaultGrokModel   = "grok-3-latest"
)

type GrokClient struct {
	*Client
}

// NewGrokClient creates Grok client (backward compatible)
func NewGrokClient() AIClient {
	return NewGrokClientWithOptions()
}

// NewGrokClientWithOptions creates Grok client (supports options pattern)
func NewGrokClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Grok preset options
	grokOpts := []ClientOption{
		WithProvider(ProviderGrok),
		WithModel(DefaultGrokModel),
		WithBaseURL(DefaultGrokBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(grokOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Grok client
	grokClient := &GrokClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to GrokClient (implement dynamic dispatch)
	baseClient.hooks = grokClient

	return grokClient
}

func (c *GrokClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] Grok API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] Grok using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] Grok using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] Grok using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] Grok using default Model: %s", c.Model)
	}
}

// Grok uses standard OpenAI-compatible API with Bearer auth
func (c *GrokClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}
