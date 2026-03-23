package provider

import (
	"net/http"

	"nofx/mcp"
)

const (
	DefaultMiniMaxBaseURL = "https://api.minimax.io/v1"
	DefaultMiniMaxModel   = "MiniMax-M2.7"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderMiniMax, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewMiniMaxClientWithOptions(opts...)
	})
}

type MiniMaxClient struct {
	*mcp.Client
}

func (c *MiniMaxClient) BaseClient() *mcp.Client { return c.Client }

// NewMiniMaxClient creates MiniMax client (backward compatible)
func NewMiniMaxClient() mcp.AIClient {
	return NewMiniMaxClientWithOptions()
}

// NewMiniMaxClientWithOptions creates MiniMax client (supports options pattern)
func NewMiniMaxClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	minimaxOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderMiniMax),
		mcp.WithModel(DefaultMiniMaxModel),
		mcp.WithBaseURL(DefaultMiniMaxBaseURL),
	}

	allOpts := append(minimaxOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	minimaxClient := &MiniMaxClient{
		Client: baseClient,
	}

	baseClient.Hooks = minimaxClient
	return minimaxClient
}

func (c *MiniMaxClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.Log.Infof("🔧 [MCP] MiniMax API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.Log.Infof("🔧 [MCP] MiniMax using custom BaseURL: %s", customURL)
	} else {
		c.Log.Infof("🔧 [MCP] MiniMax using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] MiniMax using custom Model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] MiniMax using default Model: %s", c.Model)
	}
}

// MiniMax uses standard OpenAI-compatible API with Bearer auth
func (c *MiniMaxClient) SetAuthHeader(reqHeaders http.Header) {
	c.Client.SetAuthHeader(reqHeaders)
}
