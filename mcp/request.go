package mcp

import "context"

// Message represents a conversation message.
// Supports plain messages (Role+Content), assistant tool-call messages (ToolCalls),
// and tool result messages (Role="tool", ToolCallID, Content).
type Message struct {
	Role       string     `json:"role"`                  // "system", "user", "assistant", "tool"
	Content    string     `json:"content,omitempty"`     // Text content (omitted when ToolCalls present)
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // Set by assistant when calling tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // Set on role="tool" result messages
}

// ToolCall is a single function call requested by the LLM.
type ToolCall struct {
	ID       string           `json:"id"`       // Unique call ID (e.g. "call_abc123")
	Type     string           `json:"type"`     // Always "function"
	Function ToolCallFunction `json:"function"` // Function name and JSON-serialised arguments
}

// ToolCallFunction holds the function name and raw JSON arguments string.
type ToolCallFunction struct {
	Name      string `json:"name"`      // Function name
	Arguments string `json:"arguments"` // JSON-encoded argument object
}

// LLMResponse is returned by CallWithRequestFull and carries both the assistant
// text reply (Content) and any structured tool calls (ToolCalls).
// Exactly one of the two fields will be non-empty for a well-formed response.
type LLMResponse struct {
	Content   string     // Plain-text reply (final answer)
	ToolCalls []ToolCall // Structured tool invocations
}

// Tool represents a tool/function that AI can call
type Tool struct {
	Type     string      `json:"type"`     // Usually "function"
	Function FunctionDef `json:"function"` // Function definition
}

// FunctionDef function definition
type FunctionDef struct {
	Name        string         `json:"name"`                  // Function name
	Description string         `json:"description,omitempty"` // Function description
	Parameters  map[string]any `json:"parameters,omitempty"`  // Parameter schema (JSON Schema)
}

// Request AI API request (supports advanced features)
type Request struct {
	// Basic fields
	Model    string    `json:"model"`              // Model name
	Messages []Message `json:"messages"`           // Conversation message list
	Stream   bool      `json:"stream,omitempty"`   // Whether to stream response

	// Optional parameters (for fine-grained control)
	Temperature      *float64 `json:"temperature,omitempty"`       // Temperature (0-2), controls randomness
	MaxTokens        *int     `json:"max_tokens,omitempty"`        // Maximum token count
	TopP             *float64 `json:"top_p,omitempty"`             // Nucleus sampling parameter (0-1)
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"` // Frequency penalty (-2 to 2)
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`  // Presence penalty (-2 to 2)
	Stop             []string `json:"stop,omitempty"`              // Stop sequences

	// Advanced features
	Tools      []Tool `json:"tools,omitempty"`       // Available tools list
	ToolChoice string `json:"tool_choice,omitempty"` // Tool choice strategy ("auto", "none", {"type": "function", "function": {"name": "xxx"}})

	// Context for cancellation; not serialized.
	Ctx context.Context `json:"-"`
}

// NewMessage creates a message
func NewMessage(role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// NewSystemMessage creates a system message
func NewSystemMessage(content string) Message {
	return Message{
		Role:    "system",
		Content: content,
	}
}

// NewUserMessage creates a user message
func NewUserMessage(content string) Message {
	return Message{
		Role:    "user",
		Content: content,
	}
}

// NewAssistantMessage creates an assistant message
func NewAssistantMessage(content string) Message {
	return Message{
		Role:    "assistant",
		Content: content,
	}
}
