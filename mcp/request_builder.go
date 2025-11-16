package mcp

import (
	"errors"
)

// RequestBuilder 请求构建器
type RequestBuilder struct {
	model            string
	messages         []Message
	stream           bool
	temperature      *float64
	maxTokens        *int
	topP             *float64
	frequencyPenalty *float64
	presencePenalty  *float64
	stop             []string
	tools            []Tool
	toolChoice       string
}

// NewRequestBuilder 创建请求构建器
//
// 使用示例：
//   request := NewRequestBuilder().
//       WithSystemPrompt("You are helpful").
//       WithUserPrompt("Hello").
//       WithTemperature(0.8).
//       Build()
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		messages: make([]Message, 0),
		tools:    make([]Tool, 0),
	}
}

// ============================================================
// 模型和流式配置
// ============================================================

// WithModel 设置模型名称
func (b *RequestBuilder) WithModel(model string) *RequestBuilder {
	b.model = model
	return b
}

// WithStream 设置是否使用流式响应
func (b *RequestBuilder) WithStream(stream bool) *RequestBuilder {
	b.stream = stream
	return b
}

// ============================================================
// 消息构建方法
// ============================================================

// WithSystemPrompt 添加系统提示词（便捷方法）
func (b *RequestBuilder) WithSystemPrompt(prompt string) *RequestBuilder {
	if prompt != "" {
		b.messages = append(b.messages, NewSystemMessage(prompt))
	}
	return b
}

// WithUserPrompt 添加用户提示词（便捷方法）
func (b *RequestBuilder) WithUserPrompt(prompt string) *RequestBuilder {
	if prompt != "" {
		b.messages = append(b.messages, NewUserMessage(prompt))
	}
	return b
}

// AddSystemMessage 添加系统消息
func (b *RequestBuilder) AddSystemMessage(content string) *RequestBuilder {
	return b.WithSystemPrompt(content)
}

// AddUserMessage 添加用户消息
func (b *RequestBuilder) AddUserMessage(content string) *RequestBuilder {
	return b.WithUserPrompt(content)
}

// AddAssistantMessage 添加助手消息（用于多轮对话上下文）
func (b *RequestBuilder) AddAssistantMessage(content string) *RequestBuilder {
	if content != "" {
		b.messages = append(b.messages, NewAssistantMessage(content))
	}
	return b
}

// AddMessage 添加自定义角色的消息
func (b *RequestBuilder) AddMessage(role, content string) *RequestBuilder {
	if content != "" {
		b.messages = append(b.messages, NewMessage(role, content))
	}
	return b
}

// AddMessages 批量添加消息
func (b *RequestBuilder) AddMessages(messages ...Message) *RequestBuilder {
	b.messages = append(b.messages, messages...)
	return b
}

// AddConversationHistory 添加对话历史
func (b *RequestBuilder) AddConversationHistory(history []Message) *RequestBuilder {
	b.messages = append(b.messages, history...)
	return b
}

// ClearMessages 清空所有消息
func (b *RequestBuilder) ClearMessages() *RequestBuilder {
	b.messages = make([]Message, 0)
	return b
}

// ============================================================
// 参数控制方法
// ============================================================

// WithTemperature 设置温度参数 (0-2)
// 较高的温度（如 1.2）会使输出更随机，较低的温度（如 0.2）会使输出更确定
func (b *RequestBuilder) WithTemperature(t float64) *RequestBuilder {
	if t < 0 || t > 2 {
		// 可以选择 panic 或者静默忽略，这里选择限制范围
		if t < 0 {
			t = 0
		}
		if t > 2 {
			t = 2
		}
	}
	b.temperature = &t
	return b
}

// WithMaxTokens 设置最大 token 数
func (b *RequestBuilder) WithMaxTokens(tokens int) *RequestBuilder {
	if tokens > 0 {
		b.maxTokens = &tokens
	}
	return b
}

// WithTopP 设置 top-p 核采样参数 (0-1)
// 控制考虑的 token 范围，较小的值（如 0.1）使输出更聚焦
func (b *RequestBuilder) WithTopP(p float64) *RequestBuilder {
	if p >= 0 && p <= 1 {
		b.topP = &p
	}
	return b
}

// WithFrequencyPenalty 设置频率惩罚 (-2 to 2)
// 正值会根据 token 在文本中出现的频率惩罚它们，减少重复
func (b *RequestBuilder) WithFrequencyPenalty(penalty float64) *RequestBuilder {
	if penalty >= -2 && penalty <= 2 {
		b.frequencyPenalty = &penalty
	}
	return b
}

