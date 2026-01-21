package mcp

import (
	"net/http"
)

const (
	ProviderOpenGradient       = "opengradient"
	DefaultOpenGradientBaseURL = "https://api.opengradient.ai/v1"
	DefaultOpenGradientModel   = "llama-3.3-70b"
)

type OpenGradientClient struct {
	*Client
}

// NewOpenGradientClient creates OpenGradient client (backward compatible)
func NewOpenGradientClient() AIClient {
	return NewOpenGradientClientWithOptions()
}

// NewOpenGradientClientWithOptions creates OpenGradient client (supports options pattern)
//
// Usage examples:
//
//	// Basic usage
//	client := mcp.NewOpenGradientClientWithOptions()
//
//	// Custom configuration
//	client := mcp.NewOpenGradientClientWithOptions(
//	    mcp.WithAPIKey("sk-xxx"),
//	    mcp.WithModel("custom-model"),
//	)
func NewOpenGradientClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create OpenGradient preset options
	ogOpts := []ClientOption{
		WithProvider(ProviderOpenGradient),
		WithModel(DefaultOpenGradientModel),
		WithBaseURL(DefaultOpenGradientBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(ogOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create OpenGradient client
	ogClient := &OpenGradientClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to OpenGradientClient (implement dynamic dispatch)
	baseClient.hooks = ogClient

	return ogClient
}

func (c *OpenGradientClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] OpenGradient API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] OpenGradient using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] OpenGradient using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] OpenGradient using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] OpenGradient using default Model: %s", c.Model)
	}
}

func (c *OpenGradientClient) setAuthHeader(reqHeaders http.Header) {
	// TODO: Implement x402 authentication
	c.Client.setAuthHeader(reqHeaders)
}

// TODO: Override these hooks when implementing x402 protocol:
//
// func (c *OpenGradientClient) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
// }
//
// func (c *OpenGradientClient) buildUrl() string {
// }
//
// func (c *OpenGradientClient) buildRequest(url string, jsonData []byte) (*http.Request, error) {
// }
//
// func (c *OpenGradientClient) parseMCPResponse(body []byte) (string, error) {
// }
