package provider

import (
	"net/http"

	"nofx/mcp"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderDeepSeek, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewDeepSeekClientWithOptions(opts...)
	})
}

type DeepSeekClient struct {
	*mcp.Client
}

func (c *DeepSeekClient) BaseClient() *mcp.Client { return c.Client }

// NewDeepSeekClient creates DeepSeek client (backward compatible)
//
// Deprecated: Recommend using NewDeepSeekClientWithOptions for better flexibility
func NewDeepSeekClient() mcp.AIClient {
	return NewDeepSeekClientWithOptions()
}

// NewDeepSeekClientWithOptions creates DeepSeek client (supports options pattern)
func NewDeepSeekClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	deepseekOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderDeepSeek),
		mcp.WithModel(mcp.DefaultDeepSeekModel),
		mcp.WithBaseURL(mcp.DefaultDeepSeekBaseURL),
	}

	allOpts := append(deepseekOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	dsClient := &DeepSeekClient{
		Client: baseClient,
	}

	baseClient.Hooks = dsClient
	return dsClient
}

func (dsClient *DeepSeekClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	dsClient.APIKey = apiKey

	if len(apiKey) > 8 {
		dsClient.Log.Infof("🔧 [MCP] DeepSeek API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		dsClient.BaseURL = customURL
		dsClient.Log.Infof("🔧 [MCP] DeepSeek using custom BaseURL: %s", customURL)
	} else {
		dsClient.Log.Infof("🔧 [MCP] DeepSeek using default BaseURL: %s", dsClient.BaseURL)
	}
	if customModel != "" {
		dsClient.Model = customModel
		dsClient.Log.Infof("🔧 [MCP] DeepSeek using custom Model: %s", customModel)
	} else {
		dsClient.Log.Infof("🔧 [MCP] DeepSeek using default Model: %s", dsClient.Model)
	}
}

func (dsClient *DeepSeekClient) SetAuthHeader(reqHeaders http.Header) {
	dsClient.Client.SetAuthHeader(reqHeaders)
}
