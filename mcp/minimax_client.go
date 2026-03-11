package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	ProviderMiniMax       = "minimax"
	DefaultMiniMaxBaseURL = "https://api.minimax.io/v1"   // Global - OpenAI format
	MiniMaxCNBaseURL      = "https://api.minimaxi.com/v1" // China - OpenAI format
	DefaultMiniMaxModel   = "MiniMax-M2.5"

	// Coding Plan specific constants
	// Ref: https://platform.minimax.io/docs/coding-plan/faq
	CodingPlanModelPrefix  = "minimax-coding-plan/"
	CodingPlanDefaultModel = "MiniMax-M2.5"

	// Anthropic Messages endpoint (for Coding Plan)
	// Note: Anthropic endpoint has different base URL
	AnthropicBaseURL          = "https://api.minimax.io/anthropic"   // Global - Anthropic format
	AnthropicCNBaseURL        = "https://api.minimaxi.com/anthropic" // China - Anthropic format
	AnthropicMessagesEndpoint = "/v1/messages"

	// Standard OpenAI Chat Completions endpoint
	ChatCompletionsEndpoint = "/chat/completions"
)

// MiniMaxClient MiniMax AI client with Coding Plan support
type MiniMaxClient struct {
	*Client
	isCodingPlan bool
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

// isCodingPlanKey detects if the API key is a Coding Plan key
// Coding Plan keys can be:
// - JWT tokens starting with "eyJ" (Global)
// - Keys starting with "sk-cp" (China mainland)
func isCodingPlanKey(apiKey string) bool {
	return strings.HasPrefix(apiKey, "eyJ") || strings.HasPrefix(apiKey, "sk-cp")
}

// SetAPIKey sets API key and auto-detects Coding Plan keys
func (c *MiniMaxClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	// Detect Coding Plan key
	c.isCodingPlan = isCodingPlanKey(apiKey)

	if len(apiKey) > 8 {
		keyPreview := apiKey[:4]
		if c.isCodingPlan {
			if strings.HasPrefix(apiKey, "eyJ") {
				keyPreview = "eyJ..."
			} else if strings.HasPrefix(apiKey, "sk-cp") {
				keyPreview = "sk-cp"
			}
		}
		c.logger.Infof("🔧 [MCP] MiniMax API Key: %s...%s", keyPreview, apiKey[len(apiKey)-4:])
	}

	// Handle BaseURL
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] MiniMax using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] MiniMax using default BaseURL: %s", c.BaseURL)
	}

	// Handle model name
	if customModel != "" {
		// Custom model provided
		if c.isCodingPlan && !strings.HasPrefix(customModel, CodingPlanModelPrefix) {
			// Add Coding Plan prefix
			customModel = CodingPlanModelPrefix + customModel
			c.logger.Infof("🔧 [MCP] MiniMax Coding Plan: auto-added prefix to model: %s", customModel)
		}
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] MiniMax using custom Model: %s", c.Model)
	} else if c.isCodingPlan {
		// Use Coding Plan default model
		c.Model = CodingPlanModelPrefix + CodingPlanDefaultModel
		c.logger.Infof("🔧 [MCP] MiniMax Coding Plan: using default model: %s", c.Model)
	} else {
		// Use standard default model
		c.logger.Infof("🔧 [MCP] MiniMax using default Model: %s", c.Model)
	}

	// Log Coding Plan specific info
	if c.isCodingPlan {
		c.logger.Infof("🔧 [MCP] MiniMax Coding Plan key detected")
		c.logger.Infof("🔧 [MCP]   - Endpoint: %s", c.BaseURL+AnthropicMessagesEndpoint)
		c.logger.Infof("🔧 [MCP]   - Prompt Caching: enabled (cache_control)")
		c.logger.Infof("🔧 [MCP]   - Supported models: MiniMax-M2.5, MiniMax-M2.1, MiniMax-M2, MiniMax-M2.5-highspeed")
		c.logger.Infof("🔧 [MCP] Note: If you get 'invalid api key' error, please verify:")
		c.logger.Infof("🔧 [MCP]   1. Your Coding Plan key is valid and not expired")
		c.logger.Infof("🔧 [MCP]   2. Key region matches API endpoint:")
		c.logger.Infof("🔧 [MCP]      - Global:  https://api.minimax.io/anthropic")
		c.logger.Infof("🔧 [MCP]      - China:   https://api.minimaxi.com/anthropic")
	}
}

