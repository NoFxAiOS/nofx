package provider

import (
	"net/http"

	"nofx/mcp"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderNovita, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewNovitaClientWithOptions(opts...)
	})
}

type NovitaClient struct {
	*mcp.Client
}

func (c *NovitaClient) BaseClient() *mcp.Client { return c.Client }

func NewNovitaClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	novitaOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderNovita),
		mcp.WithModel(mcp.DefaultNovitaModel),
		mcp.WithBaseURL(mcp.DefaultNovitaBaseURL),
	}

	allOpts := append(novitaOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	novitaClient := &NovitaClient{
		Client: baseClient,
	}

	baseClient.Hooks = novitaClient
	return novitaClient
}

func (c *NovitaClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.Log.Infof("🔧 [MCP] Novita API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.Log.Infof("🔧 [MCP] Novita using custom BaseURL: %s", customURL)
	} else {
		c.Log.Infof("🔧 [MCP] Novita using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] Novita using custom Model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] Novita using default Model: %s", c.Model)
	}
}

func (c *NovitaClient) SetAuthHeader(reqHeaders http.Header) {
	c.Client.SetAuthHeader(reqHeaders)
}