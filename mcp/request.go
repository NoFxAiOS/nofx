package mcp

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // Message content
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