// MiniMax uses standard OpenAI-compatible API with Bearer auth
func (c *MiniMaxClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

// buildUrl returns the appropriate endpoint based on key type
// Coding Plan uses Anthropic Messages endpoint, others use OpenAI Chat Completions
func (c *MiniMaxClient) buildUrl() string {
	if c.isCodingPlan {
		// Use Anthropic Messages endpoint for Coding Plan
		// Handle different base URL formats
		baseURL := c.BaseURL

		// Check if already using Anthropic format
		if strings.Contains(baseURL, "api.minimax.io/anthropic") {
			baseURL = AnthropicBaseURL
		} else if strings.Contains(baseURL, "api.minimaxi.com/anthropic") {
			baseURL = AnthropicCNBaseURL
		} else if strings.Contains(baseURL, "api.minimax.io/v1") {
			baseURL = AnthropicBaseURL
		} else if strings.Contains(baseURL, "api.minimaxi.com/v1") {
			baseURL = AnthropicCNBaseURL
		}

		return baseURL + AnthropicMessagesEndpoint
	}

	// Use standard OpenAI Chat Completions endpoint
	return c.BaseURL + ChatCompletionsEndpoint
}

// buildMCPRequestBody builds request body in Anthropic format for Coding Plan
func (c *MiniMaxClient) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	if c.isCodingPlan {
		// Anthropic Messages API format with prompt caching
		// Note: The prompt uses XML tags <reasoning> and <decision>
		jsonInstruction := "\n\nIMPORTANT: Use XML tags <reasoning> for your analysis and <decision> for JSON output. Output JSON inside <decision> tags only.\nExample:\n<reasoning>\nYour analysis here\n</reasoning>\n<decision>\n{\"symbol\": \"BTCUSDT\", \"action\": \"hold\", \"reasoning\": \"...\"}\n</decision>"
		userMessageWithInstruction := userPrompt + jsonInstruction

		messages := []map[string]any{}
		messages = append(messages, map[string]any{
			"role": "user",
			"content": []map[string]string{
				{"type": "text", "text": userMessageWithInstruction},
			},
		})

		requestBody := map[string]any{
			"model":      c.Model,
			"messages":   messages,
			"max_tokens": c.MaxTokens,
		}

		// Add system prompt with cache_control for prompt caching
		// Official MiniMax caching config:
		// - Minimum 512 tokens to trigger cache
		// - TTL: 5 minutes with auto-refresh
		// - Cache is prefix-matched (tools → system → user messages)
		if systemPrompt != "" {
			// Use array format for system to support cache_control
			systemMessages := []map[string]any{
				{
					"type":          "text",
					"text":          systemPrompt,
					"cache_control": map[string]string{"type": "ephemeral"},
				},
			}
			requestBody["system"] = systemMessages
		}

		// Add temperature if configured
		if c.config.Temperature > 0 {
			requestBody["temperature"] = c.config.Temperature
		}

		return requestBody
	}

	// Standard OpenAI format for non-Coding Plan
	return c.Client.buildMCPRequestBody(systemPrompt, userPrompt)
}

// parseMCPResponse parses response in Anthropic format for Coding Plan
// Note: MiniMax M2.5 supports thinking (chain of thought) in addition to text
func (c *MiniMaxClient) parseMCPResponse(body []byte) (string, error) {
	if c.isCodingPlan {
		// Anthropic Messages API response format
		var result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Usage struct {
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				InputTokens              int `json:"input_tokens"`
				OutputTokens             int `json:"output_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}

		// Log cache usage for monitoring
		if result.Usage.CacheCreationInputTokens > 0 {
			c.logger.Debugf("[MCP] MiniMax cache: created %d tokens", result.Usage.CacheCreationInputTokens)
		} else if result.Usage.CacheReadInputTokens > 0 {
			savedPercent := float64(result.Usage.CacheReadInputTokens) / float64(result.Usage.InputTokens+result.Usage.CacheReadInputTokens) * 100
			c.logger.Debugf("[MCP] MiniMax cache: read %d tokens (saved %.1f%%)", result.Usage.CacheReadInputTokens, savedPercent)
		}

		if len(result.Content) == 0 {
			return "", nil
		}

		// Extract text from content blocks
		// Note: MiniMax M2.5 can return both "thinking" and "text" blocks
		var responseText strings.Builder
		for _, block := range result.Content {
			if block.Type == "text" || block.Type == "thinking" {
				responseText.WriteString(block.Text)
			}
		}

		return responseText.String(), nil
	}

	// Standard OpenAI format for non-Coding Plan
	return c.Client.parseMCPResponse(body)
}
