package provider

import (
	"net/http"

	"nofx/mcp"
)

const (
	DefaultGrokBaseURL = "https://api.x.ai/v1"
	DefaultGrokModel   = "grok-3-latest"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderGrok, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewGrokClientWithOptions(opts...)
	})
}

type GrokClient struct {
	*mcp.Client
}

func (c *GrokClient) BaseClient() *mcp.Client { return c.Client }

// NewGrokClient creates Grok client (backward compatible)
func NewGrokClient() mcp.AIClient {
	return NewGrokClientWithOptions()
}

// NewGrokClientWithOptions creates Grok client (supports options pattern)
func NewGrokClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	grokOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderGrok),
		mcp.WithModel(DefaultGrokModel),
		mcp.WithBaseURL(DefaultGrokBaseURL),
	}

	allOpts := append(grokOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	grokClient := &GrokClient{
		Client: baseClient,
	}

	baseClient.Hooks = grokClient
	return grokClient
}

func (c *GrokClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.Log.Infof("🔧 [MCP] Grok API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.Log.Infof("🔧 [MCP] Grok using custom BaseURL: %s", customURL)
	} else {
		c.Log.Infof("🔧 [MCP] Grok using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] Grok using custom Model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] Grok using default Model: %s", c.Model)
	}
}

// Grok uses standard OpenAI-compatible API with Bearer auth
func (c *GrokClient) SetAuthHeader(reqHeaders http.Header) {
	c.Client.SetAuthHeader(reqHeaders)
}