// WithPresencePenalty 设置存在惩罚 (-2 to 2)
// 正值会根据 token 是否出现在文本中惩罚它们，增加话题多样性
func (b *RequestBuilder) WithPresencePenalty(penalty float64) *RequestBuilder {
	if penalty >= -2 && penalty <= 2 {
		b.presencePenalty = &penalty
	}
	return b
}

// WithStopSequences 设置停止序列
// 当模型生成这些序列之一时，将停止生成
func (b *RequestBuilder) WithStopSequences(sequences []string) *RequestBuilder {
	b.stop = sequences
	return b
}

// AddStopSequence 添加单个停止序列
func (b *RequestBuilder) AddStopSequence(sequence string) *RequestBuilder {
	if sequence != "" {
		b.stop = append(b.stop, sequence)
	}
	return b
}

// ============================================================
// 工具/函数调用相关
// ============================================================

// AddTool 添加工具
func (b *RequestBuilder) AddTool(tool Tool) *RequestBuilder {
	b.tools = append(b.tools, tool)
	return b
}

// AddFunction 添加函数（便捷方法）
func (b *RequestBuilder) AddFunction(name, description string, parameters map[string]any) *RequestBuilder {
	tool := Tool{
		Type: "function",
		Function: FunctionDef{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
	b.tools = append(b.tools, tool)
	return b
}

// WithToolChoice 设置工具选择策略
// - "auto": 自动选择是否调用工具
// - "none": 不调用工具
// - 也可以指定特定工具: `{"type": "function", "function": {"name": "my_function"}}`
func (b *RequestBuilder) WithToolChoice(choice string) *RequestBuilder {
	b.toolChoice = choice
	return b
}

// ============================================================
// 构建方法
// ============================================================

// Build 构建请求对象
func (b *RequestBuilder) Build() (*Request, error) {
	// 验证：至少需要一条消息
	if len(b.messages) == 0 {
		return nil, errors.New("至少需要一条消息")
	}

	// 创建请求
	req := &Request{
		Model:      b.model,
		Messages:   b.messages,
		Stream:     b.stream,
		Stop:       b.stop,
		Tools:      b.tools,
		ToolChoice: b.toolChoice,
	}

	// 只设置非 nil 的可选参数（避免发送 0 值覆盖服务端默认值）
	if b.temperature != nil {
		req.Temperature = b.temperature
	}
	if b.maxTokens != nil {
		req.MaxTokens = b.maxTokens
	}
	if b.topP != nil {
		req.TopP = b.topP
	}
	if b.frequencyPenalty != nil {
		req.FrequencyPenalty = b.frequencyPenalty
	}
	if b.presencePenalty != nil {
		req.PresencePenalty = b.presencePenalty
	}

	return req, nil
}

// MustBuild 构建请求对象，如果失败则 panic
// 适用于构建过程中确定不会出错的场景
func (b *RequestBuilder) MustBuild() *Request {
	req, err := b.Build()
	if err != nil {
		panic(err)
	}
	return req
}

// ============================================================
// 便捷方法：预设场景
// ============================================================

// ForChat 创建用于聊天的构建器（预设合理的参数）
func ForChat() *RequestBuilder {
	temp := 0.7
	tokens := 2000
	return &RequestBuilder{
		messages:    make([]Message, 0),
		tools:       make([]Tool, 0),
		temperature: &temp,
		maxTokens:   &tokens,
	}
}

// ForCodeGeneration 创建用于代码生成的构建器（低温度，更确定）
func ForCodeGeneration() *RequestBuilder {
	temp := 0.2
	tokens := 2000
	topP := 0.1
	return &RequestBuilder{
		messages:    make([]Message, 0),
		tools:       make([]Tool, 0),
		temperature: &temp,
		maxTokens:   &tokens,
		topP:        &topP,
	}
}

// ForCreativeWriting 创建用于创意写作的构建器（高温度，更随机）
func ForCreativeWriting() *RequestBuilder {
	temp := 1.2
	tokens := 4000
	topP := 0.95
	presencePenalty := 0.6
	frequencyPenalty := 0.5
	return &RequestBuilder{
		messages:         make([]Message, 0),
		tools:            make([]Tool, 0),
		temperature:      &temp,
		maxTokens:        &tokens,
		topP:             &topP,
		presencePenalty:  &presencePenalty,
		frequencyPenalty: &frequencyPenalty,
	}
}
