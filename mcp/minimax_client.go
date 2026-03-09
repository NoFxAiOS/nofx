package mcp

import (
	"net/http"
)

const (
	ProviderMiniMax       = "minimax"
	DefaultMiniMaxBaseURL = "https://api.minimax.io/v1"
	DefaultMiniMaxModel   = "MiniMax-M2.5"
)

type MiniMaxClient struct {
	*Client
}

// NewMiniMaxClient creates MiniMax client (backward compatible)
func NewMiniMaxClient() AIClient {
	return NewMiniMaxClientWithOptions()
}

// NewMiniMaxClientWithOptions creates MiniMax client (supports options pattern)
//
// Usage examples:
//
//	// Basic usage
//	client := mcp.NewMiniMaxClientWithOptions()
//
//	// Custom configuration
//	client := mcp.NewMiniMaxClientWithOptions(
//	    mcp.WithAPIKey("sk-xxx"),
//	    mcp.WithLogger(customLogger),
//	    mcp.WithTimeout(60*time.Second),
//	)
func NewMiniMaxClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create MiniMax preset options
	minimaxOpts := []ClientOption{
		WithProvider(ProviderMiniMax),
		WithModel(DefaultMiniMaxModel),
		WithBaseURL(DefaultMiniMaxBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(minimaxOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create MiniMax client
	minimaxClient := &MiniMaxClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to MiniMaxClient (implement dynamic dispatch)
	baseClient.hooks = minimaxClient

	return minimaxClient
}

func (c *MiniMaxClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] MiniMax API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] MiniMax using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] MiniMax using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] MiniMax using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] MiniMax using default Model: %s", c.Model)
	}
}

// MiniMax uses standard OpenAI-compatible API with Bearer auth
func (c *MiniMaxClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}
