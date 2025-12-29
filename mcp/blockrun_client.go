package mcp

import (
	"net/http"
)

const (
	ProviderBlockRun       = "blockrun"
	DefaultBlockRunBaseURL = "https://api.blockrun.ai/v1"
	DefaultBlockRunModel   = "openai/gpt-4o"
)

// BlockRun supported models
var BlockRunModels = map[string]string{
	// OpenAI models
	"gpt-5":            "openai/gpt-5",
	"gpt-4o":           "openai/gpt-4o",
	"gpt-4o-mini":      "openai/gpt-4o-mini",
	"gpt-4-turbo":      "openai/gpt-4-turbo",
	"gpt-3.5-turbo":    "openai/gpt-3.5-turbo",
	// Anthropic models
	"claude-3-5-sonnet": "anthropic/claude-3-5-sonnet",
	"claude-3-5-haiku":  "anthropic/claude-3-5-haiku",
	"claude-3-opus":     "anthropic/claude-3-opus",
	// Google models
	"gemini-2.0-flash": "google/gemini-2.0-flash",
	"gemini-1.5-pro":   "google/gemini-1.5-pro",
	"gemini-1.5-flash": "google/gemini-1.5-flash",
}

// BlockRunClient is a client for BlockRun AI gateway
// BlockRun enables AI agents to pay for LLM calls with USDC micropayments
// via the x402 protocol on Base. No API keys required - agents pay directly
// with their wallets.
//
// Learn more: https://blockrun.ai
type BlockRunClient struct {
	*Client
}

// NewBlockRunClient creates BlockRun client (backward compatible)
func NewBlockRunClient() AIClient {
	return NewBlockRunClientWithOptions()
}

// NewBlockRunClientWithOptions creates BlockRun client (supports options pattern)
//
// BlockRun is an x402-enabled AI gateway that allows agents to pay for
// LLM inference with USDC micropayments on Base. Features:
//   - 31+ AI models (GPT-5, GPT-4o, Claude, Gemini, etc.)
//   - OpenAI-compatible API (drop-in replacement)
//   - x402 wallet-based authentication (no API keys needed)
//   - 0% markup during beta
//
// Usage example:
//
//	client := mcp.NewBlockRunClientWithOptions(
//	    mcp.WithBlockRunConfig("x402-wallet-auth"),
//	    mcp.WithModel("openai/gpt-4o"),
//	)
func NewBlockRunClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create BlockRun preset options
	blockrunOpts := []ClientOption{
		WithProvider(ProviderBlockRun),
		WithModel(DefaultBlockRunModel),
		WithBaseURL(DefaultBlockRunBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(blockrunOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create BlockRun client
	blockrunClient := &BlockRunClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to BlockRunClient (implement dynamic dispatch)
	baseClient.hooks = blockrunClient

	return blockrunClient
}

// SetAPIKey sets API key and optional custom URL/model
// For BlockRun, the apiKey can be "x402-wallet-auth" for wallet-based auth
func (c *BlockRunClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("[MCP] BlockRun API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	} else {
		c.logger.Infof("[MCP] BlockRun using x402 wallet auth")
	}

	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("[MCP] BlockRun using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("[MCP] BlockRun using default BaseURL: %s", c.BaseURL)
	}

	if customModel != "" {
		// Convert short model name to BlockRun format if needed
		if fullModel, ok := BlockRunModels[customModel]; ok {
			c.Model = fullModel
		} else {
			c.Model = customModel
		}
		c.logger.Infof("[MCP] BlockRun using model: %s", c.Model)
	} else {
		c.logger.Infof("[MCP] BlockRun using default Model: %s", c.Model)
	}
}

// setAuthHeader sets BlockRun authentication header
// BlockRun uses standard Bearer auth, compatible with x402 wallet-based auth
func (c *BlockRunClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

// GetBlockRunModelName converts short model name to BlockRun format
func GetBlockRunModelName(model string) string {
	if fullModel, ok := BlockRunModels[model]; ok {
		return fullModel
	}
	return model
}

// ListBlockRunModels returns all available BlockRun models
func ListBlockRunModels() map[string]string {
	return BlockRunModels
}
