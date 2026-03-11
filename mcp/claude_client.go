// Package mcp — ClaudeClient implements the Anthropic Messages API.
//
// Wire-format differences from the OpenAI-compatible base Client:
//
//	┌─────────────────────┬───────────────────────────┬─────────────────────────────────┐
//	│ Concept             │ OpenAI format              │ Anthropic format                │
//	├─────────────────────┼───────────────────────────┼─────────────────────────────────┤
//	│ Endpoint            │ /v1/chat/completions       │ /v1/messages                    │
//	│ Auth header         │ Authorization: Bearer xxx  │ x-api-key: xxx                  │
//	│ System prompt       │ messages[0] role=system    │ top-level "system" field        │
//	│ Tool definition     │ type=function + parameters │ name + description + input_schema│
//	│ Tool choice         │ "auto" (string)            │ {"type":"auto"} (object)        │
//	│ Assistant tool call │ tool_calls array           │ content[{type:tool_use,...}]    │
//	│ Tool result         │ role=tool + tool_call_id   │ role=user content[tool_result]  │
//	│ Max tokens          │ max_tokens                 │ max_tokens (same)               │
//	└─────────────────────┴───────────────────────────┴─────────────────────────────────┘
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	ProviderClaude       = "claude"
	DefaultClaudeBaseURL = "https://api.anthropic.com/v1"
	DefaultClaudeModel   = "claude-opus-4-6"
)

// ClaudeClient wraps the base Client and overrides the methods that differ
// for the Anthropic Messages API.  All other behaviour (retry, timeout,
// logging) is inherited unchanged.
type ClaudeClient struct {
	*Client
}

// NewClaudeClient creates a ClaudeClient with default settings.
func NewClaudeClient() AIClient {
	return NewClaudeClientWithOptions()
}

// NewClaudeClientWithOptions creates a ClaudeClient with optional overrides.
func NewClaudeClientWithOptions(opts ...ClientOption) AIClient {
	baseClient := NewClient(append([]ClientOption{
		WithProvider(ProviderClaude),
		WithModel(DefaultClaudeModel),
		WithBaseURL(DefaultClaudeBaseURL),
	}, opts...)...).(*Client)

	c := &ClaudeClient{Client: baseClient}
	baseClient.hooks = c // wire dynamic dispatch to ClaudeClient
	return c
}

// ── Hook overrides ────────────────────────────────────────────────────────────

// SetAPIKey stores credentials and optional custom endpoint / model.
func (c *ClaudeClient) SetAPIKey(apiKey, customURL, customModel string) {
	c.APIKey = apiKey
	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] Claude API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] Claude BaseURL: %s", customURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] Claude Model: %s", customModel)
	}
}

// setAuthHeader uses x-api-key instead of Authorization: Bearer.
func (c *ClaudeClient) setAuthHeader(h http.Header) {
	h.Set("x-api-key", c.APIKey)
	h.Set("anthropic-version", "2023-06-01")
}

// buildUrl targets /messages instead of /chat/completions.
func (c *ClaudeClient) buildUrl() string {
	return fmt.Sprintf("%s/messages", c.BaseURL)
}

// buildMCPRequestBody builds the Anthropic wire format for the simple
// CallWithMessages path (no tool support).
func (c *ClaudeClient) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	return map[string]any{
		"model":      c.Model,
		"max_tokens": c.MaxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
}

// buildRequestBodyFromRequest converts a *Request into the Anthropic Messages
// API wire format.  This is the key override that makes tool calling work
// correctly with Claude.
//
// Conversions applied:
//
//   - System messages are lifted to the top-level "system" field.
//   - Tool definitions: parameters → input_schema, wrapper removed.
//   - Assistant messages with ToolCalls → content[{type:tool_use,...}].
//   - Tool result messages (role=tool) → role=user with tool_result blocks.
//     Consecutive tool results are merged into a single user turn (Anthropic
//     requires strictly alternating user/assistant turns).
//   - tool_choice "auto"/"any" → {"type":"auto"/"any"} object.
func (c *ClaudeClient) buildRequestBodyFromRequest(req *Request) map[string]any {
	// ── 1. Separate system prompt from conversation messages ──────────────────
	var systemPrompt string
	var convMsgs []Message
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			convMsgs = append(convMsgs, m)
		}
	}

	// ── 2. Convert messages to Anthropic format ───────────────────────────────
	anthropicMsgs := convertMessagesToAnthropic(convMsgs)

	// ── 3. Convert tool definitions (parameters → input_schema) ──────────────
	var anthropicTools []map[string]any
	for _, t := range req.Tools {
		anthropicTools = append(anthropicTools, map[string]any{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": t.Function.Parameters,
		})
	}

	// ── 4. Assemble request body ──────────────────────────────────────────────
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": c.MaxTokens,
		"system":     systemPrompt,
		"messages":   anthropicMsgs,
	}

	if len(anthropicTools) > 0 {
		body["tools"] = anthropicTools
	}

	// tool_choice: Anthropic uses an object, not a string.
	switch req.ToolChoice {
	case "auto":
		body["tool_choice"] = map[string]any{"type": "auto"}
	case "any":
		body["tool_choice"] = map[string]any{"type": "any"}
	case "none", "":
		// omit — no tool_choice sent
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	return body
}

