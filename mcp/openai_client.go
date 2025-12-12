package mcp

import (
	"net/http"
)

const (
	ProviderOpenAI       = "openai"
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
	DefaultOpenAIModel   = "gpt-5.2"
)

type OpenAIClient struct {
	*Client
}

// NewOpenAIClient creates OpenAI client (backward compatible)
func NewOpenAIClient() AIClient {
	return NewOpenAIClientWithOptions()
}

// NewOpenAIClientWithOptions creates OpenAI client (supports options pattern)
func NewOpenAIClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create OpenAI preset options
	openaiOpts := []ClientOption{
		WithProvider(ProviderOpenAI),
		WithModel(DefaultOpenAIModel),
		WithBaseURL(DefaultOpenAIBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(openaiOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create OpenAI client
	openaiClient := &OpenAIClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to OpenAIClient (implement dynamic dispatch)
	baseClient.hooks = openaiClient

	return openaiClient
}

func (c *OpenAIClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] OpenAI API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] OpenAI using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] OpenAI using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] OpenAI using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] OpenAI using default Model: %s", c.Model)
	}
}

// OpenAI uses standard Bearer auth
func (c *OpenAIClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}
