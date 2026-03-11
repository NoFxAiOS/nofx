package mcp

import (
	"encoding/json"
	"net/http"
	"testing"
)

// ── buildRequestBodyFromRequest ────────────────────────────────────────────────

func TestClaudeClient_BuildRequestBody_SystemPromptLifted(t *testing.T) {
	c := newTestClaudeClient()
	req := &Request{
		Model: "claude-opus-4-6",
		Messages: []Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	body := c.buildRequestBodyFromRequest(req)

	if body["system"] != "You are helpful." {
		t.Errorf("system not lifted to top level: %v", body["system"])
	}
	msgs := body["messages"].([]map[string]any)
	if len(msgs) != 1 || msgs[0]["role"] != "user" {
		t.Errorf("system message should be removed from messages array: %v", msgs)
	}
}

func TestClaudeClient_BuildRequestBody_ToolsUseInputSchema(t *testing.T) {
	c := newTestClaudeClient()
	req := &Request{
		Model:   "claude-opus-4-6",
		Messages: []Message{{Role: "user", Content: "hi"}},
		Tools: []Tool{{
			Type: "function",
			Function: FunctionDef{
				Name:        "my_tool",
				Description: "does stuff",
				Parameters:  map[string]any{"type": "object"},
			},
		}},
	}
	body := c.buildRequestBodyFromRequest(req)

	tools, ok := body["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools not set correctly: %v", body["tools"])
	}
	tool := tools[0]
	if tool["name"] != "my_tool" {
		t.Errorf("tool name wrong: %v", tool["name"])
	}
	if tool["input_schema"] == nil {
		t.Error("tool must use input_schema, not parameters")
	}
	if _, hasParams := tool["parameters"]; hasParams {
		t.Error("tool must NOT have parameters key (Anthropic uses input_schema)")
	}
}

func TestClaudeClient_BuildRequestBody_ToolChoiceObject(t *testing.T) {
	c := newTestClaudeClient()
	req := &Request{
		Model:      "claude-opus-4-6",
		Messages:   []Message{{Role: "user", Content: "hi"}},
		ToolChoice: "auto",
	}
	body := c.buildRequestBodyFromRequest(req)

	tc, ok := body["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("tool_choice must be an object, got: %T %v", body["tool_choice"], body["tool_choice"])
	}
	if tc["type"] != "auto" {
		t.Errorf("tool_choice.type must be 'auto', got: %v", tc["type"])
	}
}

// ── convertMessagesToAnthropic ─────────────────────────────────────────────────

func TestConvertMessages_AssistantToolCall(t *testing.T) {
	msgs := []Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:   "tc1",
				Type: "function",
				Function: ToolCallFunction{Name: "api_request", Arguments: `{"method":"GET","path":"/api/x","body":{}}`},
			}},
		},
	}
	out := convertMessagesToAnthropic(msgs)

	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	msg := out[0]
	if msg["role"] != "assistant" {
		t.Errorf("role should be assistant: %v", msg["role"])
	}
	blocks := msg["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "tool_use" {
		t.Errorf("content should be tool_use block: %v", blocks)
	}
	if blocks[0]["id"] != "tc1" {
		t.Errorf("tool_use id wrong: %v", blocks[0]["id"])
	}
	// Input must be parsed JSON object, not a string.
	input, ok := blocks[0]["input"].(map[string]any)
	if !ok {
		t.Errorf("tool_use input must be map, got %T", blocks[0]["input"])
	}
	if input["method"] != "GET" {
		t.Errorf("input.method wrong: %v", input)
	}
}

func TestConvertMessages_ToolResultMergedIntoUserTurn(t *testing.T) {
	// Anthropic requires strictly alternating turns; consecutive tool results
	// must be merged into a single user message.
	msgs := []Message{
		{Role: "tool", ToolCallID: "tc1", Content: `{"result":"a"}`},
		{Role: "tool", ToolCallID: "tc2", Content: `{"result":"b"}`},
	}
	out := convertMessagesToAnthropic(msgs)

	if len(out) != 1 {
		t.Fatalf("consecutive tool results must be merged into one user turn, got %d messages", len(out))
	}
	if out[0]["role"] != "user" {
		t.Errorf("tool results must become role=user: %v", out[0]["role"])
	}
	blocks := out[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Errorf("expected 2 tool_result blocks, got %d", len(blocks))
	}
	if blocks[0]["type"] != "tool_result" || blocks[1]["type"] != "tool_result" {
		t.Errorf("blocks should be tool_result: %v", blocks)
	}
	if blocks[0]["tool_use_id"] != "tc1" || blocks[1]["tool_use_id"] != "tc2" {
		t.Errorf("tool_use_id mismatch: %v", blocks)
	}
}

// ── parseMCPResponseFull ───────────────────────────────────────────────────────

func TestClaudeClient_ParseResponse_TextOnly(t *testing.T) {
	c := newTestClaudeClient()
	body := []byte(`{
		"content": [{"type":"text","text":"Hello from Claude"}],
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`)
	resp, err := c.parseMCPResponseFull(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from Claude" {
		t.Errorf("content mismatch: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls: %v", resp.ToolCalls)
	}
}

func TestClaudeClient_ParseResponse_ToolUse(t *testing.T) {
	c := newTestClaudeClient()
	body := []byte(`{
		"content": [{
			"type": "tool_use",
			"id": "toolu_01abc",
			"name": "api_request",
			"input": {"method":"POST","path":"/api/strategies","body":{"name":"BTC策略"}}
		}],
		"usage": {"input_tokens": 100, "output_tokens": 30}
	}`)
	resp, err := c.parseMCPResponseFull(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "toolu_01abc" {
		t.Errorf("tool call ID wrong: %v", tc.ID)
	}
	if tc.Function.Name != "api_request" {
		t.Errorf("function name wrong: %v", tc.Function.Name)
	}
	// Arguments must be a valid JSON string.
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		t.Errorf("arguments not valid JSON: %q — %v", tc.Function.Arguments, err)
	}
	if args["method"] != "POST" {
		t.Errorf("args.method wrong: %v", args)
	}
}

func TestClaudeClient_ParseResponse_APIError(t *testing.T) {
	c := newTestClaudeClient()
	body := []byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	_, err := c.parseMCPResponseFull(body)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

// ── Auth header ────────────────────────────────────────────────────────────────

func TestClaudeClient_SetAuthHeader(t *testing.T) {
	c := newTestClaudeClient()
	c.APIKey = "sk-ant-test123"

	// net/http.Header canonicalizes keys (x-api-key → X-Api-Key).
	h := make(http.Header)
	c.setAuthHeader(h)

	if got := h.Get("x-api-key"); got != "sk-ant-test123" {
		t.Errorf("x-api-key header not set correctly: %q", got)
	}
	if h.Get("anthropic-version") == "" {
		t.Error("anthropic-version header must be set")
	}
	// Must NOT use Authorization: Bearer (that's OpenAI format).
	if h.Get("Authorization") != "" {
		t.Error("Claude must use x-api-key, not Authorization header")
	}
}

func TestClaudeClient_BuildUrl(t *testing.T) {
	c := newTestClaudeClient()
	url := c.buildUrl()
	if url != DefaultClaudeBaseURL+"/messages" {
		t.Errorf("URL should be /messages endpoint, got: %s", url)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

func newTestClaudeClient() *ClaudeClient {
	return NewClaudeClientWithOptions().(*ClaudeClient)
}