// convertMessagesToAnthropic translates from the OpenAI-shaped mcp.Message
// slice to Anthropic's messages array.
//
// Rules:
//  1. role=assistant + ToolCalls → role=assistant, content=[tool_use, ...]
//  2. role=tool (result)         → role=user, content=[tool_result, ...]
//     Consecutive tool-result messages are merged into one user turn so the
//     conversation always alternates user/assistant.
//  3. All other messages         → {role, content} as-is.
func convertMessagesToAnthropic(msgs []Message) []map[string]any {
	var out []map[string]any

	for i := 0; i < len(msgs); {
		msg := msgs[i]

		switch {
		// ── Assistant message carrying tool calls ─────────────────────────────
		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			var blocks []map[string]any
			for _, tc := range msg.ToolCalls {
				// Arguments are a JSON string; Claude wants a parsed object.
				var input map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = map[string]any{"_raw": tc.Function.Arguments}
				}
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": input,
				})
			}
			out = append(out, map[string]any{
				"role":    "assistant",
				"content": blocks,
			})
			i++

		// ── Tool result message(s) → single user turn ─────────────────────────
		case msg.Role == "tool":
			// Collect all consecutive tool-result messages.
			var blocks []map[string]any
			for i < len(msgs) && msgs[i].Role == "tool" {
				blocks = append(blocks, map[string]any{
					"type":        "tool_result",
					"tool_use_id": msgs[i].ToolCallID,
					"content":     msgs[i].Content,
				})
				i++
			}
			out = append(out, map[string]any{
				"role":    "user",
				"content": blocks,
			})

		// ── Regular user / assistant text message ─────────────────────────────
		default:
			out = append(out, map[string]any{
				"role":    msg.Role,
				"content": msg.Content,
			})
			i++
		}
	}

	return out
}

// ── Response parsers ──────────────────────────────────────────────────────────

// parseMCPResponse extracts the plain-text reply from an Anthropic response.
// Used by CallWithMessages / CallWithRequest (no tool support).
func (c *ClaudeClient) parseMCPResponse(body []byte) (string, error) {
	r, err := c.parseMCPResponseFull(body)
	if err != nil {
		return "", err
	}
	return r.Content, nil
}

// parseMCPResponseFull extracts both text and tool calls from an Anthropic
// response envelope.
//
// Anthropic response shape:
//
//	{
//	  "content": [
//	    {"type": "text",     "text": "..."},
//	    {"type": "tool_use", "id": "...", "name": "...", "input": {...}}
//	  ],
//	  "stop_reason": "tool_use" | "end_turn"
//	}
func (c *ClaudeClient) parseMCPResponseFull(body []byte) (*LLMResponse, error) {
	var raw struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text,omitempty"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
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

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w — body: %s", err, body)
	}
	if raw.Error != nil {
		return nil, fmt.Errorf("Anthropic API error: %s — %s", raw.Error.Type, raw.Error.Message)
	}

	total := raw.Usage.InputTokens + raw.Usage.OutputTokens
	if TokenUsageCallback != nil && total > 0 {
		TokenUsageCallback(TokenUsage{
			Provider:         c.Provider,
			Model:            c.Model,
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
			TotalTokens:      total,
		})
	}

	result := &LLMResponse{}
	for _, block := range raw.Content {
		switch block.Type {
		case "text":
			result.Content = block.Text

		case "tool_use":
			// Input is a JSON object; serialise back to a JSON string so it
			// matches the ToolCallFunction.Arguments field (always a string).
			argsJSON, err := json.Marshal(block.Input)
			if err != nil {
				argsJSON = []byte("{}")
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}
	return result, nil
}
