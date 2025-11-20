package mcp

// Message 表示一条对话消息
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // 消息内容
}

// Tool 表示 AI 可以调用的工具/函数
type Tool struct {
	Type     string      `json:"type"`     // 通常为 "function"
	Function FunctionDef `json:"function"` // 函数定义
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string         `json:"name"`                  // 函数名
	Description string         `json:"description,omitempty"` // 函数描述
	Parameters  map[string]any `json:"parameters,omitempty"`  // 参数 schema (JSON Schema)
}

// Request AI API 请求（支持高级功能）
type Request struct {
	// 基础字段
	Model    string    `json:"model"`              // 模型名称
	Messages []Message `json:"messages"`           // 对话消息列表
	Stream   bool      `json:"stream,omitempty"`   // 是否流式响应

	// 可选参数（用于精细控制）
	Temperature      *float64 `json:"temperature,omitempty"`       // 温度 (0-2)，控制随机性
	MaxTokens        *int     `json:"max_tokens,omitempty"`        // 最大 token 数
	TopP             *float64 `json:"top_p,omitempty"`             // 核采样参数 (0-1)
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"` // 频率惩罚 (-2 to 2)
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`  // 存在惩罚 (-2 to 2)
	Stop             []string `json:"stop,omitempty"`              // 停止序列

	// 高级功能
	Tools      []Tool `json:"tools,omitempty"`       // 可用工具列表
	ToolChoice string `json:"tool_choice,omitempty"` // 工具选择策略 ("auto", "none", {"type": "function", "function": {"name": "xxx"}})
}

// NewMessage 创建一条消息
func NewMessage(role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(content string) Message {
	return Message{
		Role:    "system",
		Content: content,
	}
}

// NewUserMessage 创建用户消息
func NewUserMessage(content string) Message {
	return Message{
		Role:    "user",
		Content: content,
	}
}

// NewAssistantMessage 创建助手消息
func NewAssistantMessage(content string) Message {
	return Message{
		Role:    "assistant",
		Content: content,
	}
}
