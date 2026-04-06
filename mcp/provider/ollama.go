package provider

import (
	"fmt"
	"net/http"
	"strings"

	"nofx/mcp"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderOllama, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewOllamaClientWithOptions(opts...)
	})
}

type OllamaClient struct {
	*mcp.Client
}

func (c *OllamaClient) BaseClient() *mcp.Client { return c.Client }

// NewOllamaClient creates Ollama client (backward compatible)
func NewOllamaClient() mcp.AIClient {
	return NewOllamaClientWithOptions()
}

// NewOllamaClientWithOptions creates Ollama client (supports options pattern)
func NewOllamaClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	ollamaOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderOllama),
		mcp.WithModel(mcp.DefaultOllamaModel),
	}

	allOpts := append(ollamaOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	ollamaClient := &OllamaClient{
		Client: baseClient,
	}

	baseClient.Hooks = ollamaClient
	return ollamaClient
}

func (c *OllamaClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey // May be empty — Ollama typically needs no auth

	if customURL != "" {
		c.BaseURL = customURL
		c.Log.Infof("🔧 [MCP] Ollama using BaseURL: %s", customURL)
	} else if c.BaseURL == "" {
		c.Log.Warnf("⚠️ [MCP] Ollama requires a Base URL to be set")
	}
	if customModel != "" {
		c.Model = customModel
		c.Log.Infof("🔧 [MCP] Ollama using custom Model: %s", customModel)
	} else {
		c.Log.Infof("🔧 [MCP] Ollama using default Model: %s", c.Model)
	}
}

// SetAuthHeader skips Authorization header when no API key is set
func (c *OllamaClient) SetAuthHeader(reqHeaders http.Header) {
	if c.APIKey != "" {
		c.Client.SetAuthHeader(reqHeaders)
	}
}

// BuildUrl constructs the Ollama OpenAI-compatible endpoint URL
func (c *OllamaClient) BuildUrl() string {
	if c.UseFullURL {
		return c.BaseURL
	}
	return fmt.Sprintf("%s/v1/chat/completions", strings.TrimRight(c.BaseURL, "/"))
}
