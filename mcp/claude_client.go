package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	ProviderClaude       = "claude"
	DefaultClaudeBaseURL = "https://api.anthropic.com/v1"
	DefaultClaudeModel   = "claude-opus-4-5-20251101"
)

type ClaudeClient struct {
	*Client
}

// NewClaudeClient creates Claude client (backward compatible)
func NewClaudeClient() AIClient {
	return NewClaudeClientWithOptions()
}

// NewClaudeClientWithOptions creates Claude client (supports options pattern)
func NewClaudeClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Claude preset options
	claudeOpts := []ClientOption{
		WithProvider(ProviderClaude),
		WithModel(DefaultClaudeModel),
		WithBaseURL(DefaultClaudeBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(claudeOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Claude client
	claudeClient := &ClaudeClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to ClaudeClient (implement dynamic dispatch)
	baseClient.hooks = claudeClient

	return claudeClient
}

func (c *ClaudeClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] Claude API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] Claude using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] Claude using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] Claude using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] Claude using default Model: %s", c.Model)
	}
}

// setAuthHeader Claude uses x-api-key header instead of Authorization Bearer
func (c *ClaudeClient) setAuthHeader(reqHeaders http.Header) {
	reqHeaders.Set("x-api-key", c.APIKey)
	reqHeaders.Set("anthropic-version", "2023-06-01")
}

// buildUrl Claude uses /messages endpoint
func (c *ClaudeClient) buildUrl() string {
	return fmt.Sprintf("%s/messages", c.BaseURL)
}

// buildMCPRequestBody Claude has different request format
func (c *ClaudeClient) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	requestBody := map[string]any{
		"model":      c.Model,
		"max_tokens": c.MaxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}

	return requestBody
}

// parseMCPResponse Claude has different response format
func (c *ClaudeClient) parseMCPResponse(body []byte) (string, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse Claude response: %w, body: %s", err, string(body))
	}

	if response.Error != nil {
		return "", fmt.Errorf("Claude API error: %s - %s", response.Error.Type, response.Error.Message)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("Claude returned empty content, body: %s", string(body))
	}

	// Report token usage if callback is set
	totalTokens := response.Usage.InputTokens + response.Usage.OutputTokens
	if TokenUsageCallback != nil && totalTokens > 0 {
		TokenUsageCallback(TokenUsage{
			Provider:         c.Provider,
			Model:            c.Model,
			PromptTokens:     response.Usage.InputTokens,
			CompletionTokens: response.Usage.OutputTokens,
			TotalTokens:      totalTokens,
		})
	}

	// Find text content
	for _, content := range response.Content {
		if content.Type == "text" {
			return content.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in Claude response")
}
